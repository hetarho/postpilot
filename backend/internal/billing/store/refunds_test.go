package store_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

type refundTestBenefits struct {
	credits *usagestore.Store
	exports *clipstore.Store
}

func (b refundTestBenefits) credit(p billing.RefundPayment) usage.RefundFunding {
	return usage.RefundFunding{UserID: p.UserID, OrderID: p.OrderID, Kind: p.Kind, LotID: p.PackLotID,
		CoverageID: p.CoverageID, Start: p.EffectiveAt, End: p.FundingEnd}
}
func (b refundTestBenefits) export(p billing.RefundPayment) clip.RefundFunding {
	return clip.RefundFunding{UserID: p.UserID, OrderID: p.OrderID, Kind: p.Kind,
		CoverageID: p.CoverageID, Start: p.EffectiveAt, End: p.FundingEnd}
}
func (b refundTestBenefits) Inspect(ctx context.Context, p billing.RefundPayment, at time.Time) (billing.RefundEvidence, error) {
	c, err := b.credits.RefundFundingEvidence(ctx, b.credit(p), at)
	if err != nil {
		return billing.RefundEvidence{}, err
	}
	e := billing.RefundEvidence{PaidModelJobs: c.PaidJobs, CreditsUsed: c.CreditsUsed, CreditsReserved: c.CreditsReserved, FundedCreditsRemaining: c.CreditsRemaining}
	if p.Kind != "pack" {
		x, err := b.exports.RefundFundingEvidence(ctx, b.export(p))
		if err != nil {
			return e, err
		}
		e.ServerExportsUsed = x.ExportsUsed
		e.ServerExportsReserved = x.ExportsReserved
		e.FundedExportsRemaining = x.ExportsRemaining
	}
	return e, nil
}
func (b refundTestBenefits) Guard(ctx context.Context, r billing.RefundRequest, p billing.RefundPayment, _ time.Time) error {
	if err := b.credits.GuardRefundFunding(ctx, b.credit(p), r.ID); err != nil {
		return err
	}
	if p.Kind != "pack" {
		return b.exports.GuardRefundFunding(ctx, b.export(p), r.ID)
	}
	return nil
}
func (b refundTestBenefits) Release(ctx context.Context, r billing.RefundRequest, p billing.RefundPayment) error {
	if err := b.credits.ReleaseRefundFunding(ctx, r.ID); err != nil {
		return err
	}
	if p.Kind != "pack" {
		return b.exports.ReleaseRefundFunding(ctx, r.ID)
	}
	return nil
}
func (b refundTestBenefits) Confirm(ctx context.Context, r billing.RefundRequest, p billing.RefundPayment, at time.Time) error {
	if err := b.credits.ConfirmRefundFunding(ctx, r.ID, at); err != nil {
		return err
	}
	if p.Kind != "pack" {
		return b.exports.ConfirmRefundFunding(ctx, b.export(p), r.ID, at)
	}
	return nil
}

type reviewPayments struct {
	*fixedPayments
	mu                          sync.Mutex
	lookupOnce                  sync.Once
	lookupReady, lookupContinue chan struct{}
	canceled                    map[string]billing.Payment
	calls                       int
	timeout                     bool
	decline                     bool
	// refusal answers every cancel with this provider error and cancels nothing.
	refusal *billing.ProviderError
	// beforeCancel runs as a cancel arrives, before the fake decides its answer.
	beforeCancel func()
}

func (p *reviewPayments) PaymentByOrder(ctx context.Context, orderID string) (billing.Payment, bool, error) {
	if p.lookupReady != nil {
		p.lookupOnce.Do(func() { close(p.lookupReady); <-p.lookupContinue })
	}
	return p.fixedPayments.PaymentByOrder(ctx, orderID)
}

