package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

var errPreparationCrash = errors.New("simulated preparation crash")

type remoteObjects struct {
	*processingObjects
	data  map[string][]byte
	reads map[string]int
}

func (o *remoteObjects) Download(ctx context.Context, key string, w io.Writer, n int64) (int64, error) {
	if !strings.HasPrefix(key, clip.MediaAnalysisPrefix) {
		return 0, errors.New("API tried to download an original")
	}
	o.reads[key]++
	data, ok := o.data[key]
	if !ok {
		return 0, clip.ErrNotFound
	}
	return io.Copy(w, bytes.NewReader(data))
}
func (o *remoteObjects) PresignMediaRead(context.Context, string, time.Duration) (clip.MediaArtifactAccess, error) {
	return clip.MediaArtifactAccess{}, nil
}
func (o *remoteObjects) PresignMediaWrite(_ context.Context, key, contentType string, n int64, ttl time.Duration) (clip.MediaArtifactAccess, error) {
	return clip.MediaArtifactAccess{URL: "https://fixture.test/" + key, Bytes: n, ContentType: contentType, ExpiresAfter: ttl}, nil
}
func (o *remoteObjects) HeadMediaArtifact(_ context.Context, key string) (clip.SourceObjectInfo, error) {
	data, ok := o.data[key]
	if !ok {
		return clip.SourceObjectInfo{}, clip.ErrNotFound
	}
	return clip.SourceObjectInfo{Bytes: int64(len(data)), ContentType: "video/mp4"}, nil
}

type waitFault struct {
	clipapp.JobWaitTx
	fault *string
}

func (w waitFault) Park(ctx context.Context, id, key string, p job.ResumePolicy, at time.Time) error {
	if err := w.JobWaitTx.Park(ctx, id, key, p, at); err != nil {
		return err
	}
	if *w.fault == "park" {
		return errPreparationCrash
	}
	return nil
}
func (w waitFault) Wake(ctx context.Context, id, key string, at time.Time) (bool, error) {
	changed, err := w.JobWaitTx.Wake(ctx, id, key, at)
	if err == nil && *w.fault == "wake" {
		return false, errPreparationCrash
	}
	return changed, err
}

type remoteAdmission struct {
	tx      *sql.Tx
	objects *remoteObjects
	fault   *string
}

func (a remoteAdmission) Hold(ctx context.Context, h clipapp.Hold) error {
	// The production ledger supplies this idempotent behavior. Here a real writer
	// transaction records it while the integration counts dispatch/provider work.
	for key := range a.objects.data {
		if a.objects.reads[key] == 0 {
			return errors.New("hold preceded copy verification")
		}
	}
	raw, _ := json.Marshal(h)
	if _, err := a.tx.ExecContext(ctx, `INSERT INTO media_test_holds(job_id,receipt) VALUES(?,?) ON CONFLICT DO NOTHING`, h.JobID, string(raw)); err != nil {
		return err
	}
	var prior string
	if err := a.tx.QueryRowContext(ctx, `SELECT receipt FROM media_test_holds WHERE job_id=?`, h.JobID).Scan(&prior); err != nil {
		return err
	}
	if prior != string(raw) {
		return errors.New("conflicting hold")
	}
	if *a.fault == "reserve" {
		return errPreparationCrash
	}
	return nil
}

type remoteGeneration struct {
	h         *generationHarness
	objects   *remoteObjects
	bind      clipapp.Binder
	fault     string
	artifacts *clipapp.MediaArtifacts
}

type preparationFinisher struct {
	clip.ClipFinisher
	fault *string
}

func (f preparationFinisher) Complete(ctx context.Context, c clip.AttemptResult) error {
	if *f.fault == "finish" {
		return errPreparationCrash
	}
	return f.ClipFinisher.Complete(ctx, c)
}

