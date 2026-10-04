package store_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/plan"
)

type recordedMail struct{ to, subject, text string }

type recordingMailer struct {
	mu   sync.Mutex
	sent []recordedMail
}

func (m *recordingMailer) Send(_ context.Context, to, subject, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, recordedMail{to, subject, text})
	return nil
}

func (m *recordingMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// unappliedHarness is refundHarness with a verified address and a mailer, so the refund mail
// (BILL-12) is observable.
func unappliedHarness(t *testing.T, at time.Time) (*ledgerHarness, *reviewPayments, *recordingMailer, *time.Time) {
	t.Helper()
	h, provider, clock := refundHarness(t, at)
	mailer := &recordingMailer{}
	h.service = billing.NewService(h.store, provider,
		testCredits{Service: h.ledger, exports: clipstore.New(h.handle.Writer, h.handle.Reader)},
		nil, registrationAccounts{}, mailer).WithClock(func() time.Time { return *clock })
	return h, provider, mailer, clock
}

// stalePackInReview leaves alice subscribed with a pack order the provider captured at a price
// the pack no longer has: the pass moves it to review, and it locks her billing.
func stalePackInReview(t *testing.T, h *ledgerHarness, clock *time.Time, at time.Time) billing.Subscription {
	t.Helper()
	ctx := context.Background()
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.InsertIntent(ctx, packIntent("alice", "pp-buy-stale", 2500, at)); err != nil {
		t.Fatal(err)
	}
	*clock = at.Add(settledByWorker)
	if err := h.service.ReconcilePending(ctx); err != nil {
		t.Fatal(err)
	}
	if got := intentStatus(t, h, "pp-buy-stale"); got != "review" {
		t.Fatalf("stale pack order status = %s, want review", got)
	}
	if _, err := h.service.PurchasePack(ctx, "alice", "pack-1000"); !errors.Is(err, billing.ErrPaymentPending) {
		t.Fatalf("purchase while an order is in review = %v, want ErrPaymentPending", err)
	}
	return sub
}

func unappliedRefunds(t *testing.T, h *ledgerHarness, orderID string) (count, krw int) {
	t.Helper()
	if err := h.handle.Reader.QueryRow(`SELECT count(*),coalesce(sum(krw),0) FROM billing_events
		WHERE kind='refund' AND note=?`, orderID).Scan(&count, &krw); err != nil {
		t.Fatal(err)
	}
	return count, krw
}

func providerPayment(p *reviewPayments, orderID string) billing.Payment {
	p.fixedPayments.mu.Lock()
	defer p.fixedPayments.mu.Unlock()
	return p.fixedPayments.orders[orderID]
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logs
}

func logLine(logs *bytes.Buffer, message string) string {
	line := ""
	for _, candidate := range strings.Split(logs.String(), "\n") {
		if strings.Contains(candidate, message) {
			line = candidate
		}
	}
	return line
}

// BILL-22: the next billing pass returns a captured payment the product could not apply in
// full, fails the order, records the refund once and mails it, and the account buys again.
func TestUnappliedPackCaptureIsRefundedInFullByTheNextPass(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, mailer, clock := unappliedHarness(t, at)
	stalePackInReview(t, h, clock, at)
	mailed := mailer.count()

	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("RunDue = %v", err)
	}
	if got := intentStatus(t, h, "pp-buy-stale"); got != "failed" {
		t.Fatalf("refunded order status = %s, want failed", got)
	}
	if count, krw := unappliedRefunds(t, h, "pp-buy-stale"); count != 1 || krw != 2500 {
		t.Fatalf("refund events=%d krw=%d, want one of 2500", count, krw)
	}
	if paid := providerPayment(provider, "pp-buy-stale"); paid.BalanceKRW != 0 || paid.Status != "CANCELED" {
		t.Fatalf("provider payment after the refund = %+v", paid)
	}
	if len(provider.keys) != 1 || provider.keys[0] != "unapplied:pp-buy-stale" {
		t.Fatalf("cancel idempotency keys = %v", provider.keys)
	}
	var purchases int
	if err := h.handle.Reader.QueryRow(`SELECT count(*) FROM credit_purchases WHERE order_id='pp-buy-stale'`).Scan(&purchases); err != nil || purchases != 0 {
		t.Fatalf("unapplied pack granted: purchases=%d err=%v", purchases, err)
	}
	if mailer.count() != mailed+1 {
		t.Fatalf("mails = %+v, want one refund mail", mailer.sent)
	}
	refundMail := mailer.sent[len(mailer.sent)-1]
	if refundMail.to != "alice@example.com" || !strings.Contains(refundMail.subject, "could not be applied") ||
		!strings.Contains(refundMail.text, "2500원") || !strings.Contains(refundMail.text, "KRW 2500") {
		t.Fatalf("refund mail = %+v", refundMail)
	}

	// The pending-order lock went with the order: the account buys on its next call.
	pack, err := h.service.PurchasePack(ctx, "alice", "pack-1000")
	if err != nil || pack.KRW != 3000 {
		t.Fatalf("purchase after the refund = %+v err=%v", pack, err)
	}

	// A replayed pass finds nothing in review and sends nothing.
	calls, mailed := provider.calls, mailer.count()
	*clock = clock.Add(time.Hour)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("replayed RunDue = %v", err)
	}
	if provider.calls != calls || mailer.count() != mailed {
		t.Fatalf("replayed pass cancelled %d→%d times, mailed %d→%d", calls, provider.calls, mailed, mailer.count())
	}
	if count, _ := unappliedRefunds(t, h, "pp-buy-stale"); count != 1 {
		t.Fatalf("refund events after replay = %d", count)
	}
}

