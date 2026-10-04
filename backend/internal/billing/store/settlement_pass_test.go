package store_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/plan"
)

// passStore lets a test write between a billing pass's DueSubscriptions read and its steps.
type passStore struct {
	*billingstore.Store
	afterDue func()
}

func (s passStore) DueSubscriptions(ctx context.Context, at time.Time) ([]billing.Subscription, error) {
	due, err := s.Store.DueSubscriptions(ctx, at)
	if err == nil && s.afterDue != nil {
		s.afterDue()
	}
	return due, err
}

func addBillingUser(t *testing.T, h *ledgerHarness, id string, at time.Time) {
	t.Helper()
	if _, err := h.handle.Writer.Exec(`INSERT INTO users (id,password_hash,plan,email,email_verified_at,created_at)
		VALUES (?,'hash','free',?,?,?)`, id, id+"@example.com",
		at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
}

func packIntent(userID, orderID string, krw int, at time.Time) billing.Intent {
	return billing.Intent{OrderID: orderID, UserID: userID, Kind: "pack", PackID: "pack-1000", KRW: krw,
		BillingKey: "billing-key-" + userID, CustomerKey: billing.CustomerKey(userID),
		QuotedAt: at, Status: "pending", CreatedAt: at, UpdatedAt: at}
}

func intentStatus(t *testing.T, h *ledgerHarness, orderID string) string {
	t.Helper()
	var status string
	if err := h.handle.Reader.QueryRow(`SELECT status FROM billing_intents WHERE order_id=?`, orderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

// F2: one order the pass cannot settle no longer stops every account's renewal. A captured
// payment that no longer fits its order goes to review; a transient failure is logged and the
// order waits for the next pass.
func TestOneUnsettledOrderDoesNotStopThePassRenewing(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 2, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))

	t.Run("pack price changed while the order was pending goes to review", func(t *testing.T) {
		h, provider, clock := fixedService(t, at)
		sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
		if err != nil {
			t.Fatal(err)
		}
		addBillingUser(t, h, "bob", at)
		// Quoted at a price the pack no longer has, so applying it answers ErrInvalidPack.
		if err := h.store.InsertIntent(ctx, packIntent("bob", "pp-buy-stale", 2500, at)); err != nil {
			t.Fatal(err)
		}
		*clock = sub.TermEnd
		if err := h.service.RunDue(ctx, *clock); err != nil {
			t.Fatalf("RunDue = %v", err)
		}
		if got := intentStatus(t, h, "pp-buy-stale"); got != "review" {
			t.Fatalf("stale pack order status = %s, want review", got)
		}
		renewed, _, err := h.store.Subscription(ctx, "alice")
		if err != nil || !renewed.TermStart.Equal(sub.TermEnd) || renewed.Status != "active" {
			t.Fatalf("renewal blocked by the stale order: %+v err=%v", renewed, err)
		}
		var purchases int
		if err := h.handle.Reader.QueryRow(`SELECT count(*) FROM credit_purchases WHERE user_id='bob'`).Scan(&purchases); err != nil || purchases != 0 {
			t.Fatalf("stale pack granted: purchases=%d err=%v", purchases, err)
		}
		if provider.charges["pp-buy-stale"] != 1 {
			t.Fatalf("stale order charges = %v", provider.charges)
		}
	})

	t.Run("a failed local write is logged and the pass still renews", func(t *testing.T) {
		h, _, clock := fixedService(t, at)
		sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
		if err != nil {
			t.Fatal(err)
		}
		addBillingUser(t, h, "bob", at)
		if err := h.store.InsertIntent(ctx, packIntent("bob", "pp-buy-blocked", 3000, at)); err != nil {
			t.Fatal(err)
		}
		if _, err := h.handle.Writer.ExecContext(ctx, `CREATE TRIGGER block_purchase BEFORE INSERT ON credit_purchases
			BEGIN SELECT RAISE(ABORT, 'simulated disk failure'); END`); err != nil {
			t.Fatal(err)
		}
		*clock = sub.TermEnd
		if err := h.service.RunDue(ctx, *clock); err != nil {
			t.Fatalf("RunDue = %v", err)
		}
		if got := intentStatus(t, h, "pp-buy-blocked"); got != "pending" {
			t.Fatalf("transiently failed order status = %s, want pending", got)
		}
		renewed, _, err := h.store.Subscription(ctx, "alice")
		if err != nil || !renewed.TermStart.Equal(sub.TermEnd) {
			t.Fatalf("renewal blocked by the failed order: %+v err=%v", renewed, err)
		}
	})
}

// F11: a charge whose outcome is unknown is never a failed order. Only a refusal from the
// docs' definitive set lapses a renewal; the rest stay pending and settle on a later pass.
func TestUnresolvedChargeAnswerLeavesTheRenewalPending(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 3, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"rate limited", &billing.ProviderError{HTTPStatus: 429}},
		{"server error", &billing.ProviderError{Code: "FAILED_INTERNAL_SYSTEM_PROCESSING", HTTPStatus: 500}},
		{"duplicate order", &billing.ProviderError{Code: "DUPLICATED_ORDER_ID", HTTPStatus: 400}},
		{"in-flight idempotent request", &billing.ProviderError{Code: "IDEMPOTENT_REQUEST_PROCESSING", HTTPStatus: 409}},
		{"transport", errors.New("connection reset")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, provider, clock := fixedService(t, at)
			sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
			if err != nil {
				t.Fatal(err)
			}
			*clock = sub.TermEnd
			provider.chargeErr = tc.err
			if err := h.service.RunDue(ctx, *clock); err != nil {
				t.Fatalf("RunDue = %v", err)
			}
			var order string
			if err := h.handle.Reader.QueryRow(`SELECT order_id FROM billing_intents WHERE kind='renew'`).Scan(&order); err != nil {
				t.Fatal(err)
			}
			if got := intentStatus(t, h, order); got != "pending" {
				t.Fatalf("renewal order status = %s, want pending", got)
			}
			held, _, err := h.store.Subscription(ctx, "alice")
			if err != nil || held.Status != "active" {
				t.Fatalf("unresolved charge lapsed the subscription: %+v err=%v", held, err)
			}
			var failures int
			if err := h.handle.Reader.QueryRow(`SELECT count(*) FROM billing_events
				WHERE kind IN ('renewal_failed','charge_failed')`).Scan(&failures); err != nil || failures != 0 {
				t.Fatalf("failure events=%d err=%v", failures, err)
			}

			provider.chargeErr = nil
			*clock = clock.Add(settledByWorker)
			if err := h.service.RunDue(ctx, *clock); err != nil {
				t.Fatalf("settling RunDue = %v", err)
			}
			if got := intentStatus(t, h, order); got != "applied" {
				t.Fatalf("renewal order status after settling = %s", got)
			}
			renewed, _, err := h.store.Subscription(ctx, "alice")
			if err != nil || !renewed.TermStart.Equal(sub.TermEnd) || renewed.Status != "active" {
				t.Fatalf("settled renewal=%+v err=%v", renewed, err)
			}
		})
	}
}

