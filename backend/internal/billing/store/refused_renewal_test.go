package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/plan"
)

// BILL-8: a renewal the provider refuses ends paid coverage while the subscription is still on
// the term it paid for, even when another write touched the row after the order was created —
// and the account never collides with that renewal's order id again.
func TestARefusedRenewalLapsesASubscriptionChangedSinceItsOrder(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 3, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	*clock = sub.TermEnd
	provider.chargeErr = &billing.ProviderError{Code: "FAILED_INTERNAL_SYSTEM_PROCESSING", HTTPStatus: 500}
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	var order string
	if err := h.handle.Reader.QueryRow(`SELECT order_id FROM billing_intents WHERE kind='renew'`).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if got := intentStatus(t, h, order); got != "pending" {
		t.Fatalf("renewal after a transient answer = %s, want pending", got)
	}
	// Another writer touches the subscription while its renewal is pending.
	touched, _, err := h.store.Subscription(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	touched.UpdatedAt = clock.Add(time.Second)
	if err := h.store.UpsertSubscription(ctx, touched); err != nil {
		t.Fatal(err)
	}

	provider.chargeErr, provider.decline = nil, true
	*clock = clock.Add(settledByWorker)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("refusing RunDue = %v", err)
	}
	if got := intentStatus(t, h, order); got != "failed" {
		t.Fatalf("refused renewal = %s, want failed", got)
	}
	lapsed, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || lapsed.Status != "lapsed" {
		t.Fatalf("subscription after a refused renewal = %+v, %v", lapsed, err)
	}
	var tier string
	if err := h.handle.Reader.QueryRow(`SELECT plan FROM users WHERE id='alice'`).Scan(&tier); err != nil || tier != "free" {
		t.Fatalf("plan after a refused renewal = %q, %v", tier, err)
	}
	var failed int
	if err := h.handle.Reader.QueryRow(`SELECT count(*) FROM billing_events WHERE user_id='alice' AND kind='renewal_failed'`).Scan(&failed); err != nil || failed != 1 {
		t.Fatalf("renewal_failed events = %d, %v", failed, err)
	}

	charges := provider.charges[order]
	*clock = clock.Add(24 * time.Hour)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("a later pass = %v", err)
	}
	if provider.charges[order] != charges {
		t.Fatalf("a later pass charged the lapsed renewal again: %d → %d", charges, provider.charges[order])
	}
}
