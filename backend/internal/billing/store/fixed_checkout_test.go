package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/plan"
)

type fixedPayments struct {
	mu                 sync.Mutex
	orders             map[string]billing.Payment
	charges            map[string]int
	captureThenTimeout bool
	decline            bool
	wrongAmount        bool
}

func (p *fixedPayments) IssueBillingKey(context.Context, string, string) (billing.BillingKey, error) {
	return billing.BillingKey{}, nil
}
func (p *fixedPayments) ParseNotification([]byte) (billing.Notification, error) {
	return billing.Notification{}, nil
}
func (p *fixedPayments) Refund(context.Context, string, string) error { return nil }
func (p *fixedPayments) PaymentByOrder(_ context.Context, orderID string) (billing.Payment, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	payment, found := p.orders[orderID]
	return payment, found, nil
}
func (p *fixedPayments) Charge(_ context.Context, request billing.ChargeRequest) (billing.Payment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.orders == nil {
		p.orders = map[string]billing.Payment{}
	}
	if p.charges == nil {
		p.charges = map[string]int{}
	}
	p.charges[request.OrderID]++
	if p.decline {
		return billing.Payment{}, &billing.ProviderError{Code: "REJECT_CARD", HTTPStatus: 402}
	}
	payment := billing.Payment{PaymentKey: "pay-" + request.OrderID, OrderID: request.OrderID,
		Status: "DONE", AmountKRW: request.KRW, Currency: "KRW"}
	if p.wrongAmount {
		payment.AmountKRW++
	}
	p.orders[request.OrderID] = payment
	if p.captureThenTimeout {
		p.captureThenTimeout = false
		return billing.Payment{}, errors.New("charge response lost")
	}
	return payment, nil
}

func fixedService(t *testing.T, at time.Time) (*ledgerHarness, *fixedPayments, *time.Time) {
	t.Helper()
	h := newLedgerHarness(t, "fixed-billing.db", at, at)
	provider := &fixedPayments{}
	clock := at
	h.service = billing.NewService(h.store, provider, nil,
		testCredits{Service: h.ledger}, nil, nil, nil).WithFixedKRW().WithClock(func() time.Time { return clock })
	return h, provider, &clock
}

func TestFixedKRWCheckoutUpgradeAndPackFromRealStore(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	monthly, err := h.service.QuotePrice(ctx, plan.Light, billing.TermMonthly)
	if err != nil || monthly.KRW != 1900 || monthly.RatePerUSDE4 != 0 {
		t.Fatalf("fixed monthly quote=%+v err=%v", monthly, err)
	}
	annual, err := h.service.QuotePrice(ctx, plan.Max, billing.TermAnnual)
	if err != nil || annual.KRW != 299000 {
		t.Fatalf("annual quote=%+v err=%v", annual, err)
	}
	if _, err := h.service.PurchasePack(ctx, "alice", "pack-1000"); !errors.Is(err, billing.ErrSubscriptionRequired) {
		t.Fatalf("free pack purchase=%v", err)
	}
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if sub.Tier != plan.Basic || !sub.TermEnd.Equal(plan.MonthBoundary(at, 1)) || len(provider.charges) != 1 {
		t.Fatalf("subscription=%+v charges=%v", sub, provider.charges)
	}
	*clock = at.Add(10 * 24 * time.Hour)
	quote, err := h.service.QuoteChange(ctx, "alice", plan.Pro, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	benefitStart, benefitEnd := plan.BenefitWindow(at, *clock)
	want, err := plan.QuoteUpgrade(plan.Basic, plan.Pro, false, at, sub.TermEnd,
		benefitStart, benefitEnd, *clock)
	if err != nil || quote.KRW != int(want.ChargeKRW) || quote.ID == "" {
		t.Fatalf("upgrade quote=%+v want=%+v err=%v", quote, want, err)
	}
	*clock = clock.Add(time.Minute)
	updated, applied, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Pro, billing.TermMonthly, quote.ID)
	if err != nil || !applied || updated.Tier != plan.Pro || !updated.AnchorAt.Equal(at) {
		t.Fatalf("upgrade=%+v applied=%t err=%v", updated, applied, err)
	}
	if _, _, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Max, billing.TermMonthly, quote.ID); !errors.Is(err, billing.ErrStaleQuote) {
		t.Fatalf("stale quote accepted: %v", err)
	}
	pack, err := h.service.PurchasePack(ctx, "alice", "pack-1000")
	if err != nil || pack.KRW != 3000 || pack.Credits != 1000 || pack.PackID != "pack-1000" || pack.RatePerUSDE4 != 0 {
		t.Fatalf("pack=%+v err=%v", pack, err)
	}
	var lotCount, expiryCount int
	if err := h.handle.Reader.QueryRowContext(ctx, `SELECT count(*),count(expires_at) FROM credit_lots
      WHERE id=? AND kind='purchased'`, pack.LotID).Scan(&lotCount, &expiryCount); err != nil || lotCount != 1 || expiryCount != 0 {
		t.Fatalf("purchased lot count=%d expiries=%d err=%v", lotCount, expiryCount, err)
	}
	var fxFields int
	if err := h.handle.Reader.QueryRowContext(ctx, `SELECT count(*) FROM billing_events WHERE kind='charge'
      AND (usd_cents IS NOT NULL OR krw_per_usd_e4 IS NOT NULL OR rate_date IS NOT NULL)`).Scan(&fxFields); err != nil || fxFields != 0 {
		t.Fatalf("checkout FX fields=%d err=%v", fxFields, err)
	}
}

