package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type clipExportAdapter struct{ clip.ExportWindows }

func (a clipExportAdapter) OpenExportWindow(ctx context.Context, window usage.ExportWindow) error {
	return a.ExportWindows.OpenExportWindow(ctx, clip.ExportWindow{UserID: window.UserID,
		CoverageID: window.CoverageID, Start: window.Start, End: window.End, Allowance: window.Allowance}, "lazy")
}

func TestLazyReadOpensCreditAndExportWindowsInOneTransaction(t *testing.T) {
	_, handle := newServiceWithDB(t)
	removeLegacyFunding(t, handle, "alice")
	ctx := context.Background()
	anchor := time.Now().UTC().AddDate(0, -2, 0)
	end := plan.MonthBoundary(anchor, 12)
	store := usagestore.NewWithExports(handle.Writer, handle.Reader, func(tx *sql.Tx) usage.ExportWindowLedger {
		return clipExportAdapter{clipstore.NewTx(tx)}
	})
	service := usage.NewService(store, pricedModels{}, maxCompletion,
		benefitCoverage{id: "paid:alice:lazy", anchor: anchor, end: end, tier: plan.Basic})
	if _, err := handle.Writer.ExecContext(ctx, `CREATE TRIGGER reject_lazy_export BEFORE INSERT ON server_export_windows
BEGIN SELECT RAISE(ABORT, 'export unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BalanceFor(ctx, "alice", plan.Basic); err == nil {
		t.Fatal("expected atomic export failure")
	}
	var lots int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM credit_lots WHERE user_id='alice'").Scan(&lots); err != nil {
		t.Fatal(err)
	}
	if lots != 0 {
		t.Fatalf("failed export left %d credit grants", lots)
	}
	if _, err := handle.Writer.ExecContext(ctx, "DROP TRIGGER reject_lazy_export"); err != nil {
		t.Fatal(err)
	}
	balance, err := service.BalanceFor(ctx, "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Credits != 555 {
		t.Fatalf("balance=%d, want daily 45 plus bonus 510", balance.Credits)
	}
	var allowance int
	if err := handle.Reader.QueryRow("SELECT allowance FROM server_export_windows WHERE user_id='alice'").Scan(&allowance); err != nil {
		t.Fatal(err)
	}
	if allowance != 6 {
		t.Fatalf("export allowance=%d", allowance)
	}
	if _, err := handle.Writer.ExecContext(ctx, "UPDATE credit_lots SET remaining=remaining-7 WHERE user_id='alice' AND kind='monthly'"); err != nil {
		t.Fatal(err)
	}
	balance, err = service.BalanceFor(ctx, "alice", plan.Basic)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Credits != 548 {
		t.Fatalf("repeat read replenished spent credits: %d", balance.Credits)
	}
}

type benefitCoverage struct {
	id          string
	anchor, end time.Time
	tier        plan.Plan
}

func (a benefitCoverage) AnchorFor(context.Context, string) (time.Time, error) { return a.anchor, nil }
func (a benefitCoverage) CoverageFor(_ context.Context, _ string, at time.Time) (usage.Coverage, bool, error) {
	if at.Before(a.anchor) || !at.Before(a.end) {
		return usage.Coverage{}, false, nil
	}
	return usage.Coverage{ID: a.id, Anchor: a.anchor, End: a.end, Tier: a.tier, DailyTier: a.tier}, true, nil
}

func TestConcurrentCurrentWindowGrantsDoNotReplayMissedBenefits(t *testing.T) {
	_, handle := newServiceWithDB(t)
	removeLegacyFunding(t, handle, "alice")
	ctx := context.Background()
	anchor := time.Now().UTC().AddDate(0, -3, 0)
	coverage := usage.Coverage{ID: "paid:alice:race", Anchor: anchor, End: plan.MonthBoundary(anchor, 12), Tier: plan.Basic, DailyTier: plan.Basic}
	service := usage.NewService(usagestore.New(handle.Writer, handle.Reader), pricedModels{}, maxCompletion,
		benefitCoverage{id: coverage.ID, anchor: anchor, end: coverage.End, tier: plan.Basic})
	const workers = 16
	var group sync.WaitGroup
	errors := make(chan error, workers)
	for i := range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			switch i % 3 {
			case 0:
				errors <- service.OpenCoverage(ctx, "alice", coverage, time.Now(), "charge-1")
			case 1:
				_, err := service.BalanceFor(ctx, "alice", plan.Basic)
				errors <- err
			default:
				errors <- service.Hold(ctx, usage.Start{UserID: "alice", Plan: plan.Basic,
					Kind: "generate", JobID: fmt.Sprintf("racing-admission-%d", i),
					Calls: []usage.PlannedCall{{Ref: pricedRef, Count: 1}}})
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var daily, monthly int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM credit_lots WHERE user_id='alice' AND kind='daily'").Scan(&daily); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow("SELECT count(*) FROM credit_lots WHERE user_id='alice' AND kind='monthly'").Scan(&monthly); err != nil {
		t.Fatal(err)
	}
	if daily != 1 || monthly != 1 {
		t.Fatalf("racing grants: daily=%d monthly=%d", daily, monthly)
	}

	// Non-subscription lots survive a current-window materialization unchanged.
	voucherID, err := service.OpenVoucherLot(ctx, "alice", 9, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	purchasedID, err := service.OpenPurchasedLot(ctx, "alice", 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.OpenCoverage(ctx, "alice", coverage, time.Now(), "charge-1"); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]int{voucherID: 9, purchasedID: 12} {
		_, remaining := lotRemaining(t, handle, id)
		if remaining != want {
			t.Errorf("lot %s remaining=%d want %d", id, remaining, want)
		}
	}
}

func TestRenewalExtendsExistingWindowsWithoutRefillingThem(t *testing.T) {
	_, handle := newServiceWithDB(t)
	removeLegacyFunding(t, handle, "alice")
	ctx := context.Background()
	now := time.Now().UTC()
	anchor := now.Add(-10 * time.Hour)
	oldEnd := now.Add(time.Hour)
	store := usagestore.NewWithExports(handle.Writer, handle.Reader, func(tx *sql.Tx) usage.ExportWindowLedger {
		return clipExportAdapter{clipstore.NewTx(tx)}
	})
	service := usage.NewService(store, pricedModels{}, maxCompletion,
		benefitCoverage{id: "paid:alice:renew", anchor: anchor, end: oldEnd, tier: plan.Basic})
	coverage := usage.Coverage{ID: "paid:alice:renew", Anchor: anchor, End: oldEnd, Tier: plan.Basic, DailyTier: plan.Basic}
	if err := service.OpenCoverage(ctx, "alice", coverage, now, "first-charge"); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, "UPDATE credit_lots SET remaining=remaining-7 WHERE user_id='alice'"); err != nil {
		t.Fatal(err)
	}
	coverage.End = plan.MonthBoundary(anchor, 2)
	if err := service.OpenCoverage(ctx, "alice", coverage, now, "renewal-charge"); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"daily", "monthly"} {
		var granted, remaining int
		var expires string
		if err := handle.Reader.QueryRowContext(ctx, "SELECT granted,remaining,expires_at FROM credit_lots WHERE user_id='alice' AND kind=?", kind).Scan(&granted, &remaining, &expires); err != nil {
			t.Fatal(err)
		}
		if remaining != granted-7 || !timeStringAfter(t, expires, oldEnd) {
			t.Fatalf("%s renewal: granted=%d remaining=%d expires=%s", kind, granted, remaining, expires)
		}
	}
	var exportEnd string
	if err := handle.Reader.QueryRowContext(ctx, "SELECT window_end FROM server_export_windows WHERE user_id='alice'").Scan(&exportEnd); err != nil {
		t.Fatal(err)
	}
	if !timeStringAfter(t, exportEnd, oldEnd) {
		t.Fatalf("export window still ends at old term: %s", exportEnd)
	}
}

func timeStringAfter(t *testing.T, stamp string, at time.Time) bool {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.After(at)
}

func TestSettlementAfterDailyResetCannotDrawTheNewGrant(t *testing.T) {
	_, handle := newServiceWithDB(t)
	removeLegacyFunding(t, handle, "alice")
	ctx := context.Background()
	anchor := time.Now().UTC().Add(-24*time.Hour + 2*time.Second)
	end := plan.CoverageEnd(anchor, true)
	coverage := usage.Coverage{ID: "paid:alice:reset", Anchor: anchor, End: end, Tier: plan.Basic, DailyTier: plan.Basic}
	service := usage.NewService(usagestore.New(handle.Writer, handle.Reader), pricedModels{}, maxCompletion,
		benefitCoverage{id: coverage.ID, anchor: anchor, end: end, tier: plan.Basic})
	if err := service.OpenCoverage(ctx, "alice", coverage, time.Now(), "charge-1"); err != nil {
		t.Fatal(err)
	}
	start := usage.Start{UserID: "alice", Plan: plan.Basic, Kind: "generate", JobID: "cross-reset",
		Calls: []usage.PlannedCall{{Ref: pricedRef, Count: 1}}}
	if err := service.Hold(ctx, start); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO usage_events
        (user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
        VALUES ('alice','generate','cross-reset','write','test/model',1,1,100000,'reported',?)`,
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	_, firstEnd := plan.DailyWindow(anchor, time.Now())
	if wait := time.Until(firstEnd); wait > 0 {
		time.Sleep(wait + 20*time.Millisecond)
	}
	if _, err := service.BalanceFor(ctx, "alice", plan.Basic); err != nil {
		t.Fatal(err)
	}
	if err := service.Settle(ctx, "cross-reset", usage.OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	rows, err := handle.Reader.QueryContext(ctx, "SELECT remaining FROM credit_lots WHERE user_id='alice' AND kind='daily' ORDER BY window_start")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var remaining []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		remaining = append(remaining, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(remaining) != "[13 45]" {
		t.Fatalf("daily lots after settlement=%v, want old-period overrun and untouched new grant", remaining)
	}
}
