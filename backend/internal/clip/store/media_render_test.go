package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/clip/workerclient"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

type renderObjects struct{ *remoteObjects }

func (o renderObjects) PresignMediaRead(_ context.Context, key string, ttl time.Duration) (clip.MediaArtifactAccess, error) {
	return clip.MediaArtifactAccess{URL: "https://fixture.test/" + key, ExpiresAfter: ttl}, nil
}

type renderPublicationFault struct {
	clipapp.MediaPublicationTx
	fault *string
}

func (f renderPublicationFault) ConsumeMediaRender(ctx context.Context, c clip.AttemptResult, at time.Time) error {
	if err := f.MediaPublicationTx.ConsumeMediaRender(ctx, c, at); err != nil {
		return err
	}
	if *f.fault == "consume" {
		return errPreparationCrash
	}
	return nil
}

type renderApplyFault struct {
	clipapp.ClipTx
	fault *string
}

func (f renderApplyFault) ApplyAttemptResult(ctx context.Context, c clip.AttemptResult) error {
	if err := f.ClipTx.ApplyAttemptResult(ctx, c); err != nil {
		return err
	}
	if *f.fault == "apply" {
		return errPreparationCrash
	}
	return nil
}

type renderFinishFault struct {
	clipapp.JobTx
	fault *string
}

func (f renderFinishFault) Finish(ctx context.Context, id, status string, failure *job.Failure, at time.Time) error {
	if err := f.JobTx.Finish(ctx, id, status, failure, at); err != nil {
		return err
	}
	if *f.fault == "terminal" {
		return errPreparationCrash
	}
	return nil
}

type remoteRender struct {
	h        *generationHarness
	before   clip.Project
	batch    clip.SourceBatch
	objects  renderObjects
	fault    string
	finisher clipapp.Finisher
	clients  [2]*workerclient.Client
	jobID    string
}

