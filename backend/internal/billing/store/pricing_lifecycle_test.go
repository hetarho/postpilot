package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
	"github.com/postpilot/backend/internal/voucher"
	voucherstore "github.com/postpilot/backend/internal/voucher/store"
)

type lifecycleModels map[llm.ModelRef]llm.ModelInfo

func (m lifecycleModels) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	info, found := m[ref]
	return info, found
}

type lifecycleRateSource struct{}

func (lifecycleRateSource) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 14_000_000, true, nil
}

type lifecycleCoverage struct {
	start time.Time
	sub   *billing.Subscription
}

func (a *lifecycleCoverage) AnchorFor(context.Context, string) (time.Time, error) {
	if a.sub != nil {
		return a.sub.AnchorAt, nil
	}
	return a.start, nil
}
func (a *lifecycleCoverage) CoverageFor(_ context.Context, _ string, at time.Time) (usage.Coverage, bool, error) {
	if a.sub == nil || a.sub.Status != "active" || at.Before(a.sub.AnchorAt) || !at.Before(a.sub.TermEnd) {
		return usage.Coverage{}, false, nil
	}
	return usage.Coverage{ID: a.sub.CoverageID, Anchor: a.sub.AnchorAt, End: a.sub.TermEnd,
		Tier: a.sub.Tier, DailyTier: a.sub.Tier}, true, nil
}

type lifecycleVoucherCredits struct{ *usage.Service }

func (c lifecycleVoucherCredits) VoucherLotStandings(ctx context.Context, ids []string, at time.Time) (map[string]voucher.LotStanding, error) {
	rows, err := c.Service.VoucherLotStandings(ctx, ids, at)
	if err != nil {
		return nil, err
	}
	out := make(map[string]voucher.LotStanding, len(rows))
	for id, row := range rows {
		out[id] = voucher.LotStanding{Remaining: row.Remaining, ExpiresAt: row.ExpiresAt}
	}
	return out, nil
}

type lifecyclePaidCoverage struct{ tx *sql.Tx }

func (p lifecyclePaidCoverage) ActivePaidAt(ctx context.Context, user string, at time.Time) (bool, error) {
	var count int
	err := p.tx.QueryRowContext(ctx, `SELECT count(*) FROM subscriptions WHERE user_id=? AND status='active' AND anchor_at<=? AND term_end>?`,
		user, at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano)).Scan(&count)
	return count == 1, err
}

