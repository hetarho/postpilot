package main

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

// A revision request is the one charged owner action in step ② (CLIP-20,
// CLIP-132): it reserves its writing calls through the same guard a generation
// uses, meters them at the same boundary and settles against the same approved
// ceiling. Every one of those three gates named `generate_clip` alone, so the
// very first revision died in prepare with no reserved allowance and a failure
// reason that could name neither the check nor the cause.
func revisionReservation() clipapp.Reservation {
	r := cancellationReservation()
	// No observation call is repaid: the recorded observations are the evidence
	// the rewrite is bound to. Two writing calls is a flow-and-narration target.
	r.Calls[0].Count = 0
	r.Calls[1].Count = 2
	return r
}

func reserveRevision(h *cancellationHarness, ctx context.Context, id string) (context.Context, error) {
	return clipapp.NewJobs(h.queue, h.guard).Reserve(ctx, "alice", id, []job.PlannedCall{{Ref: "p/w", Count: 2, CompletionTokens: 32768}}, revisionReservation())
}

// writeCall is a reserved writing call as the planner sends it: the frozen
// policy travels with the request, which is what the metered boundary matches
// against the allowance before any provider is reached.
func writeCall() (llm.ModelRef, llm.Request) {
	w := revisionReservation().Calls[1].Policy
	return w.Ref, llm.Request{Stage: w.Stage, MaxTokens: w.CompletionTokens, Reasoning: w.Reasoning,
		Execution: &llm.ExecutionPolicy{Call: w, Delivery: llm.ExecutionTextOnly, NoFallback: true, RequireParameters: true},
		Messages:  []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart("rewrite")}}}}
}

func TestClipRevisionReservesMetersAndSettlesLikeAGeneration(t *testing.T) {
	h := newCancellationHarness(t, nil)
	id := h.enqueue(t, clip.JobKindRevise)
	metered := make(chan struct{})
	terminal := h.run(t, clip.JobKindRevise, func(ctx context.Context, j job.Job, _ job.Progress) error {
		admitted, err := reserveRevision(h, ctx, j.ID)
		if err != nil {
			return err
		}
		ref, r := writeCall()
		work := usage.WithWork(admitted, usage.Work{UserID: "alice", Kind: clip.JobKindRevise, JobID: j.ID})
		// The empty registry cannot resolve the model, so the call stops there.
		// What matters is that it got that far: a refusal of the reserved call as
		// unmetered work is the boundary rejecting the revision for its kind.
		if _, err := (meteredRegistry{Registry: &llm.Registry{}, ledger: h.ledger}).Complete(work, ref, r); errors.Is(err, clip.ErrCreditAllowance) {
			return errors.New("the metered boundary refused a reserved revision call")
		}
		close(metered)
		return nil
	})
	if err := h.queue.Activate(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	awaitCancellationSignal(t, metered)
	awaitCancellationSignal(t, terminal)
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil || j.Status != job.StatusDone {
		t.Fatal(j, err)
	}
	a, err := h.ledger.ReservationAccounting(t.Context(), "alice", id)
	if err != nil || a == nil || !a.Settled {
		t.Fatal("a charged revision left no settled hold", a, err)
	}
	if a.Approved == nil || *a.Approved != 5 || a.CancellationPolicyVersion != 1 {
		t.Fatal("the revision reserved outside its approved ceiling", a)
	}
	if a.FinalCharge == nil || *a.FinalCharge > 5 {
		t.Fatal("the revision charged past the ceiling the owner approved", a)
	}
	h.assertPreviousResult(t)
}

func TestClipRevisionCancellationSettlesUnderItsPolicy(t *testing.T) {
	h := newCancellationHarness(t, nil)
	id := h.enqueue(t, clip.JobKindRevise)
	reserved := make(chan struct{})
	terminal := h.run(t, clip.JobKindRevise, func(ctx context.Context, j job.Job, _ job.Progress) error {
		if _, err := reserveRevision(h, ctx, j.ID); err != nil {
			return err
		}
		close(reserved)
		<-ctx.Done()
		return ctx.Err()
	})
	if err := h.queue.Activate(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	awaitCancellationSignal(t, reserved)
	if _, err := h.queue.Cancel(t.Context(), "alice", job.Subject{Dimension: clip.JobSubject, ID: "clip"}, id); err != nil {
		t.Fatal(err)
	}
	awaitCancellationSignal(t, terminal)
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil || j.Status != job.StatusCancelled {
		t.Fatal(j, err)
	}
	// A cancelled revision settles exactly as a cancelled generation does; a
	// settlement that refuses the outcome would leave the hold open forever.
	a, err := h.ledger.ReservationAccounting(t.Context(), "alice", id)
	if err != nil || a == nil || !a.Settled {
		t.Fatal("a cancelled revision left its hold unsettled", a, err)
	}
	if a.ConfirmedCharge == nil || *a.ConfirmedCharge != 0 || a.CancellationFee == nil {
		t.Fatal("the cancellation charged confirmed work that never ran", a)
	}
	h.assertPreviousResult(t)
}