// BILL-8 + BILL-22: a renewal whose captured payment the product could not apply and returned
// is a finally failed renewal — the account lapses to free in the refund's transaction, both
// the refund and the renewal-failed mail go out, and no later pass renews or charges again.
func TestUnappliedRenewalIsRefundedAndLapsesTheAccount(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 31, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, mailer, clock := unappliedHarness(t, at)
	sub, err := h.service.Subscribe(ctx, "alice", plan.Basic, billing.TermMonthly)
	if err != nil {
		t.Fatal(err)
	}
	// The renewal is captured at a price its order did not quote, so it goes to review.
	*clock = sub.TermEnd
	provider.wrongAmount = true
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("renewing RunDue = %v", err)
	}
	provider.wrongAmount = false
	var order string
	if err := h.handle.Reader.QueryRow(`SELECT order_id FROM billing_intents WHERE kind='renew'`).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if got := intentStatus(t, h, order); got != "review" {
		t.Fatalf("stale renewal status = %s, want review", got)
	}
	if held, _, err := h.store.Subscription(ctx, "alice"); err != nil || held.Status != "active" || !held.TermEnd.Equal(sub.TermEnd) {
		t.Fatalf("subscription while the renewal is in review = %+v err=%v", held, err)
	}
	mailed, charges := mailer.count(), len(provider.charges)

	*clock = clock.Add(time.Hour)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("refunding RunDue = %v", err)
	}
	if got := intentStatus(t, h, order); got != "failed" {
		t.Fatalf("refunded renewal status = %s, want failed", got)
	}
	captured := providerPayment(provider, order)
	if count, krw := unappliedRefunds(t, h, order); count != 1 || krw != captured.AmountKRW || captured.BalanceKRW != 0 {
		t.Fatalf("refund events=%d krw=%d, captured=%+v", count, krw, captured)
	}
	lapsed, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || lapsed.Status != "lapsed" || !lapsed.TermEnd.Equal(sub.TermEnd) {
		t.Fatalf("subscription after the refund = %+v err=%v", lapsed, err)
	}
	var tier string
	var failures int
	if err := h.handle.Reader.QueryRow(`SELECT plan FROM users WHERE id='alice'`).Scan(&tier); err != nil || tier != "free" {
		t.Fatalf("plan after the refunded renewal = %s err=%v, want free", tier, err)
	}
	if err := h.handle.Reader.QueryRow(`SELECT count(*) FROM billing_events WHERE kind='renewal_failed'`).Scan(&failures); err != nil || failures != 1 {
		t.Fatalf("renewal_failed events=%d err=%v", failures, err)
	}
	if _, covered, err := h.service.CoverageAt(ctx, "alice", *clock); err != nil || covered {
		t.Fatalf("paid coverage after the refunded renewal = %t err=%v", covered, err)
	}
	if mailer.count() != mailed+2 {
		t.Fatalf("mails = %+v, want the refund and the renewal-failed mail", mailer.sent[mailed:])
	}
	if refund, failed := mailer.sent[mailed], mailer.sent[mailed+1]; !strings.Contains(refund.subject, "could not be applied") ||
		!strings.Contains(failed.subject, "Renewal failed") {
		t.Fatalf("mails = %+v", mailer.sent[mailed:])
	}

	// A further pass finds nothing to refund, renew or charge.
	calls, mailed := provider.calls, mailer.count()
	*clock = clock.Add(24 * time.Hour)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("further RunDue = %v", err)
	}
	if provider.calls != calls || mailer.count() != mailed || len(provider.charges) != charges {
		t.Fatalf("further pass cancelled %d→%d, mailed %d→%d, charged %d→%d",
			calls, provider.calls, mailed, mailer.count(), charges, len(provider.charges))
	}
	if count, _ := unappliedRefunds(t, h, order); count != 1 {
		t.Fatalf("refund events after a further pass = %d", count)
	}
}

