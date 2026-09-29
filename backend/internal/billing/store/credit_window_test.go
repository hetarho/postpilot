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
	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/mail"
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
	store   *billingstore.Store
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

	ledger := usage.NewService(usagestore.New(handle.Writer, handle.Reader), nil, 0, fixedAnchor{at: anchor})

	store := billingstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return testCredits{Service: usage.NewService(usagestore.NewTx(tx), nil, 0, fixedAnchor{at: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}), exports: clipstore.NewTx(tx)}
	})
	store.SetPlansForTx(func(tx *sql.Tx) billing.Plans {
		return auth.NewService(authstore.NewTx(tx), time.Hour, auth.Deps{Mailer: mail.NewLog()})
	})
	if err := store.UpsertPaymentMethod(ctx, billing.PaymentMethod{
		UserID: "alice", Provider: "toss", BillingKey: "billing-key",
		CustomerKey: billing.CustomerKey("alice"), CardLabel: "11 1234", RegisteredAt: createdAt,
	}); err != nil {
		t.Fatal(err)
	}
	return &ledgerHarness{
		handle: handle, ledger: ledger, store: store,
		service: billing.NewService(store, &registrationProvider{}, registrationRates{}, testCredits{Service: ledger, exports: clipstore.New(handle.Writer, handle.Reader)}, nil, nil, nil),
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
func (a fixedAnchor) CoverageFor(context.Context, string, time.Time) (usage.Coverage, bool, error) {
	return usage.Coverage{ID: "test-coverage", Anchor: a.at, Tier: plan.Basic, DailyTier: plan.Basic}, true, nil
}

type testCredits struct {
	*usage.Service
	exports clip.ExportWindows
}

func (c testCredits) OpenCoverage(ctx context.Context, userID string, coverage billing.Coverage, at time.Time, correlation string) error {
	if err := c.Service.OpenCoverage(ctx, userID, usage.Coverage{ID: coverage.ID, Anchor: coverage.Anchor,
		End: coverage.End, Tier: coverage.Tier, DailyTier: coverage.DailyTier}, at, correlation); err != nil {
		return err
	}
	if c.exports == nil {
		return nil
	}
	start, end := plan.BenefitWindow(coverage.Anchor, at)
	if !coverage.End.IsZero() && coverage.End.Before(end) {
		end = coverage.End
	}
	offer, _ := plan.CommercialOffer(coverage.Tier)
	return c.exports.OpenExportWindow(ctx, clip.ExportWindow{UserID: userID, CoverageID: coverage.ID,
		Start: start, End: end, Allowance: offer.ServerExports}, correlation)
}
func (c testCredits) AddUpgradeBonus(ctx context.Context, userID string, coverage billing.Coverage, at time.Time, credits, exportDelta int, correlation string) error {
	if err := c.Service.AddUpgradeBonus(ctx, userID, usage.Coverage{ID: coverage.ID, Anchor: coverage.Anchor,
		End: coverage.End, Tier: coverage.Tier, DailyTier: coverage.DailyTier}, at, credits, correlation); err != nil {
		return err
	}
	if c.exports == nil {
		return nil
	}
	start, end := plan.BenefitWindow(coverage.Anchor, at)
	if !coverage.End.IsZero() && coverage.End.Before(end) {
		end = coverage.End
	}
	offer, _ := plan.CommercialOffer(coverage.Tier)
	window := clip.ExportWindow{UserID: userID, CoverageID: coverage.ID, Start: start, End: end, Allowance: offer.ServerExports}
	return c.exports.RaiseExportWindow(ctx, window, exportDelta, correlation)
}

// Free signup creates no lot. A subscription on that same date opens only the paid window.
func TestSubscribingTheSameDayAsSignupHandsOverTheWholeTierGrant(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "same-day.db", now, now)

	if err := h.ledger.EnsureMonthlyLot(ctx, "alice", plan.Free); err != nil {
		t.Fatal(err)
	}
	if count, granted, _ := h.monthlyLots(t, now); count != 0 || granted != 0 {
		t.Fatalf("free window: lots=%d granted=%d", count, granted)
	}

	if _, err := h.service.Subscribe(ctx, "alice", plan.Pro, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}

	count, granted, remaining := h.monthlyLots(t, time.Now().UTC())
	want := 1070
	if count != 1 || granted != want || remaining != want {
		t.Fatalf("after subscribing: lots=%d granted=%d remaining=%d, want 1 lot of %d", count, granted, remaining, want)
	}
}

// A later subscription also starts with no free-credit lot carried forward.
func TestSubscribingOffTheFreeAnchorClosesTheFreeWindow(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	created := now.AddDate(0, 0, -5)
	h := newLedgerHarness(t, "off-anchor.db", created, created)

	if err := h.ledger.EnsureMonthlyLot(ctx, "alice", plan.Free); err != nil {
		t.Fatal(err)
	}
	if count, _, _ := h.monthlyLots(t, now); count != 0 {
		t.Fatalf("free window: lots=%d, want 0", count)
	}

	if _, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}

	// Read as of after the subscribe: the free window is closed AT that instant, so a
	// timestamp taken before it would still see the lot as open.
	count, granted, remaining := h.monthlyLots(t, time.Now().UTC())
	want := 510
	if count != 1 || granted != want || remaining != want {
		t.Fatalf("after subscribing: lots=%d granted=%d remaining=%d, want 1 lot of %d", count, granted, remaining, want)
	}
	var closed int
	if err := h.handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM credit_lots WHERE user_id = ? AND kind = 'monthly'", "alice").Scan(&closed); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("monthly rows = %d, want only the new paid window", closed)
	}
}

