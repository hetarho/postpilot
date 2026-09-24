package store_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
)

func TestMediaProgressFollowsDurableRenderRecovery(t *testing.T) {
	h := newRecovery(t)
	assertStage := func(want string) {
		t.Helper()
		j := h.job(t)
		if j.ID != h.jobID || j.Stage != want || j.ProgressDone != 0 || j.ProgressTotal != 0 || j.Status != job.StatusRunning {
			t.Fatal("fabricated progress or changed parent", j.ID, j.Stage, j.ProgressDone, j.ProgressTotal, j.Status)
		}
	}
	assertStage("render_wait")
	c, w := h.claim(t)
	assertStage("render")
	if err := c.Fail(t.Context(), w.Credentials, clip.MediaFailureWorkerLost); err != nil {
		t.Fatal(err)
	}
	assertStage("render_retry")
	h.restart()
	h.reconcile(t)
	assertStage("render_retry")
	h.now = h.stage(t).RetryNotBefore
	_, w = h.claim(t)
	assertStage("render")
	h.now = h.now.Add(h.stage(t).Limits.LeaseTTL + time.Second)
	h.reconcile(t)
	assertStage("render_retry")
	c, w = h.claim(t)
	assertStage("render")
	_, raw := h.upload(t, c, w)
	if err := c.Complete(t.Context(), w.Credentials, raw); err != nil {
		t.Fatal(err)
	}
	if j := h.drain(t); j.Status != job.StatusDone || j.ID != h.jobID {
		t.Fatal(j)
	}
}

func TestMediaProgressPreparationWaitAndRecoverySpendNoAI(t *testing.T) {
	g := remoteGenerationSetup(t)
	id := g.h.start(t)
	if err := g.runJob(t, g.pick(t)); err != job.ErrYield {
		t.Fatal(err)
	}
	bind := func(tx *sql.Tx) clipapp.Ports {
		p := g.bind(tx)
		c := store.NewTx(tx)
		p.Recovery, p.Control = c, c
		return p
	}
	c := clipapp.NewMediaControl(g.h.db.Writer, bind, g.h.store)
	for _, stage := range []string{"prepare_wait", "prepare", "prepare_retry"} {
		j, err := g.h.jobs.GetByID(t.Context(), id)
		if err != nil || j.Stage != stage || j.ProgressTotal != 0 || j.ProgressDone != 0 {
			t.Fatal(j.Stage, stage, j.ProgressDone, j.ProgressTotal, err)
		}
		if stage == "prepare_wait" {
			profile := mediaProfile()
			profile.Operation = clip.MediaPrepare
			if lease, claimErr := c.ClaimMediaStage(t.Context(), profile, time.Now()); claimErr != nil || lease == nil {
				t.Fatal(claimErr)
			}
		} else if stage == "prepare" {
			// A real expired lease is reclaimed by the owner, which records
			// retry state without entering the parked paid continuation.
			r := clipapp.NewMediaReconciler(g.h.db.Writer, bind, g.h.store, g.h.jobs, g.h.queue, &recoveryObjects{writer: g.h.db.Writer}, time.Minute, func() time.Time { return time.Now().Add(2 * time.Minute) })
			if err = r.Reconcile(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
	}
	if g.holds(t) != 0 || g.h.planner.observe != 0 {
		t.Fatal("progress recovery entered paid work")
	}
}