// A payment captured for a different amount than its order is returned as captured: the
// refund is what the provider holds, not what the order quoted.
func TestUnappliedCaptureRefundsTheCapturedAmount(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, _, clock := unappliedHarness(t, at)
	provider.wrongAmount = true
	if _, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); !errors.Is(err, billing.ErrPaymentPending) {
		t.Fatalf("mismatched capture = %v, want ErrPaymentPending", err)
	}
	order := ""
	if err := h.handle.Reader.QueryRow(`SELECT order_id FROM billing_intents WHERE status='review'`).Scan(&order); err != nil {
		t.Fatal(err)
	}
	provider.wrongAmount = false
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatal(err)
	}
	captured := providerPayment(provider, order)
	if count, krw := unappliedRefunds(t, h, order); count != 1 || krw != captured.AmountKRW || captured.BalanceKRW != 0 {
		t.Fatalf("refund events=%d krw=%d, captured=%+v", count, krw, captured)
	}
	*clock = clock.Add(time.Minute)
	if sub, err := h.service.Subscribe(ctx, "alice", plan.Light, billing.TermMonthly); err != nil || sub.Status != "active" {
		t.Fatalf("subscribe after the refund = %+v err=%v", sub, err)
	}
}

// An unresolved cancel answer leaves the order in review — and the account's renewal skipped
// and logged — until a later pass refunds it; that pass then renews the account.
func TestUnresolvedUnappliedRefundKeepsTheOrderInReview(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))
	h, provider, _, clock := unappliedHarness(t, at)
	sub := stalePackInReview(t, h, clock, at)
	logs := captureLogs(t)

	*clock = sub.TermEnd
	provider.refusal = &billing.ProviderError{Code: "FAILED_INTERNAL_SYSTEM_PROCESSING", HTTPStatus: 500}
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("RunDue = %v", err)
	}
	if got := intentStatus(t, h, "pp-buy-stale"); got != "review" {
		t.Fatalf("order after a 500 = %s, want review", got)
	}
	if count, _ := unappliedRefunds(t, h, "pp-buy-stale"); count != 0 {
		t.Fatalf("unresolved cancel recorded %d refunds", count)
	}
	if paid := providerPayment(provider, "pp-buy-stale"); paid.BalanceKRW != 2500 {
		t.Fatalf("provider balance after a 500 = %d", paid.BalanceKRW)
	}
	if line := logLine(logs, "renewal skipped"); !strings.Contains(line, "user_id=alice") || !strings.Contains(line, "order_id=pp-buy-stale") {
		t.Fatalf("skip log = %q\nall logs:\n%s", line, logs.String())
	}
	held, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || !held.TermEnd.Equal(sub.TermEnd) {
		t.Fatalf("renewal ran while the order was in review: %+v err=%v", held, err)
	}

	provider.refusal = nil
	*clock = clock.Add(time.Hour)
	if err := h.service.RunDue(ctx, *clock); err != nil {
		t.Fatalf("settling RunDue = %v", err)
	}
	if got := intentStatus(t, h, "pp-buy-stale"); got != "failed" {
		t.Fatalf("order after the provider answered = %s, want failed", got)
	}
	if count, krw := unappliedRefunds(t, h, "pp-buy-stale"); count != 1 || krw != 2500 {
		t.Fatalf("refund events=%d krw=%d", count, krw)
	}
	for _, key := range provider.keys {
		if key != "unapplied:pp-buy-stale" {
			t.Fatalf("cancel idempotency keys = %v", provider.keys)
		}
	}
	renewed, _, err := h.store.Subscription(ctx, "alice")
	if err != nil || !renewed.TermStart.Equal(sub.TermEnd) || renewed.Status != "active" {
		t.Fatalf("renewal after the order left review = %+v err=%v", renewed, err)
	}
}

