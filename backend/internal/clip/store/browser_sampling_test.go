package store_test

import (
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/clip/workerclient"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

// samplingSetup is the remote media harness with its server render ended, so
// the project is free for a browser render.
func samplingSetup(t *testing.T) (*remoteRender, string, string) {
	t.Helper()
	g := remoteRenderSetup(t, false)
	if _, err := g.h.queue.FailQueued(t.Context(), g.jobID, "alice", job.Failure{Reason: "JOB_INTERRUPTED"}); err != nil {
		t.Fatal(err)
	}
	if err := g.h.sources.ReleaseAttempt(t.Context(), "alice", g.jobID, time.Now()); err != nil {
		t.Fatal(err)
	}
	render, sampling, err := g.h.service.StartBrowserRender(t.Context(), "alice", g.before.ID, g.batch.ID, g.before.EditPlanRevision)
	if err != nil {
		t.Fatal(err)
	}
	if render == "" || sampling == "" {
		t.Fatalf("a browser render started as %q waiting on %q", render, sampling)
	}
	return g, render, sampling
}

func (g *remoteRender) sample(t *testing.T, j job.Job) error {
	t.Helper()
	return g.h.service.RunBrowserSampling(t.Context(), j.UserID, j.ID, j.Subject(clip.JobSubject), j.Payload, func(stage string, n, total int) {
		if err := g.h.jobs.UpdateProgress(t.Context(), j.ID, stage, n, total, time.Now()); err != nil {
			t.Fatal(err)
		}
	})
}

func (g *remoteRender) claimOp(t *testing.T, op clip.MediaOperation) (*workerclient.Client, *clip.MediaWork) {
	t.Helper()
	p := mediaProfile()
	p.Operation = op
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
				t.Fatal("two workers claimed one stage")
			}
			winner = i
		}
	}
	if winner < 0 {
		return nil, nil
	}
	return g.clients[winner], work[winner]
}

// measured is what a worker reports for a sample stage: the verified originals
// and the grounds of the frozen plan.
func measured(t *testing.T, w *clip.MediaWork, grounds []clip.SampledGround) (clip.MediaResult, string) {
	t.Helper()
	task, err := mediacodec.DecodeTask(w.Payload)
	if err != nil {
		t.Fatal(err)
	}
	result := clip.MediaResult{Version: clip.MediaContractVersion, Grounds: grounds}
	for _, s := range task.Sources {
		result.Sources = append(result.Sources, clip.MediaVerifiedSource{ID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info})
	}
	raw, err := mediacodec.EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	return result, raw
}

var outroGround = []clip.SampledGround{{InstanceID: "project-outro", Mean: .82, Sigma: .01, R: .95, G: .93, B: .9, Frames: []float64{.81, .82, .83}}}

// CLIP-192, ARCH-45: a browser render's grounds are sampled by a media worker
// under the render's own frozen task, then kept on the render for its assets.
// The API reads no footage; the job holds the project while it samples, waits
// on its stage across a restart, and refuses a measurement outside its bounds.
func TestABrowserRenderIsSampledOnAMediaWorker(t *testing.T) {
	g, render, sampling := samplingSetup(t)
	r, err := g.h.store.GetBrowserRender(t.Context(), "alice", render)
	if err != nil || r.SampleJobID != sampling || r.SampledAt != nil {
		t.Fatalf("the render does not name its sampling: %+v %v", r, err)
	}
	if _, err := g.h.service.StartRender(t.Context(), "alice", g.before.ID, g.batch.ID, g.before.EditPlanRevision, clip.RenderServer); !errors.Is(err, clip.ErrBusy) {
		t.Fatal("another render started beside a sampling job", err)
	}
	j := g.pick(t)
	if j.ID != sampling || j.Kind != clip.JobKindSampleBrowserRender {
		t.Fatalf("picked %s (%s), want the sampling job", j.ID, j.Kind)
	}
	if err := g.sample(t, j); err != job.ErrYield {
		t.Fatal(err)
	}
	wait, err := g.h.jobs.Continuation(t.Context(), sampling)
	if err != nil || wait.Policy != job.ReplaySafe {
		t.Fatal(wait, err)
	}
	if n, err := g.h.queue.SweepRunning(t.Context()); err != nil || n != 0 {
		t.Fatal("a restart lost the waiting sample stage", n, err)
	}
	if c, _ := g.claimOp(t, clip.MediaRender); c != nil {
		t.Fatal("a render worker claimed the sample stage")
	}
	c, w := g.claimOp(t, clip.MediaSample)
	if c == nil {
		t.Fatal("the sample stage is not claimable")
	}
	task, err := mediacodec.DecodeTask(w.Payload)
	if err != nil || task.Plan == "" || len(task.Sources) == 0 {
		t.Fatal("the stage does not carry the render's frozen task", err)
	}
	bad := outroGround[0]
	bad.Mean = 2
	if _, raw := measured(t, w, []clip.SampledGround{bad}); c.Complete(t.Context(), w.Credentials, raw) == nil {
		t.Fatal("a ground outside 0..1 was accepted")
	}
	result, raw := measured(t, w, outroGround)
	if err := c.Complete(t.Context(), w.Credentials, raw); err != nil {
		t.Fatal(err)
	}
	if err := g.sample(t, g.pick(t)); err != nil {
		t.Fatal(err)
	}
	r, err = g.h.store.GetBrowserRender(t.Context(), "alice", render)
	if err != nil || r.SampledAt == nil || !reflect.DeepEqual(r.Grounds, result.Grounds) {
		t.Fatalf("the render did not keep its grounds: %+v %v", r, err)
	}
	if g.h.media.probes != 0 || g.h.renderer.calls != 0 || g.h.planner.observe != 0 || len(g.h.admitter.calls) != 0 {
		t.Fatal("the API read footage or made a paid call")
	}
	// A second report of the same measurement changes nothing; another is refused.
	if err := g.h.store.SaveBrowserRenderGrounds(t.Context(), "alice", render, sampling, r.Revision, result.Grounds, time.Now()); err != nil {
		t.Fatal("the same measurement was refused on replay", err)
	}
	if err := g.h.store.SaveBrowserRenderGrounds(t.Context(), "alice", render, sampling, r.Revision, nil, time.Now()); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("a different measurement replaced the kept one", err)
	}
	if err := g.h.store.SaveBrowserRenderGrounds(t.Context(), "alice", render, "someone-else", r.Revision, result.Grounds, time.Now()); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("another job wrote the render's grounds", err)
	}
}