func (p *reviewPayments) CancelPayment(_ context.Context, key string, amount int, _, idempotency string) (billing.Payment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.beforeCancel != nil {
		p.beforeCancel()
	}
	if p.decline {
		return billing.Payment{}, &billing.ProviderError{Code: "INVALID_REQUEST", HTTPStatus: 400}
	}
	if p.refusal != nil {
		return billing.Payment{}, p.refusal
	}
	if prior, ok := p.canceled[idempotency]; ok {
		return prior, nil
	}
	p.fixedPayments.mu.Lock()
	defer p.fixedPayments.mu.Unlock()
	for order, payment := range p.fixedPayments.orders {
		if payment.PaymentKey != key {
			continue
		}
		if amount <= 0 || amount > payment.BalanceKRW {
			return billing.Payment{}, errors.New("invalid amount")
		}
		payment.BalanceKRW -= amount
		payment.Cancels = append(payment.Cancels, billing.PaymentCancel{TransactionKey: "cancel-" + idempotency, AmountKRW: amount, Status: "DONE"})
		if payment.BalanceKRW == 0 {
			payment.Status = "CANCELED"
		} else {
			payment.Status = "PARTIAL_CANCELED"
		}
		p.fixedPayments.orders[order] = payment
		if p.canceled == nil {
			p.canceled = map[string]billing.Payment{}
		}
		p.canceled[idempotency] = payment
		if p.timeout {
			p.timeout = false
			return billing.Payment{}, errors.New("response lost")
		}
		return payment, nil
	}
	return billing.Payment{}, errors.New("payment missing")
}

func refundHarness(t *testing.T, at time.Time) (*ledgerHarness, *reviewPayments, *time.Time) {
	t.Helper()
	h, base, clock := fixedService(t, at)
	if _, err := h.handle.Writer.Exec(`INSERT INTO users (id,password_hash,plan,email,email_verified_at,created_at)
		VALUES ('operator','hash','master','operator@example.com',?,?)`,
		at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	p := &reviewPayments{fixedPayments: base}
	h.store.SetRefundBenefits(refundTestBenefits{usagestore.New(h.handle.Writer, h.handle.Reader), clipstore.New(h.handle.Writer, h.handle.Reader)})
	h.store.SetRefundBenefitsForTx(func(tx *sql.Tx) billing.RefundBenefits {
		return refundTestBenefits{usagestore.NewTx(tx), clipstore.NewTx(tx)}
	})
	h.service = billing.NewService(h.store, p, nil, testCredits{Service: h.ledger, exports: clipstore.New(h.handle.Writer, h.handle.Reader)}, nil, nil, nil).WithFixedKRW().WithClock(func() time.Time { return *clock })
	return h, p, clock
}

func chargeOrder(t *testing.T, h *ledgerHarness, kind string) string {
	t.Helper()
	var order string
	if err := h.handle.Reader.QueryRow(`SELECT order_id FROM billing_intents WHERE kind=? AND status='applied' ORDER BY applied_at DESC LIMIT 1`, kind).Scan(&order); err != nil {
		t.Fatal(err)
	}
	return order
}

func TestReviewedRefundOfUnusedSubscriptionPreservesOtherPaymentAndReplaysOnce(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	h, p, clock := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	order := chargeOrder(t, h, "subscribe")
	pack, err := h.service.PurchasePack(ctx, "alice", "pack-1000")
	if err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(6 * 24 * time.Hour)
	request, err := h.service.RequestRefund(ctx, "alice", order, "unused subscription")
	if err != nil {
		t.Fatal(err)
	}
	if !request.Evidence.Unused() {
		t.Fatalf("unexpected use: %+v", request.Evidence)
	}
	if _, err := h.service.RequestRefund(ctx, "bob", order, "other owner"); !errors.Is(err, billing.ErrRefundNotFound) {
		t.Fatalf("owner leak: %v", err)
	}
	if _, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW/2); !errors.Is(err, billing.ErrRefundAmount) {
		t.Fatalf("partial unused refund: %v", err)
	}
	p.timeout = true
	started, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW)
	if err != nil || started.Status != "processing" {
		t.Fatalf("timeout review=%+v err=%v", started, err)
	}
	lots, err := usagestore.New(h.handle.Writer, h.handle.Reader).LotsInConsumptionOrder(ctx, "alice", *clock)
	if err != nil {
		t.Fatal(err)
	}
	for _, lot := range lots {
		if lot.CoverageID == request.Payment.CoverageID {
			t.Fatalf("guarded lot spendable: %+v", lot)
		}
	}
	if err := clipstore.New(h.handle.Writer, h.handle.Reader).ReserveExport(ctx, "alice", "project-1", 1, "render-1", *clock); err == nil {
		t.Fatal("guarded export was reserved")
	}
	if err := h.service.ReconcilePendingRefunds(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.service.ReconcileRefund(ctx, request.ID); err != nil {
		t.Fatal(err)
	}
	resolved, err := h.service.RefundRequest(ctx, request.ID)
	if err != nil || resolved.Status != "completed" || resolved.ConfirmedAmountKRW != request.Payment.KRW || !resolved.Evidence.Unused() {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	if p.calls != 2 {
		t.Fatalf("cancel attempts=%d", p.calls)
	}
	var remaining, events int
	if err := h.handle.Reader.QueryRow(`SELECT remaining FROM credit_lots WHERE id=?`, pack.LotID).Scan(&remaining); err != nil || remaining != pack.Credits {
		t.Fatalf("unrelated pack=%d err=%v", remaining, err)
	}
	if err := h.handle.Reader.QueryRow(`SELECT count(*) FROM billing_events WHERE kind='refund' AND note=?`, request.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("refund events=%d err=%v", events, err)
	}
	sub, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || sub.Status != "lapsed" {
		t.Fatalf("subscription=%+v err=%v", sub, err)
	}
	packRequest, err := h.service.RequestRefund(ctx, "alice", pack.OrderID, "unused purchased credits")
	if err != nil || !packRequest.Evidence.Unused() {
		t.Fatalf("pack request=%+v err=%v", packRequest, err)
	}
	packDecision, err := h.service.ReviewRefund(ctx, "operator", packRequest.ID, "approve", pack.KRW)
	if err != nil || packDecision.Status != "completed" {
		t.Fatalf("pack decision=%+v err=%v", packDecision, err)
	}
	if err := h.handle.Reader.QueryRow(`SELECT remaining FROM credit_lots WHERE id=?`, pack.LotID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("refunded pack remaining=%d err=%v", remaining, err)
	}
}

func TestRefundReviewAtExactlySevenDaysAllowsOperatorAmount(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	h, _, clock := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermAnnual); err != nil {
		t.Fatal(err)
	}
	order := chargeOrder(t, h, "subscribe")
	request, err := h.service.RequestRefund(ctx, "alice", order, "seven-day boundary")
	if err != nil {
		t.Fatal(err)
	}
	if !request.Evidence.Unused() {
		t.Fatalf("evidence=%+v", request.Evidence)
	}
	*clock = at.Add(7 * 24 * time.Hour)
	approved, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW/2)
	if err != nil || approved.Status != "completed" {
		t.Fatalf("review=%+v err=%v", approved, err)
	}
	if approved.ConfirmedAmountKRW != request.Payment.KRW/2 {
		t.Fatal(approved)
	}
}