// A definitive refusal is settled by the payment read back: the money already returned
// completes the refund, anything else keeps the order in review with a log naming it.
func TestUnappliedRefundRefusalIsSettledByThePaymentReadBack(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.FixedZone("KST", 9*3600))

	t.Run("a refusal after a lost answer completes", func(t *testing.T) {
		h, provider, mailer, clock := unappliedHarness(t, at)
		stalePackInReview(t, h, clock, at)
		mailed := mailer.count()
		provider.timeout = true
		if err := h.service.RunDue(ctx, *clock); err != nil {
			t.Fatal(err)
		}
		if got := intentStatus(t, h, "pp-buy-stale"); got != "review" {
			t.Fatalf("order after a lost answer = %s, want review", got)
		}
		provider.refusal = &billing.ProviderError{Code: "ALREADY_CANCELED_PAYMENT", HTTPStatus: 400}
		*clock = clock.Add(time.Hour)
		if err := h.service.RunDue(ctx, *clock); err != nil {
			t.Fatal(err)
		}
		if got := intentStatus(t, h, "pp-buy-stale"); got != "failed" {
			t.Fatalf("order after the read-back = %s, want failed", got)
		}
		if count, krw := unappliedRefunds(t, h, "pp-buy-stale"); count != 1 || krw != 2500 {
			t.Fatalf("refund events=%d krw=%d", count, krw)
		}
		if len(provider.keys) != 2 || provider.keys[0] != provider.keys[1] {
			t.Fatalf("cancel idempotency keys = %v, want one key twice", provider.keys)
		}
		if mailer.count() != mailed+1 {
			t.Fatalf("mails = %+v", mailer.sent)
		}
	})

	t.Run("a refusal with the money still held stays in review", func(t *testing.T) {
		h, provider, mailer, clock := unappliedHarness(t, at)
		stalePackInReview(t, h, clock, at)
		mailed := mailer.count()
		logs := captureLogs(t)
		provider.refusal = &billing.ProviderError{Code: "NOT_CANCELABLE_PAYMENT", HTTPStatus: 403}
		if err := h.service.RunDue(ctx, *clock); err != nil {
			t.Fatal(err)
		}
		if got := intentStatus(t, h, "pp-buy-stale"); got != "review" {
			t.Fatalf("refused order = %s, want review", got)
		}
		if count, _ := unappliedRefunds(t, h, "pp-buy-stale"); count != 0 || mailer.count() != mailed {
			t.Fatalf("refused cancel recorded %d refunds, mails %+v", count, mailer.sent)
		}
		line := logLine(logs, "unapplied order refund refused")
		if !strings.Contains(line, "user_id=alice") || !strings.Contains(line, "order_id=pp-buy-stale") ||
			!strings.Contains(line, "code=NOT_CANCELABLE_PAYMENT") {
			t.Fatalf("refusal log = %q\nall logs:\n%s", line, logs.String())
		}
	})
}