func remoteGenerationSetup(t *testing.T) *remoteGeneration {
	t.Helper()
	h := generationSetup(t)
	h.planner.portableFlow = true
	if _, err := h.db.Writer.Exec(`CREATE TABLE media_test_holds(job_id TEXT PRIMARY KEY,receipt TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	g := &remoteGeneration{h: h, objects: &remoteObjects{processingObjects: h.objects, data: map[string][]byte{}, reads: map[string]int{}}}
	h.cfg.Media = clip.DefaultMediaConfig(clip.Environment{SourceBatchTTL: 6 * time.Hour, PutTTL: 10 * time.Minute})
	g.bind = func(tx *sql.Tx) clipapp.Ports {
		clips := store.NewTx(tx)
		jobs := jobstore.NewTx(tx, jobKindsForTest())
		return clipapp.Ports{Clips: clips, Media: clips, Stages: clips, Jobs: jobs, Waits: waitFault{jobs, &g.fault}, Admission: remoteAdmission{tx, g.objects, &g.fault}}
	}
	dispatch, err := clipapp.NewMediaDispatch(h.db.Writer, g.bind, clip.DefaultMediaStageLimits(clip.Environment{}), h.cfg.Media, nil)
	if err != nil {
		t.Fatal(err)
	}
	finisher := clipapp.NewFinisher(h.db.Writer, g.bind, h.jobs, h.store, nil)
	guard := clipapp.NewGuard(h.db.Writer, g.bind, h.jobs)
	deps := generationDeps(preparationFinisher{finisher, &g.fault}, &quotePricing{}, nil)
	deps.RemoteMedia = dispatch
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, g.objects, h.media, h.planner, h.renderer, clipapp.NewJobs(h.queue, guard), h.cfg, deps)
	g.artifacts = clipapp.NewMediaArtifacts(h.db.Writer, g.bind, g.objects, h.cfg.Media, nil)
	return g
}
func (g *remoteGeneration) runJob(t *testing.T, j job.Job) error {
	t.Helper()
	return g.h.service.Run(t.Context(), j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, func(stage string, done, total int) {
		if err := g.h.jobs.UpdateProgress(t.Context(), j.ID, stage, done, total, time.Now()); err != nil {
			t.Fatal(err)
		}
	})
}
func (g *remoteGeneration) pick(t *testing.T) job.Job {
	t.Helper()
	j, err := g.h.jobs.PickNextQueued(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return j
}
func (g *remoteGeneration) holds(t *testing.T) int {
	t.Helper()
	var n int
	if err := g.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM media_test_holds`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func (g *remoteGeneration) preparedReceipt(t *testing.T) (clip.MediaLeaseCredentials, string) {
	t.Helper()
	profile := mediaProfile()
	profile.Operation = clip.MediaPrepare
	lease, err := g.h.store.ClaimMediaStage(t.Context(), profile, time.Now())
	if err != nil || lease == nil {
		t.Fatal("claim", err)
	}
	task, err := mediacodec.DecodeTask(lease.Stage.Payload)
	if err != nil {
		t.Fatal(err)
	}
	result := clip.MediaResult{Version: clip.MediaContractVersion}
	sum := sha256.Sum256([]byte("proxy"))
	for _, s := range task.Sources {
		info := clip.MediaInfo{DurationMS: s.DurationMS, Width: s.Width, Height: s.Height}
		result.Sources = append(result.Sources, clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: info})
		for index, offset := 0, 0; offset < s.DurationMS; index, offset = index+1, offset+60000 {
			if slices.Contains(s.ReusedChunks, index) {
				continue
			}
			duration := min(60000, s.DurationMS-offset)
			result.Outputs = append(result.Outputs, clip.MediaOutput{Slot: clip.MediaAnalysisSlot(s.ID, index), SourceID: s.ID, Index: index, OffsetMS: offset, DurationMS: duration, Bytes: 5, ContentType: "video/mp4", Digest: hex.EncodeToString(sum[:]), Info: clip.MediaInfo{DurationMS: duration, ContainerDurationMS: duration, Width: 720, Height: 404}})
		}
	}
	if len(result.Outputs) > 0 {
		if _, err = g.artifacts.Reserve(t.Context(), lease.Credentials, result.Outputs); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := g.h.store.MediaArtifacts(t.Context(), lease.Credentials.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range rows {
		g.objects.data[a.ObjectKey] = []byte("proxy")
	}
	raw, err := mediacodec.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	return lease.Credentials, raw
}
func TestRemotePreparationCommitsParkAndReceiptWithOneContinuation(t *testing.T) {
	g := remoteGenerationSetup(t)
	id := g.h.start(t)
	first := g.pick(t)
	g.fault = "park"
	if err := g.runJob(t, first); !errors.Is(err, errPreparationCrash) {
		t.Fatal(err)
	}
	if _, err := g.h.store.MediaStageForJob(t.Context(), id, clip.MediaPrepare); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("orphan stage survived rolled-back park", err)
	}
	if _, err := g.h.jobs.Continuation(t.Context(), id); !errors.Is(err, job.ErrInvalidWait) {
		t.Fatal("orphan wait", err)
	}
	g.fault = ""
	if err := g.runJob(t, first); err != job.ErrYield {
		t.Fatal("pending wrapped as failure", err)
	}
	if g.holds(t) != 0 || g.h.planner.observe != 0 || g.h.media.probes != 0 {
		t.Fatal("waiting spent local media or paid work")
	}
	if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
		t.Fatal("restart failed valid wait", n, err)
	}
	// An unrelated API job remains immediately dispatchable while this job waits.
	other, err := g.h.queue.Enqueue(t.Context(), job.NewJob{Kind: "other", UserID: "alice", NonMetered: true})
	if err != nil {
		t.Fatal(err)
	}
	picked := g.pick(t)
	if picked.ID != other {
		t.Fatal("wait consumed queue slot")
	}
	if err = g.h.jobs.Finish(t.Context(), other, job.StatusDone, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	auth, raw := g.preparedReceipt(t)
	g.fault = "wake"
	if err = g.artifacts.Complete(t.Context(), auth, raw); !errors.Is(err, errPreparationCrash) {
		t.Fatal(err)
	}
	stage, err := g.h.store.MediaStageForJob(t.Context(), id, clip.MediaPrepare)
	if err != nil || stage.State != clip.MediaRunning {
		t.Fatal("receipt escaped failed wake", stage.State, err)
	}
	wait, err := g.h.jobs.Continuation(t.Context(), id)
	if err != nil || wait.State != job.ContinuationWaiting {
		t.Fatal(wait, err)
	}
	g.fault = ""
	if err = g.artifacts.Complete(t.Context(), auth, raw); err != nil {
		t.Fatal(err)
	}
	if err = g.artifacts.Complete(t.Context(), auth, raw); err != nil {
		t.Fatal("receipt replay", err)
	}
	if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
		t.Fatal("restart lost ready handoff", n, err)
	}
	if err = g.runJob(t, g.pick(t)); err != nil {
		t.Fatal(err)
	}
	if g.holds(t) != 1 || g.h.planner.observe != 2 || g.h.planner.flows != 1 || g.h.planner.narrations != 1 {
		t.Fatal("paid call count", g.holds(t), g.h.planner.observe, g.h.planner.flows, g.h.planner.narrations)
	}
	completed, err := g.h.jobs.GetByID(t.Context(), id)
	if err != nil || completed.Status != job.StatusDone {
		t.Fatal(completed, err)
	}
	if err = g.artifacts.Complete(t.Context(), auth, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = g.h.jobs.PickNextQueued(t.Context(), time.Now()); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("completion replay scheduled analysis", err)
	}
	for key, count := range g.objects.reads {
		if count < 2 {
			t.Fatal("copy was not checked before its inline read", key, count)
		}
	}
	if err = g.h.sources.ReleaseAttempt(t.Context(), "alice", id, *completed.FinishedAt); err != nil {
		t.Fatal(err)
	}
	for _, s := range g.h.batch.Sources {
		if _, ok := g.h.objects.info[s.Key]; !ok {
			t.Fatal("completion deleted retained original")
		}
	}
}
func TestRemotePreparationRefusalsSpendNoHoldOrCall(t *testing.T) {
	for _, failure := range []string{"corrupt", "cancel", "expired", "revision", "deleted", "source"} {
		t.Run(failure, func(t *testing.T) {
			g := remoteGenerationSetup(t)
			id := g.h.start(t)
			if err := g.runJob(t, g.pick(t)); err != job.ErrYield {
				t.Fatal(err)
			}
			auth, raw := g.preparedReceipt(t)
			if err := g.artifacts.Complete(t.Context(), auth, raw); err != nil {
				t.Fatal(err)
			}
			resumed := g.pick(t)
			switch failure {
			case "corrupt":
				for key := range g.objects.data {
					g.objects.data[key] = []byte("wrong")
				}
			case "cancel":
				if _, err := g.h.db.Writer.Exec(`UPDATE generation_jobs SET cancel_requested_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id); err != nil {
					t.Fatal(err)
				}
			case "expired":
				if _, err := g.h.db.Writer.Exec(`UPDATE clip_source_leases SET retention_expires_at=?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
			case "revision":
				if _, err := g.h.db.Writer.Exec(`UPDATE clip_projects SET edit_plan_revision=edit_plan_revision+1 WHERE id=?`, g.h.project.ID); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if _, err := g.h.db.Writer.Exec(`UPDATE generation_jobs SET cancel_requested_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id); err != nil {
					t.Fatal(err)
				}
				if err := g.h.jobs.Finish(t.Context(), id, job.StatusCancelled, nil, time.Now()); err != nil {
					t.Fatal(err)
				}
				if _, err := g.h.db.Writer.Exec(`DELETE FROM clip_projects WHERE id=?`, g.h.project.ID); err != nil {
					t.Fatal(err)
				}
			case "source":
				if _, err := g.h.db.Writer.Exec(`UPDATE clip_source_leases SET fingerprint=fingerprint || 'changed'`); err != nil {
					t.Fatal(err)
				}
			}
			if err := g.runJob(t, resumed); err == nil {
				t.Fatal("invalid continuation accepted")
			}
			if g.holds(t) != 0 || g.h.planner.observe != 0 {
				t.Fatal("invalid artifacts spent paid work")
			}
		})
	}
}
func TestClaimedPreparationNeverReplaysUncertainPaidWork(t *testing.T) {
	for _, point := range []string{"reserve", "first-call"} {
		t.Run(point, func(t *testing.T) {
			g := remoteGenerationSetup(t)
			id := g.h.start(t)
			if err := g.runJob(t, g.pick(t)); err != job.ErrYield {
				t.Fatal(err)
			}
			auth, raw := g.preparedReceipt(t)
			if err := g.artifacts.Complete(t.Context(), auth, raw); err != nil {
				t.Fatal(err)
			}
			resumed := g.pick(t)
			if point == "reserve" {
				g.fault = "reserve"
			} else {
				g.h.planner.observeErr = context.Canceled
				g.h.planner.failObserveAt = 1
			}
			if err := g.runJob(t, resumed); err == nil {
				t.Fatal("crash point missed")
			}
			want := 0
			if point == "first-call" {
				want = 1
			}
			if g.holds(t) != want || g.h.planner.observe != want {
				t.Fatal("unexpected work before crash", g.holds(t), g.h.planner.observe)
			}
			if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 1 {
				t.Fatal("uncertain continuation replayed", n, err)
			}
			if _, err := g.h.jobs.PickNextQueued(t.Context(), time.Now()); !errors.Is(err, job.ErrNotFound) {
				t.Fatal("paid work became runnable again", err)
			}
			j, err := g.h.jobs.GetByID(t.Context(), id)
			if err != nil || j.Status != job.StatusFailed {
				t.Fatal(j, err)
			}
		})
	}
}

func TestRemotePreparationRecoveryReusesCompletedObservationAndPlan(t *testing.T) {
	for _, completePlan := range []bool{false, true} {
		t.Run(map[bool]string{false: "observations", true: "plan"}[completePlan], func(t *testing.T) {
			g := remoteGenerationSetup(t)
			id := g.h.start(t)
			if err := g.runJob(t, g.pick(t)); err != job.ErrYield {
				t.Fatal(err)
			}
			auth, raw := g.preparedReceipt(t)
			if err := g.artifacts.Complete(t.Context(), auth, raw); err != nil {
				t.Fatal(err)
			}
			if completePlan {
				g.fault = "finish"
			} else {
				g.h.planner.errorPlan = errPreparationCrash
			}
			if err := g.runJob(t, g.pick(t)); !errors.Is(err, errPreparationCrash) {
				t.Fatal(err)
			}
			if err := g.h.jobs.Finish(t.Context(), id, job.StatusFailed, &job.Failure{Reason: "JOB_INTERRUPTED"}, time.Now()); err != nil {
				t.Fatal(err)
			}
			if err := g.h.sources.ReleaseAttempt(t.Context(), "alice", id, time.Now()); err != nil {
				t.Fatal(err)
			}
			g.fault, g.h.planner.errorPlan = "", nil
			beforeObserve, beforeFlow, beforeNarrate, beforeHolds := g.h.planner.observe, g.h.planner.flows, g.h.planner.narrations, g.holds(t)
			q, err := g.h.service.Quote(t.Context(), "alice", g.h.project.ID, g.h.batch.ID, "p/o", "p/w")
			if err != nil || q.Pricing.ObservationCalls != 0 || q.Pricing.ReusedChunks != 2 || q.Pricing.RenderOnly() != completePlan {
				t.Fatal("wrong remaining work", q.Pricing, err)
			}
			g.h.start(t)
			if err := g.runJob(t, g.pick(t)); err != job.ErrYield {
				t.Fatal(err)
			}
			auth, raw = g.preparedReceipt(t)
			result, err := mediacodec.DecodeResult(raw)
			if err != nil || len(result.Outputs) != 0 {
				t.Fatal("re-encoded completed observations", err)
			}
			if err = g.artifacts.Complete(t.Context(), auth, raw); err != nil {
				t.Fatal(err)
			}
			if err = g.runJob(t, g.pick(t)); err != nil {
				t.Fatal(err)
			}
			additional := 1
			if completePlan {
				additional = 0
			}
			if g.h.planner.observe != beforeObserve || g.h.planner.flows != beforeFlow+additional || g.h.planner.narrations != beforeNarrate+additional || g.holds(t) != beforeHolds+additional || g.h.media.probes != 0 {
				t.Fatal("completed paid work repeated")
			}
		})
	}
}

func TestRemotePreparationWorkerRetrySpendsNothingBeforeAcceptedReceipt(t *testing.T) {
	g := remoteGenerationSetup(t)
	id := g.h.start(t)
	if err := g.runJob(t, g.pick(t)); err != job.ErrYield {
		t.Fatal(err)
	}
	oldAuth, oldReceipt := g.preparedReceipt(t)
	if _, err := g.h.db.Writer.Exec(`UPDATE clip_media_attempts SET lease_expires_at=? WHERE id=?`, time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano), oldAuth.AttemptID); err != nil {
		t.Fatal(err)
	}
	newAuth, receipt := g.preparedReceipt(t)
	if oldAuth.AttemptID == newAuth.AttemptID {
		t.Fatal("retry reused authority")
	}
	if err := g.artifacts.Complete(t.Context(), oldAuth, oldReceipt); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal("stale receipt accepted", err)
	}
	if g.holds(t) != 0 || g.h.planner.observe != 0 {
		t.Fatal("media retry spent paid work")
	}
	stage, err := g.h.store.MediaStageForJob(t.Context(), id, clip.MediaPrepare)
	if err != nil || stage.AttemptCount != 2 {
		t.Fatal(stage, err)
	}
	// Abandoned uploads are not inputs to the continuing model work.
	rows, err := g.h.store.MediaArtifacts(t.Context(), oldAuth.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		delete(g.objects.data, row.ObjectKey)
	}
	if err = g.artifacts.Complete(t.Context(), newAuth, receipt); err != nil {
		t.Fatal(err)
	}
	if err = g.runJob(t, g.pick(t)); err != nil {
		t.Fatal(err)
	}
	if g.holds(t) != 1 || g.h.planner.observe != 2 || g.h.planner.flows != 1 || g.h.planner.narrations != 1 {
		t.Fatal("wrong post-retry paid work")
	}
}
