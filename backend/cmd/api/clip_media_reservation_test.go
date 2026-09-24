package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/job"
)

// The preparation continuation reaches the production admission adapter. A
// lost reserve response can repeat the request, but cannot debit a second hold.
func TestMediaContinuationApprovedReservationIsIdempotent(t *testing.T) {
	h := newCancellationHarness(t, nil)
	id := h.enqueue(t, clip.JobKindGenerate)
	ctx := t.Context()
	if err := h.queue.Activate(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.jobs.PickNextQueued(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := h.jobs.Park(ctx, id, "clip-media:preparation", job.FailOnInterrupt, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.jobs.Wake(ctx, id, "clip-media:preparation", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.jobs.PickNextQueued(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := h.jobs.UpdateProgress(ctx, id, "prepare", 2, 2, time.Now()); err != nil {
		t.Fatal(err)
	}
	r := cancellationReservation()
	p := clip.GenerationPricing{Version: clip.PricingPolicyVersion, CancellationPolicyVersion: 1, Observe: r.Calls[0].Policy, Plan: r.Calls[1].Policy, Narration: r.Calls[1].Policy, ObservationCalls: 2, MaxCredits: 20}
	approval := clip.GenerationApproval{QuoteID: "approved", MaxCredits: 20, Pricing: p}
	jobs := clipapp.NewJobs(h.queue, h.guard)
	if _, err := jobs.ReserveApproved(ctx, "alice", id, approval, 2); err != nil {
		t.Fatal(err)
	}
	before, err := h.ledger.ReservationAccounting(ctx, "alice", id)
	if err != nil || before == nil || before.Settled {
		t.Fatal(before, err)
	}
	var balanceBefore int
	if err := h.db.Reader.QueryRow(`SELECT SUM(remaining) FROM credit_lots WHERE user_id='alice'`).Scan(&balanceBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := jobs.ReserveApproved(ctx, "alice", id, approval, 2); err != nil {
		t.Fatal(err)
	}
	after, err := h.ledger.ReservationAccounting(ctx, "alice", id)
	if err != nil || after == nil || !reflect.DeepEqual(before, after) {
		t.Fatal("duplicate reserve changed ledger", before, after, err)
	}
	approval.MaxCredits++
	approval.Pricing.MaxCredits++
	if _, err := jobs.ReserveApproved(ctx, "alice", id, approval, 2); err == nil {
		t.Fatal("conflicting approval changed a hold")
	}
	var count int
	if err := h.db.Reader.QueryRow(`SELECT COUNT(*) FROM usage_admissions WHERE job_id=?`, id).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate credit hold", count, err)
	}
	var balanceAfter int
	if err := h.db.Reader.QueryRow(`SELECT SUM(remaining) FROM credit_lots WHERE user_id='alice'`).Scan(&balanceAfter); err != nil || balanceAfter != balanceBefore {
		t.Fatal("reservation replay changed account balance", balanceBefore, balanceAfter, err)
	}
	h.assertPreviousResult(t)
}