// A worker's failure ends the sampling with its reason and keeps no ground,
// and the API never samples in its place (CLIP-155, ARCH-45).
func TestAFailedSampleStageLeavesTheRenderUnsampled(t *testing.T) {
	g, render, _ := samplingSetup(t)
	j := g.pick(t)
	if err := g.sample(t, j); err != job.ErrYield {
		t.Fatal(err)
	}
	c, w := g.claimOp(t, clip.MediaSample)
	if err := c.Fail(t.Context(), w.Credentials, clip.MediaFailureInvalidInput); err != nil {
		t.Fatal(err)
	}
	var failure *clip.MediaStageFailure
	if err := g.sample(t, j); !errors.As(err, &failure) || failure.Code != clip.MediaFailureInvalidInput {
		t.Fatal("the stage's failure was not the job's", err)
	}
	r, err := g.h.store.GetBrowserRender(t.Context(), "alice", render)
	if err != nil || r.SampledAt != nil || r.Grounds != nil {
		t.Fatalf("a failed sampling kept grounds: %+v %v", r, err)
	}
	if g.h.media.probes != 0 || g.h.renderer.calls != 0 {
		t.Fatal("the API sampled after the worker failed")
	}
}

// Cancelling the browser render stops its sampling job, so no worker claims its
// stage and nothing is kept (ARCH-51); the job spends nothing and may always stop.
func TestCancellingABrowserRenderStopsItsSampling(t *testing.T) {
	for _, when := range []string{"queued", "waiting"} {
		t.Run(when, func(t *testing.T) {
			g, render, sampling := samplingSetup(t)
			if when == "waiting" {
				if err := g.sample(t, g.pick(t)); err != job.ErrYield {
					t.Fatal(err)
				}
			}
			cancelled, err := g.h.service.CancelBrowserRender(t.Context(), "alice", render)
			if err != nil || !cancelled {
				t.Fatal(cancelled, err)
			}
			j, err := g.h.jobs.GetByID(t.Context(), sampling)
			if err != nil || j.CancelRequestedAt == nil {
				t.Fatalf("the sampling job was not stopped: %+v %v", j, err)
			}
			// The media reconciler retires the stage of a stopped job, as it does
			// a server render's; from then on no worker can claim it.
			bind := func(tx *sql.Tx) clipapp.Ports {
				c := store.NewTx(tx)
				j := jobstore.NewTx(tx, jobKindsForTest())
				return clipapp.Ports{Clips: c, Jobs: j, Waits: j, Stages: c, Media: c, Publication: c, Recovery: c, Control: c}
			}
			reconciler := clipapp.NewMediaReconciler(g.h.db.Writer, bind, g.h.store, g.h.jobs, g.h.queue, &recoveryObjects{renderObjects: g.objects, writer: g.h.db.Writer}, time.Minute, time.Now)
			if err := reconciler.Reconcile(t.Context()); err != nil {
				t.Fatal(err)
			}
			if c, _ := g.claimOp(t, clip.MediaSample); c != nil {
				t.Fatal("a worker claimed a cancelled render's stage")
			}
			r, err := g.h.store.GetBrowserRender(t.Context(), "alice", render)
			if err != nil || r.SampledAt != nil {
				t.Fatalf("a cancelled render kept grounds: %+v %v", r, err)
			}
		})
	}
}