func TestFixedKRWCapturedChargeRecoversAfterLocalCommitFailure(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 3, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, _ := fixedService(t, at)
	if _, err := h.handle.Writer.ExecContext(ctx, `CREATE TRIGGER block_charge BEFORE INSERT ON billing_events
      WHEN NEW.kind='charge' BEGIN SELECT RAISE(ABORT, 'simulated disk failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermAnnual); err == nil {
		t.Fatal("local commit failure should be reported")
	}
	var intents, charges int
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM billing_intents WHERE status='pending'").Scan(&intents); err != nil || intents != 1 {
		t.Fatalf("pending intent=%d err=%v", intents, err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM billing_events WHERE kind='charge'").Scan(&charges); err != nil || charges != 0 {
		t.Fatalf("premature charge event=%d err=%v", charges, err)
	}
	if _, err := h.handle.Writer.ExecContext(ctx, "DROP TRIGGER block_charge"); err != nil {
		t.Fatal(err)
	}
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	sub, found, err := h.store.Subscription(ctx, "alice")
	if err != nil || !found || sub.Term != billing.TermAnnual || sub.Tier != plan.Light {
		t.Fatalf("reconciled subscription=%+v found=%t err=%v", sub, found, err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM billing_events WHERE kind='charge'").Scan(&charges); err != nil || charges != 1 {
		t.Fatalf("duplicate charge event=%d err=%v", charges, err)
	}
	for order, n := range provider.charges {
		if n != 1 {
			t.Fatalf("order %s captured %d times", order, n)
		}
	}
}

func TestFixedKRWUnknownOutcomeAndFinalRenewalFailure(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 5, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	provider.captureThenTimeout = true
	if _, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); !errors.Is(err, billing.ErrPaymentPending) {
		t.Fatalf("unknown charge outcome=%v", err)
	}
	if _, found, err := h.store.Subscription(ctx, "alice"); err != nil || found {
		t.Fatalf("unknown outcome granted subscription: found=%t err=%v", found, err)
	}
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	sub, found, err := h.store.Subscription(ctx, "alice")
	if err != nil || !found {
		t.Fatalf("recovered subscription=%+v err=%v", sub, err)
	}
	*clock = sub.TermEnd
	provider.decline = true
	if err := h.service.RunDue(ctx, *clock); err == nil {
		t.Fatal("declined renewal should be observable")
	}
	sub, _, err = h.store.Subscription(ctx, "alice")
	if err != nil || sub.Status != "lapsed" {
		t.Fatalf("renewal failure did not lapse: %+v err=%v", sub, err)
	}
	var tier string
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT plan FROM users WHERE id='alice'").Scan(&tier); err != nil || tier != "free" {
		t.Fatalf("lapsed plan=%s err=%v", tier, err)
	}
	if _, err := h.service.PurchasePack(ctx, "alice", "pack-3000"); !errors.Is(err, billing.ErrSubscriptionRequired) {
		t.Fatalf("lapsed pack purchase=%v", err)
	}
}

func TestFixedKRWScheduledTermChangesAndAnnualBenefitProgression(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := h.service.QuoteChange(ctx, "alice", plan.Light, billing.TermAnnual)
	if err != nil || quote.AppliedNow || !quote.EffectiveAt.Equal(sub.TermEnd) {
		t.Fatalf("monthly to annual quote=%+v err=%v", quote, err)
	}
	scheduled, applied, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Light, billing.TermAnnual, quote.ID)
	if err != nil || applied || scheduled.ScheduledTerm == nil || *scheduled.ScheduledTerm != billing.TermAnnual {
		t.Fatalf("scheduled term=%+v applied=%t err=%v", scheduled, applied, err)
	}
	if len(provider.charges) != 1 {
		t.Fatalf("scheduling charged: %v", provider.charges)
	}
	if _, err := h.service.CancelScheduledChange(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	quote, err = h.service.QuoteChange(ctx, "alice", plan.Basic, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	if _, applied, err = h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Basic, billing.TermAnnual, quote.ID); err != nil || applied {
		t.Fatalf("replacement schedule applied=%t err=%v", applied, err)
	}
	*clock = sub.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	annual, found, err := h.store.Subscription(ctx, "alice")
	if err != nil || !found || annual.Term != billing.TermAnnual || annual.Tier != plan.Basic || !annual.AnchorAt.Equal(at) {
		t.Fatalf("annual renewal=%+v found=%t err=%v", annual, found, err)
	}
	if len(provider.charges) != 2 || annual.TermEnd.Sub(annual.TermStart) < 360*24*time.Hour {
		t.Fatalf("annual charged incorrectly: %v term=%+v", provider.charges, annual)
	}
	*clock = plan.MonthBoundary(at, 2)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	if len(provider.charges) != 2 {
		t.Fatalf("monthly annual benefit charged: %v", provider.charges)
	}
	quote, err = h.service.QuoteChange(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if _, applied, err = h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Basic, billing.TermMonthly, quote.ID); err != nil || applied {
		t.Fatalf("annual to monthly applied=%t err=%v", applied, err)
	}
	if len(provider.charges) != 2 {
		t.Fatalf("early annual conversion charge: %v", provider.charges)
	}
	*clock = annual.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	monthly, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || monthly.Term != billing.TermMonthly || monthly.Tier != plan.Basic || len(provider.charges) != 3 {
		t.Fatalf("annual expiry conversion=%+v charges=%v err=%v", monthly, provider.charges, err)
	}
}

func TestFixedKRWSimultaneousUpgradeChargesOnce(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 6, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(24 * time.Hour)
	quote, err := h.service.QuoteChange(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Basic, billing.TermMonthly, quote.ID)
			results <- err
		}()
	}
	group.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, billing.ErrPaymentPending) && !errors.Is(err, billing.ErrStaleQuote) {
			t.Fatalf("unexpected concurrent outcome=%v", err)
		}
	}
	if success != 1 || len(provider.charges) != 2 {
		t.Fatalf("upgrade success=%d charges=%v", success, provider.charges)
	}
}

func TestFixedKRWPackReplayAndProviderAmountMismatch(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, _ := fixedService(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	provider.captureThenTimeout = true
	if _, err := h.service.PurchasePack(ctx, "alice", "pack-3000"); !errors.Is(err, billing.ErrPaymentPending) {
		t.Fatalf("lost pack response=%v", err)
	}
	var purchased int
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_purchases").Scan(&purchased); err != nil || purchased != 0 {
		t.Fatalf("unconfirmed pack count=%d err=%v", purchased, err)
	}
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_purchases").Scan(&purchased); err != nil || purchased != 1 {
		t.Fatalf("replayed pack count=%d err=%v", purchased, err)
	}
	var packOrder string
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT id FROM credit_purchases").Scan(&packOrder); err != nil {
		t.Fatal(err)
	}
	provider.mu.Lock()
	late := provider.orders[packOrder]
	late.Status = "ABORTED"
	provider.orders[packOrder] = late
	provider.mu.Unlock()
	if err := h.service.ReconcileOrder(ctx, packOrder); err != nil {
		t.Fatalf("late conflicting notification changed settled order: %v", err)
	}
	if err := h.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM credit_purchases").Scan(&purchased); err != nil || purchased != 1 {
		t.Fatalf("late notification changed purchase count=%d err=%v", purchased, err)
	}
	if _, err := h.service.PurchasePack(ctx, "alice", "arbitrary"); !errors.Is(err, billing.ErrInvalidPack) {
		t.Fatalf("arbitrary pack=%v", err)
	}
	bad, wrongProvider, _ := fixedService(t, at)
	wrongProvider.wrongAmount = true
	if _, err := bad.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); !errors.Is(err, billing.ErrPaymentPending) {
		t.Fatalf("amount mismatch=%v", err)
	}
	if _, found, err := bad.store.Subscription(ctx, "alice"); err != nil || found {
		t.Fatalf("wrong amount granted subscription found=%t err=%v", found, err)
	}
	var review int
	if err := bad.handle.Reader.QueryRowContext(ctx, "SELECT count(*) FROM billing_intents WHERE status='review'").Scan(&review); err != nil || review != 1 {
		t.Fatalf("mismatched order review=%d err=%v", review, err)
	}
}

func TestFixedKRWAnnualUpgradeAndCancelResumeKeepAnchors(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, clock := fixedService(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	*clock = at.AddDate(0, 4, 0)
	quote, err := h.service.QuoteChange(ctx, "alice", plan.Pro, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	start, end := plan.BenefitWindow(at, *clock)
	want, err := plan.QuoteUpgrade(plan.Basic, plan.Pro, true, sub.TermStart, sub.TermEnd, start, end, *clock)
	if err != nil || quote.KRW != int(want.ChargeKRW) || quote.KRW <= 0 {
		t.Fatalf("annual proration=%+v want=%+v err=%v", quote, want, err)
	}
	upgraded, applied, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Pro, billing.TermAnnual, quote.ID)
	if err != nil || !applied || upgraded.Tier != plan.Pro || !upgraded.TermEnd.Equal(sub.TermEnd) || !upgraded.AnchorAt.Equal(at) {
		t.Fatalf("annual upgrade=%+v applied=%t err=%v", upgraded, applied, err)
	}
	quote, err = h.service.QuoteChange(ctx, "alice", plan.Light, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	if _, applied, err = h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Light, billing.TermAnnual, quote.ID); err != nil || applied {
		t.Fatalf("downgrade schedule applied=%t err=%v", applied, err)
	}
	quote, err = h.service.QuoteChange(ctx, "alice", plan.Max, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	upgraded, applied, err = h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Max, billing.TermAnnual, quote.ID)
	if err != nil || !applied || upgraded.ScheduledTier != nil || upgraded.ScheduledTerm != nil {
		t.Fatalf("upgrade did not clear scheduled downgrade: %+v err=%v", upgraded, err)
	}
	count := len(provider.charges)
	cancelled, err := h.service.CancelSubscription(ctx, "alice")
	if err != nil || cancelled.AutoRenew {
		t.Fatalf("cancel=%+v err=%v", cancelled, err)
	}
	if coverage, yes, err := h.service.CoverageAt(ctx, "alice", *clock); err != nil || !yes || coverage.Tier != plan.Max {
		t.Fatalf("cancelled paid coverage=%+v yes=%t err=%v", coverage, yes, err)
	}
	resumed, err := h.service.ResumeSubscription(ctx, "alice")
	if err != nil || !resumed.AutoRenew || !resumed.AnchorAt.Equal(at) || len(provider.charges) != count {
		t.Fatalf("resume=%+v charges=%v err=%v", resumed, provider.charges, err)
	}
	if _, err := h.service.CancelSubscription(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	*clock = sub.TermEnd
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	lapsed, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || lapsed.Status != "lapsed" || len(provider.charges) != count {
		t.Fatalf("cancelled expiry=%+v charges=%v err=%v", lapsed, provider.charges, err)
	}
	*clock = clock.Add(time.Minute)
	rejoined, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly)
	if err != nil || rejoined.Tier != plan.Light || !rejoined.AnchorAt.Equal(*clock) ||
		rejoined.CoverageID == sub.CoverageID || len(provider.charges) != count+1 {
		t.Fatalf("paid rejoin=%+v charges=%v err=%v", rejoined, provider.charges, err)
	}
}
