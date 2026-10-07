package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

func capacityPorts(h *generationHarness) clipapp.Binder {
	return func(tx *sql.Tx) clipapp.Ports {
		c, j := store.NewTx(tx), jobstore.NewTx(tx, jobKindsForTest())
		return clipapp.Ports{Jobs: j, Starts: j, Waits: j, RenderSources: c, Clips: c, Media: c, Stages: c, Publication: c, Recovery: c, Control: c, Exports: c}
	}
}

type capacityInput struct {
	start clip.GenerationStart
	batch clip.SourceBatch
	task  clip.MediaTask
}

// Reuse an already validated native recipe; these tests exercise its admission
// transaction, while media_render_test covers the complete worker/output path.
func capacityProject(t *testing.T, g *remoteRender, user string) capacityInput {
	t.Helper()
	h, ctx := g.h, t.Context()
	if _, err := authstore.New(h.db.Writer, h.db.Reader).GetUserPlan(ctx, user); err != nil {
		if err := authstore.New(h.db.Writer, h.db.Reader).CreateUser(ctx, auth.User{ID: user, PasswordHash: "hash", Plan: plan.Max, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	p, err := h.projects.CreateProject(ctx, user, clip.ProjectInput{Language: "ko", Title: user, Ratio: g.before.Ratio, TargetDurationMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	if err = h.store.SaveGeneratedPlan(ctx, user, p.ID, g.before.Analysis, g.before.EditPlan, "", nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	metadata := make([]clip.SourceMetadata, len(g.batch.Sources))
	for i, s := range g.batch.Sources {
		metadata[i] = s.SourceMetadata
	}
	upload, err := h.sources.Create(ctx, user, p.ID, metadata)
	if err != nil {
		t.Fatal(err)
	}
	h.objects.upload(upload.Batch)
	b := upload.Batch
	for _, s := range b.Sources {
		b, err = h.sources.Confirm(ctx, user, b.ID, s.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	original, err := h.jobs.GetByID(ctx, g.jobID)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(original.Payload, &raw); err != nil {
		t.Fatal(err)
	}
	raw["ProjectID"], _ = json.Marshal(p.ID)
	raw["Batch"], _ = json.Marshal(b)
	payload, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var task clip.MediaTask
	if err = json.Unmarshal(raw["Execution"], &task); err != nil {
		t.Fatal(err)
	}
	return capacityInput{clip.GenerationStart{UserID: user, ProjectID: p.ID, RenderOnly: true, Payload: payload}, b, task}
}

func nativeAdmission(t *testing.T, h *generationHarness, l clip.RenderCapacity) *clipapp.RenderAdmission {
	t.Helper()
	a, err := clipapp.NewRenderAdmission(h.db.Writer, capacityPorts(h), l, clip.DefaultMediaStageLimits(clip.Environment{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestNativeCapacityConcurrentAdmissionDuplicateAndAccountBounds(t *testing.T) {
	g := remoteRenderSetup(t, false)
	h, ctx := g.h, t.Context()
	// A browser request must never reuse a native job identity or change kind.
	if render, sampling, err := h.service.StartBrowserRender(ctx, "alice", g.before.ID, g.batch.ID, g.before.EditPlanRevision); !errors.Is(err, clip.ErrBusy) || render != "" || sampling != "" {
		t.Fatal(render, sampling, err)
	}
	// The ordinary public start is idempotent while its accepted job is active.
	for range 2 {
		id, err := h.service.StartRender(ctx, "alice", g.before.ID, g.batch.ID, g.before.EditPlanRevision, clip.RenderServer)
		if err != nil || id != g.jobID {
			t.Fatal(id, err)
		}
	}
	a := nativeAdmission(t, h, clip.DefaultRenderCapacity(clip.Environment{}))
	var seq int
	var name, filename string
	if err := h.db.Reader.QueryRowContext(ctx, "PRAGMA database_list").Scan(&seq, &name, &filename); err != nil {
		t.Fatal(err)
	}
	second, err := db.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { second.Close() })
	otherAPI, err := clipapp.NewRenderAdmission(second.Writer, capacityPorts(h), clip.DefaultRenderCapacity(clip.Environment{}), clip.DefaultMediaStageLimits(clip.Environment{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]capacityInput, 12)
	for i := range inputs {
		inputs[i] = capacityProject(t, g, fmt.Sprintf("capacity-%d", i))
		in := inputs[i]
		now := time.Now()
		if err := h.store.OpenExportWindow(ctx, clip.ExportWindow{UserID: in.start.UserID, CoverageID: in.start.UserID, Start: now.Add(-time.Hour), End: now.Add(time.Hour), Allowance: 1}, in.start.UserID); err != nil {
			t.Fatal(err)
		}
	}
	type outcome struct {
		id    string
		err   error
		input capacityInput
	}
	results := make(chan outcome, len(inputs))
	var wg sync.WaitGroup
	for i, in := range inputs {
		actor := a
		if i%2 == 1 {
			actor = otherAPI
		}
		wg.Go(func() {
			id, err := actor.Admit(ctx, in.start, in.batch.ID, 1, in.task, false)
			results <- outcome{id, err, in}
		})
	}
	wg.Wait()
	close(results)
	accepted, refused := 0, 0
	var winner, rejected capacityInput
	for r := range results {
		if r.err == nil {
			accepted++
			winner = r.input
			// Lost replies/repeated starts reserve no second capacity or monthly count.
			id, err := a.Admit(ctx, r.input.start, r.input.batch.ID, 1, r.input.task, false)
			if err != nil || id != r.id {
				t.Fatal(id, r.id, err)
			}
		} else if errors.Is(r.err, clip.ErrRenderOverloaded) {
			refused++
			rejected = r.input
		} else {
			t.Fatal(r.err)
		}
	}
	if accepted != 2 || refused != 10 {
		t.Fatal(accepted, refused)
	}
	var held, used, count int
	if err := h.db.Reader.QueryRowContext(ctx, "SELECT SUM(reserved),SUM(used) FROM server_export_windows").Scan(&held, &used); err != nil || held != 2 || used != 0 {
		t.Fatal(held, used, err)
	}
	if err := h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM generation_jobs WHERE kind='render_clip' AND status IN ('queued','running')").Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	other := capacityProject(t, g, winner.start.UserID)
	_, err = a.Admit(ctx, other.start, other.batch.ID, 1, other.task, true)
	if !errors.Is(err, clip.ErrRenderAccountBusy) {
		t.Fatal(err)
	}
	// Lowering admission limits preserves every accepted identity.
	lower := nativeAdmission(t, h, clip.RenderCapacity{Active: 1, Waiting: 0, PerAccount: 1})
	if _, err = lower.Admit(ctx, rejected.start, rejected.batch.ID, 1, rejected.task, true); !errors.Is(err, clip.ErrRenderOverloaded) {
		t.Fatal(err)
	}

	if len(h.admitter.calls) != 0 {
		t.Fatal("render admission spent AI credits")
	}
}

func TestNativeAdmissionRollbackAndOriginWindowRecovery(t *testing.T) {
	g := remoteRenderSetup(t, false)
	h, ctx := g.h, t.Context()
	in := capacityProject(t, g, "bob")
	now := time.Now().UTC()
	if err := h.store.OpenExportWindow(ctx, clip.ExportWindow{UserID: "bob", CoverageID: "origin", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Allowance: 1}, "origin"); err != nil {
		t.Fatal(err)
	}
	a := nativeAdmission(t, h, clip.DefaultRenderCapacity(clip.Environment{}))
	if _, err := a.Admit(ctx, in.start, in.batch.ID, 2, in.task, false); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
	w, _, err := h.store.CurrentExportWindow(ctx, "bob", now)
	if err != nil || w.Reserved != 0 || w.Used != 0 {
		t.Fatal(w, err)
	}
	b, err := h.store.GetSourceBatch(ctx, "bob", in.batch.ID)
	if err != nil || b.State != "ready" || b.JobID != "" {
		t.Fatal(b, err)
	}
	id, err := a.Admit(ctx, in.start, in.batch.ID, 1, in.task, false)
	if err != nil {
		t.Fatal(err)
	}
	j, err := h.jobs.GetByID(ctx, id)
	if err != nil || !j.DispatchReady || j.WaitExpiresAt == nil || j.WaitExpiresAt.Sub(j.CreatedAt) != clip.MediaWaitTimeout {
		t.Fatal(j, err)
	}
	// A new constructor/API process sees the same capacity and origin deadline.
	restarted := nativeAdmission(t, h, clip.DefaultRenderCapacity(clip.Environment{}))
	if got, err := restarted.Admit(ctx, in.start, in.batch.ID, 1, in.task, false); err != nil || got != id {
		t.Fatal(got, err)
	}
	if _, err = h.db.Writer.ExecContext(ctx, "UPDATE server_export_windows SET window_end=? WHERE coverage_id='origin'", now.Add(-time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err = h.store.OpenExportWindow(ctx, clip.ExportWindow{UserID: "bob", CoverageID: "next", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Allowance: 1}, "next"); err != nil {
		t.Fatal(err)
	}
	if _, err = h.queue.FailQueued(ctx, id, "bob", job.Failure{Reason: "CLIP_MEDIA_WAIT_EXPIRED"}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = h.store.RecoverExports(ctx); err != nil {
			t.Fatal(err)
		}
	}
	w, _, err = h.store.CurrentExportWindow(ctx, "bob", now)
	if err != nil || w.Allowance != 1 || w.Reserved != 0 || w.Used != 0 {
		t.Fatal("expired origin revived allowance", w, err)
	}
	var reserved int
	if err = h.db.Reader.QueryRowContext(ctx, "SELECT reserved FROM server_export_windows WHERE coverage_id='origin'").Scan(&reserved); err != nil || reserved != 0 {
		t.Fatal(reserved, err)
	}
}

func TestNativeClaimLimitIncludesCancelledLeasesButExcludesAnalysis(t *testing.T) {
	g := remoteRenderSetup(t, false)
	h, ctx := g.h, t.Context()
	a := nativeAdmission(t, h, clip.DefaultRenderCapacity(clip.Environment{}))
	in := capacityProject(t, g, "bob")
	if _, err := a.Admit(ctx, in.start, in.batch.ID, 1, in.task, true); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	first, err := h.store.ClaimMediaStage(ctx, mediaProfile(), now)
	if err != nil || first == nil {
		t.Fatal(first, err)
	}
	if next, err := h.store.ClaimMediaStage(ctx, mediaProfile(), now); err != nil || next != nil {
		t.Fatal("second native execution", next, err)
	}
	if err = h.store.SetMediaRecoveryState(ctx, first.Stage.ID, clip.MediaCancelled, "", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if next, err := h.store.ClaimMediaStage(ctx, mediaProfile(), now); err != nil || next != nil {
		t.Fatal("cancelled worker still owns execution", next, err)
	}
	// An unrelated bounded analysis stage uses independent execution accounting.
	input := first.Stage.MediaStageInput
	input.ID = "analysis-independent"
	input.ParentJobID = first.Stage.ParentJobID
	input.Operation = clip.MediaPrepare
	if _, err = h.store.CreateMediaStage(ctx, input, now); err != nil {
		t.Fatal(err)
	}
	profile := mediaProfile()
	profile.Operation = clip.MediaPrepare
	if lease, err := h.store.ClaimMediaStage(ctx, profile, now); err != nil || lease == nil {
		t.Fatal("native queue blocked analysis", lease, err)
	}
	// Worker loss/expired lease opens native execution exactly once; stale tokens
	// remain fenced by the existing completion and heartbeat guards.
	later := first.ExpiresAt.Add(time.Second)
	next, err := h.store.ClaimMediaStage(ctx, mediaProfile(), later)
	if err != nil || next == nil {
		t.Fatal(next, err)
	}
	if third, err := h.store.ClaimMediaStage(ctx, mediaProfile(), later); err != nil || third != nil {
		t.Fatal(third, err)
	}
	if _, err = h.store.RenewMediaLease(ctx, first.Credentials, 0, later); !errors.Is(err, clip.ErrMediaCancelled) && !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal(err)
	}
}

func TestNativeQueueExpiryBeforeDispatchPreservesPreviousResult(t *testing.T) {
	g := remoteRenderSetup(t, false)
	h, ctx := g.h, t.Context()
	j, err := h.jobs.GetByID(ctx, g.jobID)
	if err != nil {
		t.Fatal(err)
	}
	now := j.CreatedAt.Add(time.Second)
	objects := &recoveryObjects{renderObjects: g.objects, writer: h.db.Writer}
	r := clipapp.NewMediaReconciler(h.db.Writer, capacityPorts(h), h.store, h.jobs, h.queue, objects, time.Minute, func() time.Time { return now })
	h.queue.OnTerminal(clip.JobKindRender, func(ctx context.Context, j job.Job, at time.Time) error {
		return errors.Join(h.service.ReleaseExport(ctx, j.ID), h.sources.ReleaseAttempt(ctx, j.UserID, j.ID, at))
	})
	if err = r.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	j, err = h.jobs.GetByID(ctx, g.jobID)
	if err != nil || j.Status != job.StatusQueued {
		t.Fatal("valid undispatched admission lost on restart", j, err)
	}
	now = j.WaitExpiresAt.Add(time.Second)
	for range 2 {
		if err = r.Reconcile(ctx); err != nil {
			t.Fatal(err)
		}
	}
	j, err = h.jobs.GetByID(ctx, g.jobID)
	if err != nil || j.Status != job.StatusFailed || j.Failure == nil || j.Failure.Reason != "CLIP_MEDIA_WAIT_EXPIRED" {
		t.Fatal(j, err)
	}
	p, err := h.store.GetProject(ctx, "alice", g.before.ID)
	if err != nil || p.Result == nil || p.Result.Key != g.before.Result.Key || p.EditPlan != g.before.EditPlan {
		t.Fatal("expiry replaced prior result", p, err)
	}
	count, err := h.jobs.ActiveCount(ctx, job.Filter{Kind: clip.JobKindRender})
	if err != nil || count != 0 {
		t.Fatal("expired admission occupied capacity", count, err)
	}
	if len(h.admitter.calls) != 0 {
		t.Fatal("expiry invoked AI")
	}
}
