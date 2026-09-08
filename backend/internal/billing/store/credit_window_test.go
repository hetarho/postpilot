package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// ledgerHarness drives billing against the REAL credit ledger over real SQL.
//
// It exists because every Credits fake in the billing package answers nil unconditionally,
// so the credit half of subscribing and upgrading had never run against a statement: a
// subscription that silently kept the free grant and an upgrade that discarded a captured
// payment both shipped green (review/diff-260908 F13).
type ledgerHarness struct {
	handle  *db.DB
	ledger  *usage.Service
	service *billing.Service
}

func newLedgerHarness(t *testing.T, name string, createdAt time.Time, anchor time.Time) *ledgerHarness {
	t.Helper()
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO users (id, password_hash, plan, email, email_verified_at, created_at) VALUES (?, 'hash', 'free', ?, ?, ?)",
		"alice", "alice@example.com", createdAt.Format(time.RFC3339Nano), createdAt.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}

	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), nil, 0)
	ledger.SetAnchors(fixedAnchor{at: anchor})

	store := billingstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(conn *sql.Conn) billing.Credits {
		return usage.NewService(usagestore.NewTx(conn), nil, 0)
	})
	store.SetPlansForTx(func(conn *sql.Conn) billing.Plans {
		return auth.NewService(authstore.NewTx(conn), time.Hour)
	})
	if err := store.UpsertPaymentMethod(ctx, billing.PaymentMethod{
		UserID: "alice", Provider: "toss", BillingKey: "billing-key",
		CustomerKey: billing.CustomerKey("alice"), CardLabel: "11 1234", RegisteredAt: createdAt,
	}); err != nil {
		t.Fatal(err)
	}
	return &ledgerHarness{
		handle: handle, ledger: ledger,
		service: billing.NewService(store, &registrationProvider{}, registrationRates{}, ledger, nil, nil, nil),
	}
}

// monthlyLots reports the account's non-expired monthly lots as of now, with what they hold.
func (h *ledgerHarness) monthlyLots(t *testing.T, now time.Time) (count, granted, remaining int) {
	t.Helper()
	err := h.handle.Reader.QueryRowContext(context.Background(),
		`SELECT count(*), coalesce(sum(granted), 0), coalesce(sum(remaining), 0) FROM credit_lots
		 WHERE user_id = ? AND kind = 'monthly' AND expires_at IS NOT NULL AND expires_at > ?`,
		"alice", now.UTC().Format(time.RFC3339Nano)).Scan(&count, &granted, &remaining)
	if err != nil {
		t.Fatal(err)
	}
	return count, granted, remaining
}

type fixedAnchor struct{ at time.Time }

func (a fixedAnchor) AnchorFor(context.Context, string) (time.Time, error) { return a.at, nil }

// The collision F1 exists for: a window's id is its start date, so signing up and
// subscribing on one Seoul date derives one id for the free window and the paid one.
func TestSubscribingTheSameDayAsSignupHandsOverTheWholeTierGrant(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "same-day.db", now, now)

	if err := h.ledger.EnsureMonthlyLot(ctx, "alice", plan.Free); err != nil {
		t.Fatal(err)
	}
	if count, granted, _ := h.monthlyLots(t, now); count != 1 || granted != plan.MonthlyCredits(plan.Free) {
		t.Fatalf("free window: lots=%d granted=%d", count, granted)
	}

	if _, err := h.service.Subscribe(ctx, "alice", plan.Pro, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}

	count, granted, remaining := h.monthlyLots(t, time.Now().UTC())
	want := plan.MonthlyCredits(plan.Pro)
	if count != 1 || granted != want || remaining != want {
		t.Fatalf("after subscribing: lots=%d granted=%d remaining=%d, want 1 lot of %d", count, granted, remaining, want)
	}
}