func remoteRenderSetup(t *testing.T, legacy bool) *remoteRender {
	t.Helper()
	h, p, _ := completedClip(t)
	b := rerenderBatch(t, h, true)
	// Current reselection preserves canonical IDs. Exercise the older queued
	// shape too: a fresh lease whose ID differs from the retained plan identity.
	if _, err := h.db.Writer.Exec(`UPDATE clip_source_leases SET canonical_id=canonical_id || '-new',retain_original_audio=1 WHERE batch_id=?`, b.ID); err != nil {
		t.Fatal(err)
	}
	var err error
	b, err = h.store.GetSourceBatch(t.Context(), "alice", b.ID)
	if err != nil {
		t.Fatal(err)
	}
	g := &remoteRender{h: h, before: p, batch: b, objects: renderObjects{&remoteObjects{processingObjects: h.objects, data: map[string][]byte{}, reads: map[string]int{}}}}
	// Existing version-1 jobs must remain executable after the API upgrade.
	if legacy {
		id, err := h.service.StartRender(t.Context(), "alice", p.ID, b.ID, p.EditPlanRevision, clip.RenderServer)
		if err != nil {
			t.Fatal(err)
		}
		g.jobID = id
	}
	h.cfg.Media = clip.DefaultMediaConfig(clip.Environment{})
	bind := func(tx *sql.Tx) clipapp.Ports {
		c := store.NewTx(tx)
		j := jobstore.NewTx(tx, jobKindsForTest())
		return clipapp.Ports{Clips: renderApplyFault{c, &g.fault}, Jobs: renderFinishFault{j, &g.fault}, Stages: c, Media: c, Waits: j, Publication: renderPublicationFault{c, &g.fault}}
	}
	dispatch, err := clipapp.NewMediaDispatch(h.db.Writer, bind, clip.DefaultMediaStageLimits(clip.Environment{}), h.cfg.Media, nil)
	if err != nil {
		t.Fatal(err)
	}
	g.finisher = clipapp.NewFinisher(h.db.Writer, bind, h.jobs, h.store, nil)
	deps := generationDeps(g.finisher, &quotePricing{}, nil)
	deps.RemoteMedia = dispatch
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, g.objects, h.media, h.planner, h.renderer, clipapp.NewJobs(h.queue, clipapp.NewGuard(h.db.Writer, bind, h.jobs)), h.cfg, deps)
	artifacts := clipapp.NewMediaArtifacts(h.db.Writer, bind, g.objects, h.cfg.Media, nil)
	api := httptest.NewServer(cliprpc.NewMediaWorkerServer("", map[string]string{"render-one": "one", "render-two": "two"}, clipapp.NewMediaWorker(h.store, artifacts, nil)).Handler)
	t.Cleanup(api.Close)
	g.clients = [2]*workerclient.Client{workerclient.New(api.URL, "render-one", "one"), workerclient.New(api.URL, "render-two", "two")}
	if !legacy {
		g.jobID, err = h.service.StartRender(t.Context(), "alice", p.ID, b.ID, p.EditPlanRevision, clip.RenderServer)
		if err != nil {
			t.Fatal(err)
		}
	}
	return g
}
func (g *remoteRender) pick(t *testing.T) job.Job {
	t.Helper()
	j, err := g.h.jobs.PickNextQueued(t.Context(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return j
}
func (g *remoteRender) run(t *testing.T, j job.Job) error {
	t.Helper()
	return g.h.service.RunRender(t.Context(), j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, func(stage string, n, total int) {
		if err := g.h.jobs.UpdateProgress(t.Context(), j.ID, stage, n, total, time.Now()); err != nil {
			t.Fatal(err)
		}
	})
}
func (g *remoteRender) claim(t *testing.T) (*workerclient.Client, *clip.MediaWork) {
	t.Helper()
	p := mediaProfile()
	p.Operation = clip.MediaRender
	var wg sync.WaitGroup
	var work [2]*clip.MediaWork
	var errs [2]error
	for i := range 2 {
		wg.Go(func() { work[i], errs[i] = g.clients[i].Claim(t.Context(), p) })
	}
	wg.Wait()
	winner := -1
	for i := range 2 {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if work[i] != nil {
			if winner >= 0 {
				t.Fatal("two workers claimed one render")
			}
			winner = i
		}
	}
	if winner < 0 {
		t.Fatal("render not claimable")
	}
	return g.clients[winner], work[winner]
}
func (g *remoteRender) upload(t *testing.T, c *workerclient.Client, w *clip.MediaWork) (clip.MediaResult, string) {
	t.Helper()
	task, err := mediacodec.DecodeTask(w.Payload)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := clip.DecodeEditPlan(task.Plan)
	if err != nil {
		t.Fatal(err)
	}
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		t.Fatal(err)
	}
	// A real renderer may append a measured contrast notice without changing
	// any authored cut, text, placement or audio setting.
	clip.AddPlanNotice(&plan, "composition_contrast", plan.Cuts[0].ID, "caption", "shortfall")
	reported, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	result := clip.MediaResult{Version: clip.MediaContractVersion, Plan: reported}
	for _, s := range task.Sources {
		result.Sources = append(result.Sources, clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info})
	}
	out := clip.MediaOutput{Slot: "result", Bytes: 6, ContentType: "video/mp4", Digest: strings.Repeat("a", 64), DurationMS: plan.DurationMS, Info: clip.MediaInfo{Width: canvas.Width, Height: canvas.Height, DurationMS: plan.DurationMS, PixelFormat: "yuv420p", SampleAspectRatio: "1:1", FrameRateNumerator: 30, FrameRateDenominator: 1, DecodedFrames: plan.DurationMS * 30 / 1000, Streams: []clip.MediaStream{{Kind: "video", Codec: "h264", Profile: "High"}}}}
	result.Outputs = []clip.MediaOutput{out}
	if _, err = c.Reserve(t.Context(), w.Credentials, result.Outputs); err != nil {
		t.Fatal(err)
	}
	rows, err := g.h.store.MediaArtifacts(t.Context(), w.Credentials.AttemptID)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	g.objects.data[rows[0].ObjectKey] = []byte("render")
	raw, err := mediacodec.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	return result, raw
}
func (g *remoteRender) preserved(t *testing.T) {
	t.Helper()
	p, err := g.h.store.GetProject(t.Context(), "alice", g.before.ID)
	if err != nil || p.Result.Key != g.before.Result.Key || p.EditPlan != g.before.EditPlan {
		t.Fatal("previous clip replaced", p, err)
	}
}
func TestRemoteRenderDurablePublicationAndLegacyPayload(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "frozen-v2", true: "queued-v1"}[legacy], func(t *testing.T) {
			g := remoteRenderSetup(t, legacy)
			if err := g.run(t, g.pick(t)); err != job.ErrYield {
				t.Fatal(err)
			}
			// A resume uses the already validated execution snapshot, including
			// version-1 jobs resolved at their first dispatch.
			g.h.renderer.captionErr = errPreparationCrash
			wait, err := g.h.jobs.Continuation(t.Context(), g.jobID)
			if err != nil || wait.Policy != job.ReplaySafe {
				t.Fatal(wait, err)
			}
			c, w := g.claim(t)
			task, err := mediacodec.DecodeTask(w.Payload)
			if err != nil {
				t.Fatal(err)
			}
			if task.Sources[0].ID == g.batch.Sources[0].ID {
				t.Fatal("fixture did not replace source lease identity")
			}
			plan, err := clip.DecodeEditPlan(task.Plan)
			if err != nil || !plan.RetainsOriginalAudio(plan.Cuts[0]) {
				t.Fatal("source rebind lost the owner's audio setting", err)
			}
			access, err := c.Read(t.Context(), w.Credentials, "source/"+task.Sources[0].ID)
			if err != nil || !strings.HasSuffix(access.URL, g.batch.Sources[0].Key) {
				t.Fatal("retained identity did not resolve to current original", err)
			}
			_, raw := g.upload(t, c, w)
			if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
				t.Fatal("restart lost live uploaded attempt", n, err)
			}
			if err = c.Complete(t.Context(), w.Credentials, raw); err != nil {
				t.Fatal(err)
			}
			if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
				t.Fatal("restart lost accepted result", n, err)
			}
			// A crash after each publication write must roll back the whole outcome.
			for _, point := range []string{"consume", "apply", "terminal"} {
				g.fault = point
				if err = g.run(t, g.pick(t)); !errors.Is(err, errPreparationCrash) {
					t.Fatal(point, err)
				}
				g.preserved(t)
				var canonical int
				if err = g.h.db.Reader.QueryRow(`SELECT canonical FROM clip_media_artifacts WHERE attempt_id=?`, w.Credentials.AttemptID).Scan(&canonical); err != nil || canonical != 0 {
					t.Fatal("partial publication", canonical, err)
				}
				if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
					t.Fatal("safe continuation not recovered", n, err)
				}
			}
			g.fault = ""
			if err = g.run(t, g.pick(t)); err != nil {
				t.Fatal(err)
			}
			if err = c.Complete(t.Context(), w.Credentials, raw); err != nil {
				t.Fatal("lost reply receipt replay", err)
			}
			j, err := g.h.jobs.GetByID(t.Context(), g.jobID)
			if err != nil || j.Status != job.StatusDone {
				t.Fatal(j, err)
			}
			rows, err := g.h.store.MediaArtifacts(t.Context(), w.Credentials.AttemptID)
			if err != nil {
				t.Fatal(err)
			}
			p, err := g.h.store.GetProject(t.Context(), "alice", g.before.ID)
			if err != nil || p.Result.Key != rows[0].ObjectKey || p.Result.Kind != clip.RenderServer || p.EditPlan != g.before.EditPlan {
				t.Fatal(p, err)
			}
			candidate := clip.AttemptResult{JobID: g.jobID, UserID: "alice", ProjectID: p.ID, ExpectedRevision: p.EditPlanRevision, Result: *p.Result}
			if err = g.finisher.Complete(t.Context(), candidate); err != nil {
				t.Fatal("canonical response loss", err)
			}
			if g.h.media.probes != 0 || g.h.renderer.calls != 0 || g.h.planner.observe != 0 || g.h.planner.plans != 0 || len(g.h.admitter.calls) != 0 {
				t.Fatal("API performed media or paid work")
			}
			for _, client := range g.clients {
				profile := mediaProfile()
				profile.Operation = clip.MediaRender
				if next, err := client.Claim(t.Context(), profile); err != nil || next != nil {
					t.Fatal("accepted render dispatched again", err)
				}
			}
			var n int
			if err = g.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_artifacts WHERE canonical=1`).Scan(&n); err != nil || n != 1 {
				t.Fatal("canonical count", n, err)
			}
		})
	}
}

func TestRemoteRenderRejectsStaleOrInvalidWorkerResult(t *testing.T) {
	for _, mode := range []string{"revision", "source", "cancel", "plan", "cadence", "dimensions", "slot"} {
		t.Run(mode, func(t *testing.T) {
			g := remoteRenderSetup(t, false)
			if err := g.run(t, g.pick(t)); err != job.ErrYield {
				t.Fatal(err)
			}
			c, w := g.claim(t)
			result, raw := g.upload(t, c, w)
			switch mode {
			case "revision":
				if _, err := g.h.db.Writer.Exec(`UPDATE clip_projects SET edit_plan_revision=edit_plan_revision+1 WHERE id=?`, g.before.ID); err != nil {
					t.Fatal(err)
				}
			case "source":
				if _, err := g.h.db.Writer.Exec(`UPDATE clip_source_leases SET fingerprint=fingerprint || 'changed' WHERE batch_id=?`, g.batch.ID); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				if err := g.h.jobs.RequestCancellation(t.Context(), "alice", g.before.ID, g.jobID, time.Now()); err != nil {
					t.Fatal(err)
				}
			case "plan":
				plan, err := clip.DecodeEditPlan(result.Plan)
				if err != nil {
					t.Fatal(err)
				}
				plan.Cuts[0].Focal.X = .2
				result.Plan, err = clip.EncodeEditPlan(plan)
				if err != nil {
					t.Fatal(err)
				}
			case "cadence":
				result.Sources[0].Info.DurationMS++
			case "dimensions":
				result.Outputs[0].Info.Width++
			case "slot":
				result.Outputs[0].Slot = "other"
			}
			if mode == "plan" || mode == "cadence" || mode == "dimensions" || mode == "slot" {
				var err error
				raw, err = mediacodec.EncodeResult(result)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := c.Complete(t.Context(), w.Credentials, raw); err == nil {
				t.Fatal("invalid result accepted")
			}
			g.preserved(t)
		})
	}
}

func TestRemoteRenderModeLeavesBrowserOutOfTheMediaQueue(t *testing.T) {
	g := remoteRenderSetup(t, false)
	// End the unstarted server job, then exercise the independent browser path.
	if _, err := g.h.queue.FailQueued(t.Context(), g.jobID, "alice", job.Failure{Reason: "JOB_INTERRUPTED"}); err != nil {
		t.Fatal(err)
	}
	if err := g.h.sources.ReleaseAttempt(t.Context(), "alice", g.jobID, time.Now()); err != nil {
		t.Fatal(err)
	}
	id, err := g.h.service.StartRender(t.Context(), "alice", g.before.ID, g.batch.ID, g.before.EditPlanRevision, clip.RenderBrowser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.h.store.GetBrowserRender(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	var stages int
	if err = g.h.db.Reader.QueryRow(`SELECT COUNT(*) FROM clip_media_stages`).Scan(&stages); err != nil || stages != 0 {
		t.Fatal("browser acquired media stage", stages, err)
	}
	var payload map[string]json.RawMessage
	j, err := g.h.jobs.GetByID(t.Context(), g.jobID)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(j.Payload, &payload); err != nil || string(payload["Version"]) != "2" || len(payload["Execution"]) == 0 {
		t.Fatal("server did not freeze execution", err)
	}
}

func TestRemoteRenderCancellationAndPublicationHaveOneWinner(t *testing.T) {
	for range 6 {
		g := remoteRenderSetup(t, false)
		if err := g.run(t, g.pick(t)); err != job.ErrYield {
			t.Fatal(err)
		}
		c, w := g.claim(t)
		_, raw := g.upload(t, c, w)
		if err := c.Complete(t.Context(), w.Credentials, raw); err != nil {
			t.Fatal(err)
		}
		g.pick(t)
		rows, err := g.h.store.MediaArtifacts(t.Context(), w.Credentials.AttemptID)
		if err != nil {
			t.Fatal(err)
		}
		a := rows[0]
		candidate := clip.AttemptResult{JobID: g.jobID, UserID: "alice", ProjectID: g.before.ID, ExpectedRevision: g.before.EditPlanRevision, Result: clip.Result{Kind: clip.RenderServer, Key: a.ObjectKey, ContentType: a.ContentType, Bytes: a.Bytes, DurationMS: a.DurationMS, CreatedAt: a.CreatedAt}}
		start := make(chan struct{})
		var wg sync.WaitGroup
		var commitErr, cancelErr error
		wg.Go(func() { <-start; commitErr = g.finisher.Complete(t.Context(), candidate) })
		wg.Go(func() {
			<-start
			cancelErr = g.h.jobs.RequestCancellation(t.Context(), "alice", g.before.ID, g.jobID, time.Now())
		})
		close(start)
		wg.Wait()
		if cancelErr != nil {
			t.Fatal(cancelErr)
		}
		j, err := g.h.jobs.GetByID(t.Context(), g.jobID)
		if err != nil {
			t.Fatal(err)
		}
		var canonical int
		if err = g.h.db.Reader.QueryRow(`SELECT canonical FROM clip_media_artifacts WHERE attempt_id=?`, w.Credentials.AttemptID).Scan(&canonical); err != nil {
			t.Fatal(err)
		}
		if j.Status == job.StatusDone {
			if j.CancelRequestedAt != nil || commitErr != nil || canonical != 1 {
				t.Fatal("completion lost its winner", j, commitErr, canonical)
			}
		} else {
			if j.CancelRequestedAt == nil || !errors.Is(commitErr, context.Canceled) || canonical != 0 {
				t.Fatal("cancellation lost its winner", j, commitErr, canonical)
			}
			g.preserved(t)
		}
	}
}

func TestRemoteRenderAcceptedSourceChangeCannotPublish(t *testing.T) {
	g := remoteRenderSetup(t, false)
	if err := g.run(t, g.pick(t)); err != job.ErrYield {
		t.Fatal(err)
	}
	c, w := g.claim(t)
	_, raw := g.upload(t, c, w)
	if err := c.Complete(t.Context(), w.Credentials, raw); err != nil {
		t.Fatal(err)
	}
	j := g.pick(t)
	if _, err := g.h.db.Writer.Exec(`UPDATE clip_source_leases SET fingerprint=fingerprint || 'changed' WHERE batch_id=?`, g.batch.ID); err != nil {
		t.Fatal(err)
	}
	if err := g.run(t, j); err == nil {
		t.Fatal("changed original published accepted result")
	}
	g.preserved(t)
	if g.h.renderer.calls != 0 || g.h.planner.observe != 0 {
		t.Fatal("refusal ran media or provider")
	}
}

func TestRemoteRenderFailureCannotFallBackToAPI(t *testing.T) {
	g := remoteRenderSetup(t, false)
	j := g.pick(t)
	if err := g.run(t, j); err != job.ErrYield {
		t.Fatal(err)
	}
	c, w := g.claim(t)
	if err := c.Fail(t.Context(), w.Credentials, clip.MediaFailureInvalidOutput); err != nil {
		t.Fatal(err)
	}
	if err := g.run(t, j); err == nil || errors.Is(err, job.ErrYield) {
		t.Fatal("worker failure hidden", err)
	}
	if g.h.media.probes != 0 || g.h.renderer.calls != 0 || g.h.planner.observe != 0 {
		t.Fatal("worker failure executed local fallback")
	}
	g.preserved(t)
}