func lifecycleCount(t *testing.T, h *ledgerHarness, query string, args ...any) int {
	t.Helper()
	var count int
	if err := h.handle.Reader.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// One clock, real SQLite stores, and an authoritative in-memory payment provider
// exercise the transitions between billing, credits, vouchers and server exports.
func TestPricingLifecycleFromFreeThroughRefund(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := refundHarness(t, start)
	anchor := &lifecycleCoverage{start: start}
	freeRef := llm.ModelRef{ProviderID: "openrouter", ModelID: "fixture-free"}
	valueRef := llm.ModelRef{ProviderID: "openrouter", ModelID: "fixture-value"}
	models := lifecycleModels{
		freeRef:  {Ref: freeRef, Stages: []string{"write"}, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"},
		valueRef: {Ref: valueRef, Stages: []string{"write"}, Levels: map[string]string{"write": "value"}, InputUSDPerMillion: "0.1", OutputUSDPerMillion: "0.3"},
	}
	creditStore := usagestore.New(h.handle.Writer, h.handle.Reader)
	rates := usage.NewRateSelector(lifecycleRateSource{}, creditStore)
	credits := usage.NewService(creditStore, models, 4_000, anchor, rates).WithModelGrades().WithClock(func() time.Time { return *clock })
	if lifecycleCount(t, h, "SELECT count(*) FROM credit_lots") != 0 {
		t.Fatal("signup granted credits")
	}
	if err := credits.Hold(ctx, usage.Start{UserID: "alice", Plan: plan.Free, Kind: "write", JobID: "free-work",
		Calls: []usage.PlannedCall{{Ref: freeRef, Stage: "write", Count: 1}}}); err != nil {
		t.Fatal(err)
	}
	if lifecycleCount(t, h, "SELECT hold_credits FROM usage_admissions WHERE job_id='free-work'") != 0 {
		t.Fatal("free model consumed credits")
	}
	monthly, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly)
	if err != nil || !monthly.TermEnd.Equal(start.AddDate(0, 1, 0)) {
		t.Fatalf("31-day monthly start=%+v err=%v", monthly, err)
	}
	anchor.sub = &monthly
	if lifecycleCount(t, h, "SELECT count(*) FROM credit_lots WHERE user_id='alice' AND kind='daily'") != 1 {
		t.Fatal("paid start did not open first daily grant")
	}
	paid := usage.Start{UserID: "alice", Plan: plan.Light, Kind: "write", JobID: "spans-reset",
		Calls: []usage.PlannedCall{{Ref: valueRef, Stage: "write", Count: 1, CompletionTokens: 4_000}}}
	if err := credits.Hold(ctx, paid); err != nil {
		t.Fatal(err)
	}
	*clock = start.Add(25 * time.Hour)
	if _, err := credits.BalanceFor(ctx, "alice", plan.Light); err != nil {
		t.Fatal(err)
	}
	if lifecycleCount(t, h, "SELECT count(*) FROM credit_lots WHERE user_id='alice' AND kind='daily'") != 2 {
		t.Fatal("daily inactivity/reset did not open next window")
	}
	if _, err := h.handle.Writer.ExecContext(ctx, `INSERT INTO usage_events(user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
		VALUES ('alice','write','spans-reset','write','fixture-value',100,100,1000,'reported',?)`, clock.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := credits.SettleCause(ctx, "spans-reset", usage.OutcomeSucceeded, "succeeded"); err != nil {
		t.Fatal(err)
	}
	if err := credits.SettleCause(ctx, "spans-reset", usage.OutcomeSucceeded, "succeeded"); err != nil {
		t.Fatal("settlement replay", err)
	}
	if lifecycleCount(t, h, "SELECT count(*) FROM usage_admissions WHERE job_id='spans-reset' AND settled_at IS NOT NULL") != 1 {
		t.Fatal("in-flight job settled twice or lost")
	}
	*clock = monthly.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	monthly, _, err = h.store.Subscription(ctx, "alice")
	if err != nil || !monthly.TermEnd.Equal(start.AddDate(0, 2, 0)) || len(provider.fixedPayments.charges) != 2 {
		t.Fatalf("31-day renewal=%+v charges=%v err=%v", monthly, provider.fixedPayments.charges, err)
	}
	anchor.sub = &monthly
	quote, err := h.service.QuoteChange(ctx, "alice", plan.Light, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	if _, applied, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Light, billing.TermAnnual, quote.ID); err != nil || applied {
		t.Fatalf("annual schedule applied early=%t err=%v", applied, err)
	}
	*clock = monthly.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	annual, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || annual.Term != billing.TermAnnual || len(provider.fixedPayments.charges) != 3 {
		t.Fatalf("annual transition=%+v err=%v", annual, err)
	}
	anchor.sub = &annual
	*clock = annual.TermStart.AddDate(0, 1, 0)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	if _, err := credits.BalanceFor(ctx, "alice", plan.Light); err != nil {
		t.Fatal(err)
	}
	if len(provider.fixedPayments.charges) != 3 {
		t.Fatal("annual monthly benefit charged again")
	}
	*clock = clock.Add(10 * 24 * time.Hour)
	quote, err = h.service.QuoteChange(ctx, "alice", plan.Basic, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	upgraded, applied, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Basic, billing.TermAnnual, quote.ID)
	if err != nil || !applied || upgraded.Tier != plan.Basic || !upgraded.AnchorAt.Equal(start) || !upgraded.TermEnd.Equal(annual.TermEnd) {
		t.Fatalf("mid-cycle upgrade=%+v applied=%t err=%v", upgraded, applied, err)
	}
	anchor.sub = &upgraded
	pack, err := h.service.PurchasePack(ctx, "alice", "pack-1000")
	if err != nil || pack.Credits != 1000 {
		t.Fatalf("pack=%+v err=%v", pack, err)
	}
	voucherStore := voucherstore.New(h.handle.Writer, h.handle.Reader)
	voucherStore.SetCreditsForTx(func(tx *sql.Tx) voucher.Credits {
		return lifecycleVoucherCredits{usage.NewService(usagestore.NewTx(tx), models, 4_000, anchor, rates).WithClock(func() time.Time { return *clock })}
	})
	voucherStore.SetPaidCoverageForTx(func(tx *sql.Tx) voucher.PaidCoverage { return lifecyclePaidCoverage{tx} })
	vouchers := voucher.NewService(voucherStore, lifecycleVoucherCredits{credits}).WithClock(func() time.Time { return *clock })
	issued, err := vouchers.Issue(ctx, "operator", voucher.Issue{Credits: 120, ValidityDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vouchers.Redeem(ctx, "alice", issued.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := vouchers.Redeem(ctx, "alice", issued.Token); !errors.Is(err, voucher.ErrRedeemed) {
		t.Fatalf("voucher replay=%v", err)
	}
	if lifecycleCount(t, h, "SELECT count(*) FROM credit_lots WHERE kind='voucher' AND user_id='alice'") != 1 {
		t.Fatal("voucher lot missing")
	}
	// A reported service failure charges confirmed cost and grants half as separate compensation.
	failed := paid
	failed.Plan, failed.JobID = plan.Basic, "service-fault"
	if err := credits.Hold(ctx, failed); err != nil {
		t.Fatal(err)
	}
	if _, err := h.handle.Writer.ExecContext(ctx, `INSERT INTO usage_events(user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
		VALUES ('alice','write','service-fault','write','fixture-value',100,100,1000,'reported',?)`, clock.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := credits.SettleCause(ctx, failed.JobID, usage.OutcomeFailed, "service"); err != nil {
		t.Fatal(err)
	}
	if lifecycleCount(t, h, "SELECT count(*) FROM credit_lots WHERE kind='compensation' AND user_id='alice'") != 1 {
		t.Fatal("service fault did not grant compensation")
	}
	exports := clipstore.New(h.handle.Writer, h.handle.Reader)
	if err := exports.ReserveExport(ctx, "alice", "project", 1, "export-success", *clock); err != nil {
		t.Fatal(err)
	}
	if err := exports.BindExport(ctx, "export-success", "render-success"); err != nil {
		t.Fatal(err)
	}
	result := clip.AttemptResult{JobID: "render-success", UserID: "alice", ProjectID: "project", ExpectedRevision: 1,
		Result: clip.Result{Kind: clip.RenderServer, Key: "result.mp4"}}
	if err := exports.CommitExport(ctx, result); err != nil {
		t.Fatal(err)
	}
	if err := exports.CommitExport(ctx, result); err != nil {
		t.Fatal("export replay", err)
	}
	if err := exports.ReserveExport(ctx, "alice", "project", 2, "export-failed", *clock); err != nil {
		t.Fatal(err)
	}
	if err := exports.ReleaseExport(ctx, "export-failed"); err != nil {
		t.Fatal(err)
	}
	window, ok, err := exports.CurrentExportWindow(ctx, "alice", *clock)
	if err != nil || !ok || window.Used != 1 || window.Reserved != 0 {
		t.Fatalf("export count=%+v ok=%t err=%v", window, ok, err)
	}
	if _, err := h.service.CancelSubscription(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	*clock = annual.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	lapsed, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || lapsed.Status != "lapsed" || lifecycleCount(t, h, "SELECT count(*) FROM users WHERE id='alice' AND plan='free'") != 1 {
		t.Fatalf("lapse=%+v err=%v", lapsed, err)
	}
	*clock = clock.Add(time.Minute)
	rejoined, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly)
	if err != nil || rejoined.CoverageID == annual.CoverageID || !rejoined.AnchorAt.Equal(*clock) {
		t.Fatalf("rejoin=%+v err=%v", rejoined, err)
	}
	order := chargeOrder(t, h, "subscribe")
	request, err := h.service.RequestRefund(ctx, "alice", order, "unused rejoin")
	if err != nil || !request.Evidence.Unused() {
		t.Fatalf("refund request=%+v err=%v", request, err)
	}
	reviewed, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW)
	if err != nil || reviewed.Status != "completed" {
		t.Fatalf("refund review=%+v err=%v", reviewed, err)
	}
	if err := h.service.ReconcileRefund(ctx, request.ID); err != nil || lifecycleCount(t, h, "SELECT count(*) FROM billing_refund_provider_outcomes WHERE request_id=?", request.ID) != 1 {
		t.Fatalf("refund reconciliation replay=%v", err)
	}
}
