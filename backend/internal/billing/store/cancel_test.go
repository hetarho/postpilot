package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/plan"
)

// racingStore writes once right after the first subscription read outside a transaction, so a
// change lands between a request's read and its write transaction.
type racingStore struct {
	*billingstore.Store
	wrote *bool
	write func()
}

func (s racingStore) Subscription(ctx context.Context, userID string) (billing.Subscription, bool, error) {
	sub, found, err := s.Store.Subscription(ctx, userID)
	if err == nil && !*s.wrote {
		*s.wrote = true
		s.write()
	}
	return sub, found, err
}

// BILL-7: a fixed-KRW subscriber cancels auto-renewal at any later time, and the account then
// lapses at term end without a charge.
func TestAFixedKRWCancelSucceedsAfterTimeHasPassed(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(time.Hour)
	cancelled, err := h.service.CancelSubscription(ctx, "alice")
	if err != nil || cancelled.AutoRenew || cancelled.ScheduledTier != nil || cancelled.ScheduledTerm != nil {
		t.Fatalf("cancel an hour later = %+v, %v", cancelled, err)
	}
	stored, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || stored.AutoRenew || !stored.UpdatedAt.Equal(*clock) {
		t.Fatalf("stored after cancel = %+v, %v", stored, err)
	}
	var events int
	if err := h.handle.Reader.QueryRow(`SELECT COUNT(*) FROM billing_events WHERE user_id='alice' AND kind='cancel_scheduled'`).Scan(&events); err != nil || events != 1 {
		t.Fatalf("cancel_scheduled events = %d, %v", events, err)
	}
	charges := len(provider.charges)
	*clock = sub.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	ended, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || ended.Status != "lapsed" || len(provider.charges) != charges {
		t.Fatalf("cancelled monthly at term end = %+v, charges %d → %d, %v", ended, charges, len(provider.charges), err)
	}
}

// A subscription row written between the cancel's read and its transaction is not overwritten:
// the cancel is refused and the row stays as the other writer left it.
func TestAFixedKRWCancelRefusesARowChangedSinceItsRead(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(time.Hour)
	changedAt := clock.Add(time.Second)
	wrote := false
	racing := racingStore{Store: h.store, wrote: &wrote, write: func() {
		current, _, err := h.store.Subscription(ctx, "alice")
		if err != nil {
			t.Error(err)
			return
		}
		current.UpdatedAt = changedAt
		if err := h.store.UpsertSubscription(ctx, current); err != nil {
			t.Error(err)
		}
	}}
	service := billing.NewService(racing, provider, nil, testCredits{Service: h.ledger}, nil, nil, nil).
		WithFixedKRW().WithClock(func() time.Time { return *clock })
	if _, err := service.CancelSubscription(ctx, "alice"); !errors.Is(err, billing.ErrStaleQuote) {
		t.Fatalf("cancel over a changed row = %v, want ErrStaleQuote", err)
	}
	stored, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || !stored.AutoRenew || !stored.UpdatedAt.Equal(changedAt) {
		t.Fatalf("row after the refused cancel = %+v, %v", stored, err)
	}
}