func TestRefundRequestAfterSevenDaysRemainsReviewable(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	h, _, clock := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	order := chargeOrder(t, h, "subscribe")
	*clock = at.Add(8 * 24 * time.Hour)
	request, err := h.service.RequestRefund(ctx, "alice", order, "later request")
	if err != nil || request.Status != "requested" {
		t.Fatalf("later request=%+v err=%v", request, err)
	}
	decision, err := h.service.ReviewRefund(ctx, "operator", request.ID, "reject", 0)
	if err != nil || decision.Status != "rejected" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
}

func TestDefinitiveProviderRefusalReleasesFundedGuard(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	h, provider, clock := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	order := chargeOrder(t, h, "subscribe")
	request, err := h.service.RequestRefund(ctx, "alice", order, "cancel declined")
	if err != nil {
		t.Fatal(err)
	}
	provider.decline = true
	if _, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW); !errors.Is(err, billing.ErrRefundFailed) {
		t.Fatalf("provider refusal=%v", err)
	}
	stored, err := h.service.RefundRequest(ctx, request.ID)
	if err != nil || stored.Status != "failed" {
		t.Fatalf("failed status=%+v err=%v", stored, err)
	}
	lots, err := usagestore.New(h.handle.Writer, h.handle.Reader).LotsInConsumptionOrder(ctx, "alice", *clock)
	if err != nil || len(lots) == 0 {
		t.Fatalf("guard not released: lots=%+v err=%v", lots, err)
	}
}

