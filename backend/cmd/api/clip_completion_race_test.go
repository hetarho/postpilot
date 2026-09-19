package main

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

func TestClipResultCommitRacingCancellationKeepsOneOutcome(t *testing.T) {
	for range 8 {
		h := newCancellationHarness(t, nil)
		id := h.enqueue(t, clip.JobKindGenerate)
		ready, compete := make(chan struct{}), make(chan struct{})
		terminal := h.run(t, clip.JobKindGenerate, func(ctx context.Context, j job.Job, _ job.Progress) error {
			if _, err := h.reserve(ctx, j.ID); err != nil {
				return err
			}
			candidate := h.candidate(j.ID, false)
			close(ready)
			<-compete
			return h.finisher.Complete(ctx, candidate)
		})
		if err := h.queue.Activate(t.Context(), "alice", id); err != nil {
			t.Fatal(err)
		}
		awaitCancellationSignal(t, ready)
		cancelled := make(chan error, 1)
		go func() {
			<-compete
			_, err := h.queue.Cancel(t.Context(), "alice", job.Subject{Dimension: clip.JobSubject, ID: "clip"}, id)
			cancelled <- err
		}()
		close(compete)
		if err := <-cancelled; err != nil {
			t.Fatal(err)
		}
		awaitCancellationSignal(t, terminal)
		j, err := h.jobs.GetByID(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		wantCharge, wantFee := 2, 0
		switch j.Status {
		case job.StatusCancelled:
			wantCharge, wantFee = 3, 3
			h.assertPreviousResult(t)
			if j.CancelRequestedAt == nil {
				t.Fatal("cancellation lost its durable cause")
			}
		case job.StatusDone:
			p, err := h.clips.GetProject(t.Context(), "alice", "clip")
			if err != nil || p.Result == nil || p.Result.Key != "clip-results/alice/clip/new.mp4" || p.EditPlanRevision != 2 || j.CancelRequestedAt != nil {
				t.Fatal("successful commit was changed by cancellation", p, j, err)
			}
		default:
			t.Fatal("race produced a third outcome", j)
		}
		if err := h.finisher.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		a, err := h.ledger.ReservationAccounting(t.Context(), "alice", id)
		if err != nil || a == nil || !a.Settled || a.FinalCharge == nil || *a.FinalCharge != wantCharge || a.CancellationFee == nil || *a.CancellationFee != wantFee || *a.Refund != 5-wantCharge {
			t.Fatal("settlement disagrees with durable result", a, err)
		}
	}
}