// F11: the worker leaves an order its request path created moments ago alone.
func TestWorkerLeavesAFreshOrderToItsRequestPath(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 4, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	if err := h.store.InsertIntent(ctx, packIntent("alice", "pp-buy-fresh", 3000, at)); err != nil {
		t.Fatal(err)
	}
	if fresh, err := h.store.DueIntents(ctx, at.Add(-time.Second)); err != nil || len(fresh) != 0 {
		t.Fatalf("fresh order listed: %+v err=%v", fresh, err)
	}
	*clock = at.Add(time.Minute)
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	if provider.charges["pp-buy-fresh"] != 0 || intentStatus(t, h, "pp-buy-fresh") != "pending" {
		t.Fatalf("worker raced the request path: charges=%v", provider.charges)
	}
	*clock = at.Add(settledByWorker)
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	if provider.charges["pp-buy-fresh"] != 1 || intentStatus(t, h, "pp-buy-fresh") != "applied" {
		t.Fatalf("aged order not settled: charges=%v", provider.charges)
	}
}

// F38: the annual benefit step moves next_grant_at alone, so a cancel written after the pass
// read the row survives it — the customer is not renewed and charged at term end.
func TestAnnualBenefitStepKeepsACancelWrittenDuringThePass(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	*clock = sub.NextGrantAt
	cancelledAt := clock.Add(time.Second)
	hooked := passStore{Store: h.store, afterDue: func() {
		current, _, err := h.store.Subscription(ctx, "alice")
		if err != nil {
			t.Error(err)
			return
		}
		current.AutoRenew, current.UpdatedAt = false, cancelledAt
		if err := h.store.UpsertSubscription(ctx, current); err != nil {
			t.Error(err)
		}
	}}
	service := billing.NewService(hooked, provider, nil, testCredits{Service: h.ledger}, nil, nil, nil).
		WithFixedKRW().WithClock(func() time.Time { return *clock })
	if err := service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	after, _, err := h.store.Subscription(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if after.AutoRenew || !after.UpdatedAt.Equal(cancelledAt) || after.Tier != plan.Basic || after.Status != "active" {
		t.Fatalf("benefit step reverted the cancel: %+v", after)
	}
	if !after.NextGrantAt.Equal(plan.MonthBoundary(at, 2)) {
		t.Fatalf("next grant = %s, want %s", after.NextGrantAt, plan.MonthBoundary(at, 2))
	}
	if moved, err := h.store.AdvanceNextGrant(ctx, "alice", sub.NextGrantAt, plan.MonthBoundary(at, 3)); err != nil || moved {
		t.Fatalf("advance from a stale boundary moved=%t err=%v", moved, err)
	}
	*clock = sub.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	ended, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || ended.Status != "lapsed" || len(provider.charges) != 1 {
		t.Fatalf("cancelled annual at term end=%+v charges=%v err=%v", ended, provider.charges, err)
	}
}

// F12: a renewal the pass skips because the account has an order in review is logged with the
// account and the order, never skipped silently.
func TestRenewalBlockedByAnOrderInReviewIsLogged(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 6, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.InsertIntent(ctx, packIntent("alice", "pp-buy-review", 3000, at)); err != nil {
		t.Fatal(err)
	}
	if marked, err := h.store.MarkIntent(ctx, "pp-buy-review", "review", "DONE", "pay-review", at); err != nil || !marked {
		t.Fatalf("review mark=%t err=%v", marked, err)
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	*clock = sub.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	line := ""
	for _, candidate := range strings.Split(logs.String(), "\n") {
		if strings.Contains(candidate, "renewal skipped") {
			line = candidate
		}
	}
	if !strings.Contains(line, "user_id=alice") || !strings.Contains(line, "order_id=pp-buy-review") {
		t.Fatalf("skip log = %q\nall logs:\n%s", line, logs.String())
	}
	held, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || !held.TermEnd.Equal(sub.TermEnd) || len(provider.charges) != 1 {
		t.Fatalf("blocked renewal ran: %+v charges=%v err=%v", held, provider.charges, err)
	}
}

// F21: the billing pass deletes the quotes that expired more than a day ago and keeps the rest.
func TestBillingPassPurgesOnlyLongExpiredQuotes(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, _, clock := fixedService(t, at)
	for id, expires := range map[string]time.Time{
		"q-old":    at.Add(-48 * time.Hour),
		"q-recent": at.Add(-time.Hour),
		"q-live":   at.Add(5 * time.Minute),
	} {
		if err := h.store.PutQuote(ctx, billing.QuoteRecord{ID: id, UserID: "alice", Tier: plan.Basic,
			Term: billing.TermMonthly, KRW: 4900, EffectiveAt: at, SubscriptionUpdatedAt: at,
			QuotedAt: expires.Add(-10 * time.Minute), ExpiresAt: expires}); err != nil {
			t.Fatal(err)
		}
	}
	*clock = at
	if err := h.service.RunDue(ctx, at); err != nil {
		t.Fatal(err)
	}
	for id, kept := range map[string]bool{"q-old": false, "q-recent": true, "q-live": true} {
		if _, found, err := h.store.Quote(ctx, id); err != nil || found != kept {
			t.Fatalf("quote %s found=%t, want %t (err=%v)", id, found, kept, err)
		}
	}
}