func TestConcurrentConsumptionBeforeReviewIsRevalidated(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	h, provider, _ := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	order := chargeOrder(t, h, "subscribe")
	request, err := h.service.RequestRefund(ctx, "alice", order, "concurrent use")
	if err != nil || !request.Evidence.Unused() {
		t.Fatalf("initial=%+v err=%v", request, err)
	}
	provider.lookupReady, provider.lookupContinue = make(chan struct{}), make(chan struct{})
	type result struct {
		item billing.RefundRequest
		err  error
	}
	finished := make(chan result, 1)
	go func() {
		item, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW)
		finished <- result{item, err}
	}()
	select {
	case <-provider.lookupReady:
	case <-time.After(5 * time.Second):
		t.Fatal("provider read did not start")
	}
	if _, err := h.handle.Writer.Exec(`UPDATE credit_lots SET remaining=remaining-1 WHERE user_id='alice' AND kind='monthly' AND remaining>0`); err != nil {
		t.Fatal(err)
	}
	close(provider.lookupContinue)
	select {
	case reviewed := <-finished:
		if reviewed.err != nil || reviewed.item.Status != "completed" || reviewed.item.Evidence.CreditsUsed != 1 {
			t.Fatalf("revalidated=%+v err=%v", reviewed.item, reviewed.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("review did not finish")
	}
}

func TestRefundReviewCountsActiveFundedHoldButNotFreeWork(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	h, provider, clock := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	order := chargeOrder(t, h, "subscribe")
	if _, err := h.handle.Writer.Exec(`INSERT INTO usage_admissions(user_id,kind,job_id,created_at)
		VALUES ('alice','generate_post','free-job',?)`, at.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	request, err := h.service.RequestRefund(ctx, "alice", order, "unused paid benefits")
	if err != nil || !request.Evidence.Unused() {
		t.Fatalf("free work evidence=%+v err=%v", request.Evidence, err)
	}
	var lot string
	if err := h.handle.Writer.QueryRow(`SELECT id FROM credit_lots WHERE user_id='alice' AND kind='monthly' LIMIT 1`).Scan(&lot); err != nil {
		t.Fatal(err)
	}
	if _, err := h.handle.Writer.Exec(`INSERT INTO usage_admissions(user_id,kind,job_id,created_at,hold_credits)
		VALUES ('alice','generate_post','paid-job',?,1)`, at.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.handle.Writer.Exec(`INSERT INTO credit_hold_lots(job_id,lot_id,credits) VALUES ('paid-job',?,1)`, lot); err != nil {
		t.Fatal(err)
	}
	if _, err := h.handle.Writer.Exec(`UPDATE credit_lots SET remaining=remaining-1 WHERE id=?`, lot); err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(time.Hour)
	items, err := h.service.RefundRequests(ctx, "alice")
	if err != nil || len(items) != 1 || items[0].Evidence.CreditsReserved != 1 || items[0].Evidence.PaidModelJobs != 1 {
		t.Fatalf("hold evidence=%+v err=%v", items, err)
	}
	if _, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW); !errors.Is(err, billing.ErrRefundActiveUse) {
		t.Fatalf("active hold accepted: %v", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider called during active hold: %d", provider.calls)
	}
	if _, err := h.handle.Writer.Exec(`UPDATE usage_admissions SET settled_at=?,settled_credits=1 WHERE job_id='paid-job'`, at.UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	approved, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW/2)
	if err != nil || approved.Status != "completed" {
		t.Fatalf("review after settlement=%+v err=%v", approved, err)
	}
}

func TestRefundOfUpgradeRevertsOnlyUpgradeFunding(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	h, _, clock := refundHarness(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	baseOrder := chargeOrder(t, h, "subscribe")
	*clock = at.Add(2 * 24 * time.Hour)
	quote, err := h.service.QuoteChange(ctx, "alice", plan.Pro, billing.TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	if _, applied, err := h.service.ChangeSubscriptionQuoted(ctx, "alice", plan.Pro, billing.TermAnnual, quote.ID); err != nil || !applied {
		t.Fatalf("upgrade=%v applied=%t", err, applied)
	}
	upgradeOrder := chargeOrder(t, h, "upgrade")
	baseRequest, err := h.service.RequestRefund(ctx, "alice", baseOrder, "base payment")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.ReviewRefund(ctx, "operator", baseRequest.ID, "approve", baseRequest.Payment.KRW); !errors.Is(err, billing.ErrRefundDependentPayment) {
		t.Fatalf("base refund skipped active upgrade: %v", err)
	}
	request, err := h.service.RequestRefund(ctx, "alice", upgradeOrder, "unused upgrade")
	if err != nil || request.Payment.PriorTier != plan.Basic || !request.Payment.FundingEnd.Equal(sub.TermEnd) || !request.Evidence.Unused() {
		t.Fatalf("request=%+v err=%v", request, err)
	}
	approved, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW)
	if err != nil || approved.Status != "completed" {
		t.Fatalf("review=%+v err=%v", approved, err)
	}
	current, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || current.Tier != plan.Basic || current.Status != "active" {
		t.Fatalf("subscription=%+v err=%v", current, err)
	}
	base, found, err := h.store.RefundPayment(ctx, "alice", baseOrder)
	if err != nil || !found || base.CoverageID != request.Payment.CoverageID {
		t.Fatalf("base=%+v found=%t err=%v", base, found, err)
	}
	var baseRemaining int
	if err := h.handle.Reader.QueryRow(`SELECT coalesce(sum(remaining),0) FROM credit_lots WHERE coverage_id=? AND kind='monthly' AND issuance_cause<>'upgrade'`, base.CoverageID).Scan(&baseRemaining); err != nil || baseRemaining == 0 {
		t.Fatalf("base remaining=%d err=%v", baseRemaining, err)
	}
	baseDecision, err := h.service.ReviewRefund(ctx, "operator", baseRequest.ID, "approve", baseRequest.Payment.KRW)
	if err != nil || baseDecision.Status != "completed" {
		t.Fatalf("base after upgrade=%+v err=%v", baseDecision, err)
	}
}

// QUOTA-63: a confirmed refund voids the entitlement the refunded payment funded, never the
// master tier the account holds since (BILL-20).
func TestConfirmedRefundNeverDemotesAMaster(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	h, _, clock := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	order := chargeOrder(t, h, "subscribe")
	if err := h.service.AssignSupportTier(ctx, "alice", plan.Master); err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(24 * time.Hour)
	request, err := h.service.RequestRefund(ctx, "alice", order, "promoted to operator")
	if err != nil {
		t.Fatal(err)
	}
	decided, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", request.Payment.KRW)
	if err != nil || decided.Status != "completed" {
		t.Fatalf("review=%+v err=%v", decided, err)
	}
	var tier string
	if err := h.handle.Reader.QueryRow(`SELECT plan FROM users WHERE id='alice'`).Scan(&tier); err != nil || tier != "master" {
		t.Fatalf("plan after refund = %s err=%v, want master", tier, err)
	}
}

// F3: a later partial refund of a pack whose purchase the first refund already marked is
// recorded once like the first — its event, its share of the confirmed total, completion.
func TestSecondPartialRefundOfAPackIsRecordedLikeTheFirst(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	h, provider, clock := refundHarness(t, at)
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
		t.Fatal(err)
	}
	pack, err := h.service.PurchasePack(ctx, "alice", "pack-3000")
	if err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(8 * 24 * time.Hour)
	for index, amount := range []int{4000, 3000} {
		request, err := h.service.RequestRefund(ctx, "alice", pack.OrderID, "partial refund")
		if err != nil {
			t.Fatalf("request %d: %v", index, err)
		}
		reviewed, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", amount)
		if err != nil || reviewed.Status != "completed" || reviewed.ConfirmedAmountKRW != amount {
			t.Fatalf("refund %d=%+v err=%v", index, reviewed, err)
		}
		*clock = clock.Add(time.Hour)
	}
	if total, err := h.store.ConfirmedRefundTotal(ctx, pack.OrderID); err != nil || total != 7000 {
		t.Fatalf("confirmed total=%d err=%v", total, err)
	}
	var events, refunded int
	if err := h.handle.Reader.QueryRow(`SELECT count(*),coalesce(sum(krw),0) FROM billing_events
		WHERE kind='refund' AND provider_payment_key=?`, pack.ProviderPaymentKey).Scan(&events, &refunded); err != nil ||
		events != 2 || refunded != 7000 {
		t.Fatalf("refund events=%d sum=%d err=%v", events, refunded, err)
	}
	if provider.calls != 2 {
		t.Fatalf("cancel attempts=%d", provider.calls)
	}
	var remaining int
	if err := h.handle.Reader.QueryRow(`SELECT remaining FROM credit_lots WHERE id=?`, pack.LotID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("refunded pack remaining=%d err=%v", remaining, err)
	}
}

// F12: a cancel the provider definitively refuses is settled by reading the payment back:
// an untouched balance fails the request and frees the frozen lot, our amount already
// cancelled completes it, and anything else leaves it processing.
func TestDefinitiveCancelRefusalIsSettledByThePaymentReadBack(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	packRequest := func(t *testing.T) (*ledgerHarness, *reviewPayments, billing.Purchase, billing.RefundRequest) {
		t.Helper()
		h, provider, _ := refundHarness(t, at)
		if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil {
			t.Fatal(err)
		}
		pack, err := h.service.PurchasePack(ctx, "alice", "pack-1000")
		if err != nil {
			t.Fatal(err)
		}
		request, err := h.service.RequestRefund(ctx, "alice", pack.OrderID, "unused credits")
		if err != nil {
			t.Fatal(err)
		}
		return h, provider, pack, request
	}
	guardOf := func(t *testing.T, h *ledgerHarness, lotID string) (guard sql.NullString, remaining int) {
		t.Helper()
		if err := h.handle.Reader.QueryRow(`SELECT refund_request_id,remaining FROM credit_lots WHERE id=?`, lotID).
			Scan(&guard, &remaining); err != nil {
			t.Fatal(err)
		}
		return guard, remaining
	}

	for _, refusal := range []*billing.ProviderError{
		{Code: "NOT_CANCELABLE_AMOUNT", HTTPStatus: 403},
		{Code: "ALREADY_CANCELED_PAYMENT", HTTPStatus: 400},
	} {
		t.Run(refusal.Code+" with an unchanged balance fails and frees the lot", func(t *testing.T) {
			h, provider, pack, request := packRequest(t)
			provider.refusal = refusal
			if _, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", pack.KRW); !errors.Is(err, billing.ErrRefundFailed) {
				t.Fatalf("refusal=%v", err)
			}
			stored, err := h.service.RefundRequest(ctx, request.ID)
			if err != nil || stored.Status != "failed" || stored.ProviderStatus != refusal.Code {
				t.Fatalf("request=%+v err=%v", stored, err)
			}
			if guard, remaining := guardOf(t, h, pack.LotID); guard.Valid || remaining != pack.Credits {
				t.Fatalf("lot still frozen: guard=%v remaining=%d", guard, remaining)
			}
			lots, err := usagestore.New(h.handle.Writer, h.handle.Reader).LotsInConsumptionOrder(ctx, "alice", at)
			if err != nil {
				t.Fatal(err)
			}
			spendable := false
			for _, lot := range lots {
				spendable = spendable || lot.ID == pack.LotID
			}
			if !spendable {
				t.Fatalf("pack lot not spendable: %+v", lots)
			}
		})
	}

	t.Run("a refusal after our cancel went through completes", func(t *testing.T) {
		h, provider, pack, request := packRequest(t)
		provider.timeout = true
		started, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", pack.KRW)
		if err != nil || started.Status != "processing" {
			t.Fatalf("lost response=%+v err=%v", started, err)
		}
		provider.refusal = &billing.ProviderError{Code: "ALREADY_CANCELED_PAYMENT", HTTPStatus: 400}
		if err := h.service.ReconcileRefund(ctx, request.ID); err != nil {
			t.Fatal(err)
		}
		resolved, err := h.service.RefundRequest(ctx, request.ID)
		if err != nil || resolved.Status != "completed" || resolved.ConfirmedAmountKRW != pack.KRW ||
			resolved.ProviderTransactionKey != "cancel-refund:"+request.ID {
			t.Fatalf("resolved=%+v err=%v", resolved, err)
		}
	})

	t.Run("a refusal after someone else moved the balance stays processing", func(t *testing.T) {
		h, provider, pack, request := packRequest(t)
		provider.refusal = &billing.ProviderError{Code: "NOT_CANCELABLE_AMOUNT", HTTPStatus: 403}
		provider.beforeCancel = func() {
			provider.fixedPayments.mu.Lock()
			defer provider.fixedPayments.mu.Unlock()
			payment := provider.fixedPayments.orders[pack.OrderID]
			payment.BalanceKRW -= 100
			payment.Cancels = append(payment.Cancels, billing.PaymentCancel{TransactionKey: "dashboard", AmountKRW: 100, Status: "DONE"})
			provider.fixedPayments.orders[pack.OrderID] = payment
		}
		reviewed, err := h.service.ReviewRefund(ctx, "operator", request.ID, "approve", pack.KRW)
		if err != nil || reviewed.Status != "processing" {
			t.Fatalf("review=%+v err=%v", reviewed, err)
		}
		if guard, _ := guardOf(t, h, pack.LotID); guard.String != request.ID {
			t.Fatalf("lot guard=%v, want %s", guard, request.ID)
		}
	})
}