// The other half of QUOTA-42: when the two windows do NOT share an id, the free one is
// closed rather than left beside the paid one, and its remainder does not carry over.
func TestSubscribingOffTheFreeAnchorClosesTheFreeWindow(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	created := now.AddDate(0, 0, -5)
	h := newLedgerHarness(t, "off-anchor.db", created, created)

	if err := h.ledger.EnsureMonthlyLot(ctx, "alice", plan.Free); err != nil {
		t.Fatal(err)
	}
	if count, _, _ := h.monthlyLots(t, now); count != 1 {
		t.Fatalf("free window: lots=%d, want 1", count)
	}

	if _, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}

	// Read as of after the subscribe: the free window is closed AT that instant, so a
	// timestamp taken before it would still see the lot as open.
	count, granted, remaining := h.monthlyLots(t, time.Now().UTC())
	want := plan.MonthlyCredits(plan.Basic)
	if count != 1 || granted != want || remaining != want {
		t.Fatalf("after subscribing: lots=%d granted=%d remaining=%d, want 1 lot of %d", count, granted, remaining, want)
	}
	var closed int
	if err := h.handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM credit_lots WHERE user_id = ? AND kind = 'monthly'", "alice").Scan(&closed); err != nil {
		t.Fatal(err)
	}
	if closed != 2 {
		t.Fatalf("monthly rows = %d, want the closed free window kept beside the new one", closed)
	}
}

// A provider retry must not mint a second window or re-grant the one already open.
func TestStartingTheSameWindowTwiceLeavesOneLot(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "idempotent.db", now, now)

	start, end := plan.AnchorWindow(now, now)
	if err := h.ledger.StartMonthlyWindow(ctx, "alice", plan.Max, start, end); err != nil {
		t.Fatal(err)
	}
	if err := h.ledger.StartMonthlyWindow(ctx, "alice", plan.Max, start, end); err != nil {
		t.Fatalf("second start: %v", err)
	}

	count, granted, remaining := h.monthlyLots(t, now)
	want := plan.MonthlyCredits(plan.Max)
	if count != 1 || granted != want || remaining != want {
		t.Fatalf("lots=%d granted=%d remaining=%d, want 1 lot of %d", count, granted, remaining, want)
	}
	var expiries int
	if err := h.handle.Reader.QueryRowContext(ctx,
		"SELECT count(DISTINCT expires_at) FROM credit_lots WHERE user_id = ? AND kind = 'monthly'", "alice").Scan(&expiries); err != nil {
		t.Fatal(err)
	}
	if expiries != 1 {
		t.Fatalf("distinct expiries = %d, want the window's own end unchanged", expiries)
	}
}

// F2: an upgrade lands in the gap between one window expiring and the next request opening
// one. The card is charged outside the transaction (ARCH-10), so the credit step must not be
// able to throw the payment away.
func TestUpgradeCommitsWhenNoMonthlyLotIsOpen(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "upgrade-gap.db", now, now)

	if _, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	// The gap itself: the window has ended and nothing has re-opened one yet.
	if _, err := h.handle.Writer.ExecContext(ctx,
		"UPDATE credit_lots SET expires_at = ? WHERE user_id = ? AND kind = 'monthly'",
		now.AddDate(0, 0, -1).Format(time.RFC3339Nano), "alice"); err != nil {
		t.Fatal(err)
	}
	if count, _, _ := h.monthlyLots(t, now); count != 0 {
		t.Fatalf("open monthly lots = %d, want the gap", count)
	}

	updated, applied, err := h.service.ChangeSubscription(ctx, "alice", plan.Pro, billing.TermMonthly)
	if err != nil || !applied || updated.Tier != plan.Pro {
		t.Fatalf("upgrade = %+v applied=%v err=%v", updated, applied, err)
	}

	var tier, storedTier string
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT tier FROM subscriptions WHERE user_id = ?", "alice").Scan(&tier); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT plan FROM users WHERE id = ?", "alice").Scan(&storedTier); err != nil {
		t.Fatal(err)
	}
	var charges, tierChanges int
	if err := h.handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM billing_events WHERE user_id = ? AND kind = 'charge'", "alice").Scan(&charges); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM billing_events WHERE user_id = ? AND kind = 'tier_change'", "alice").Scan(&tierChanges); err != nil {
		t.Fatal(err)
	}
	if tier != "pro" || storedTier != "pro" || charges != 2 || tierChanges != 2 {
		t.Fatalf("tier=%s users.plan=%s charges=%d tier_changes=%d", tier, storedTier, charges, tierChanges)
	}
}