// A provider retry must not mint a second window or re-grant the one already open.
func TestOpeningTheSameCoverageTwiceDoesNotReplenishSpentBenefits(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "idempotent.db", now, now)

	coverage := usage.Coverage{ID: "paid:alice:test", Anchor: now, End: plan.CoverageEnd(now, false), Tier: plan.Max, DailyTier: plan.Max}
	if err := h.ledger.OpenCoverage(ctx, "alice", coverage, now, "charge-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.handle.Writer.ExecContext(ctx, "UPDATE credit_lots SET remaining=remaining-7 WHERE user_id=? AND kind='monthly'", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := h.ledger.OpenCoverage(ctx, "alice", coverage, now, "charge-1"); err != nil {
		t.Fatalf("second start: %v", err)
	}

	count, granted, remaining := h.monthlyLots(t, now)
	want := 3170
	if count != 1 || granted != want || remaining != want-7 {
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

func TestSupportAssignmentAtomicallyOpensAndUpgradesBenefits(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "support-benefits.db", now, now)
	if err := h.service.AssignSupportTier(ctx, "alice", plan.Light); err != nil {
		t.Fatal(err)
	}
	var tier string
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT plan FROM users WHERE id='alice'").Scan(&tier); err != nil {
		t.Fatal(err)
	}
	if tier != "light" {
		t.Fatalf("tier = %q", tier)
	}
	var daily, monthly, exports, subscriptions, charges int
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT COALESCE(sum(granted),0) FROM credit_lots WHERE user_id='alice' AND kind='daily'").Scan(&daily); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT COALESCE(sum(granted),0) FROM credit_lots WHERE user_id='alice' AND kind='monthly'").Scan(&monthly); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT allowance FROM server_export_windows WHERE user_id='alice'").Scan(&exports); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM subscriptions WHERE user_id='alice'").Scan(&subscriptions); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM billing_events WHERE user_id='alice' AND kind='charge'").Scan(&charges); err != nil {
		t.Fatal(err)
	}
	if daily != 15 || monthly != 290 || exports != 2 || subscriptions != 0 || charges != 0 {
		t.Fatalf("support benefits daily=%d monthly=%d exports=%d subscriptions=%d charges=%d", daily, monthly, exports, subscriptions, charges)
	}
	if err := h.service.AssignSupportTier(ctx, "alice", plan.Light); err != nil {
		t.Fatal(err)
	}
	if _, err := h.handle.Writer.ExecContext(ctx, "UPDATE credit_lots SET remaining=remaining-7 WHERE user_id='alice' AND kind='daily'"); err != nil {
		t.Fatal(err)
	}
	if err := h.service.AssignSupportTier(ctx, "alice", plan.Basic); err != nil {
		t.Fatal(err)
	}
	var remaining, dailyRows int
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*),sum(remaining) FROM credit_lots WHERE user_id='alice' AND kind='daily'").Scan(&dailyRows, &remaining); err != nil {
		t.Fatal(err)
	}
	if dailyRows != 1 || remaining != 8 {
		t.Fatalf("today's daily grant changed: rows=%d remaining=%d", dailyRows, remaining)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT allowance FROM server_export_windows WHERE user_id='alice'").Scan(&exports); err != nil {
		t.Fatal(err)
	}
	if exports < 2 || exports > 6 {
		t.Fatalf("prorated exports=%d", exports)
	}
}

func TestFailedSupportExportMutationRollsBackTierAndCredits(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "support-rollback.db", now, now)
	if _, err := h.handle.Writer.ExecContext(ctx, `CREATE TRIGGER reject_export BEFORE INSERT ON server_export_windows
BEGIN SELECT RAISE(ABORT, 'blocked export'); END`); err != nil {
		t.Fatal(err)
	}
	if err := h.service.AssignSupportTier(ctx, "alice", plan.Light); err == nil {
		t.Fatal("expected export insert failure")
	}
	var tier string
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT plan FROM users WHERE id='alice'").Scan(&tier); err != nil {
		t.Fatal(err)
	}
	var lots, support int
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_lots WHERE user_id='alice'").Scan(&lots); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM support_coverages WHERE user_id='alice'").Scan(&support); err != nil {
		t.Fatal(err)
	}
	if tier != "free" || lots != 0 || support != 0 {
		t.Fatalf("partial assignment tier=%s lots=%d support=%d", tier, lots, support)
	}
}

func TestCoverageExpiresBeforeRenewalWorkerAndUpgradeKeepsTodaysDailyTier(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	h := newLedgerHarness(t, "coverage-tier.db", now, now)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := h.service.CoverageAt(ctx, "alice", sub.TermEnd.Add(time.Nanosecond)); err != nil || found {
		t.Fatalf("expired coverage found=%v err=%v", found, err)
	}
	if _, applied, err := h.service.ChangeSubscription(ctx, "alice", plan.Pro, billing.TermMonthly); err != nil || !applied {
		t.Fatalf("upgrade applied=%v err=%v", applied, err)
	}
	current, found, err := h.service.CoverageAt(ctx, "alice", time.Now())
	if err != nil || !found {
		t.Fatalf("current coverage found=%v err=%v", found, err)
	}
	if current.Tier != plan.Pro || current.DailyTier != plan.Basic || current.ID != sub.CoverageID || !current.Anchor.Equal(sub.AnchorAt) {
		t.Fatalf("upgrade moved today's daily rights or anchor: %+v", current)
	}
}
