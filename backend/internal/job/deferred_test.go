package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
)

// deferredJob is work whose dispatch waits for its owner's approval. It carries a post so
// the enqueue guard has a subject, and nothing about it is a product's.
func deferredJob(user string) job.NewJob {
	return attach(job.NewJob{Kind: deferredKind, UserID: user, DeferHold: true, ObserveModel: "p/observe", WriteModel: "p/write"}, "post-a", "")
}

func TestDeferredWorkIsNotDispatchedOrHeldUntilItIsActivated(t *testing.T) {
	h := newHarness(t)
	a := &recordingAdmitter{}
	h.queue.Admit(a)
	ctx := context.Background()
	id, err := h.queue.Enqueue(ctx, deferredJob("alice"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.starts) != 0 {
		t.Fatal("deferred work took a hold at enqueue")
	}
	if _, err = h.store.PickNextQueued(ctx, time.Now()); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("unactivated work dispatched", err)
	}
	if err = h.queue.Activate(ctx, "bob", id); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("a foreign activation released the work", err)
	}
	if err = h.queue.Activate(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	picked, err := h.store.PickNextQueued(ctx, time.Now())
	if err != nil || picked.ID != id {
		t.Fatal(picked, err)
	}
	if len(a.starts) != 0 {
		t.Fatal("activation is not a hold: the owner's approval reserves its own credits")
	}
}

func TestBootSweepFailsWorkStillWaitingForItsActivation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	id, err := h.queue.Enqueue(ctx, deferredJob("alice"))
	if err != nil {
		t.Fatal(err)
	}
	immediate, err := h.queue.Enqueue(ctx, attach(job.NewJob{Kind: "immediate", UserID: "alice"}, "post-b", ""))
	if err != nil {
		t.Fatal(err)
	}
	n, err := h.queue.SweepUnactivated(ctx)
	if err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v; want only the unactivated job", n, err)
	}
	waiting, err := h.queue.Get(ctx, id, "alice")
	if err != nil || waiting.Status != job.StatusFailed || waiting.Failure.Reason != job.FailureReasonInterrupted {
		t.Fatal(waiting, err)
	}
	untouched, err := h.queue.Get(ctx, immediate, "alice")
	if err != nil || untouched.Status != job.StatusQueued {
		t.Fatal("the sweep touched work that never waited", untouched, err)
	}
}

func TestWorkThatSpendsNothingMayNotAlsoDeferAHold(t *testing.T) {
	h := newHarness(t)
	bad := deferredJob("alice")
	bad.NonMetered = true
	if _, err := h.queue.Enqueue(context.Background(), bad); !errors.Is(err, job.ErrInvalidTarget) {
		t.Fatal(err)
	}
}

func TestNonMeteredWorkNeverReachesTheLedger(t *testing.T) {
	h := newHarness(t)
	a := &recordingAdmitter{refuse: errors.New("no credits")}
	h.queue.Admit(a)
	ctx := context.Background()
	free := attach(job.NewJob{Kind: "free-work", UserID: "alice", NonMetered: true}, "post-a", "")
	id, err := h.queue.Enqueue(ctx, free)
	if err != nil {
		t.Fatal("a refusing ledger blocked work that spends nothing", err)
	}
	if len(a.starts) != 0 || len(a.released) != 0 {
		t.Fatal("free work touched the ledger", a)
	}
	if _, err := h.store.PickNextQueued(ctx, time.Now()); err != nil {
		t.Fatal("free work waits for an activation it never needs", err)
	}
	if _, err := h.queue.Get(ctx, id, "alice"); err != nil {
		t.Fatal(err)
	}
}
