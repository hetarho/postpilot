package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/billing"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
)

func TestPaymentMethodFailuresHaveStableCodesAndReasons(t *testing.T) {
	tests := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{billing.ErrEmailVerificationRequired, connect.CodeFailedPrecondition, "EMAIL_VERIFICATION_REQUIRED"},
		{billing.ErrCustomerKeyMismatch, connect.CodeInvalidArgument, "CUSTOMER_KEY_MISMATCH"},
		{billing.ErrUnavailable, connect.CodeFailedPrecondition, "BILLING_UNAVAILABLE"},
	}
	for _, test := range tests {
		err := registrationError("alice", test.err)
		detail := billingErrorDetail(t, err)
		if connect.CodeOf(err) != test.code || detail.GetReason() != test.reason {
			t.Errorf("%s: code=%s reason=%s", test.reason, connect.CodeOf(err), detail.GetReason())
		}
	}
	removeErr := removalError("alice", billing.ErrSubscriptionNeedsMethod)
	if connect.CodeOf(removeErr) != connect.CodeFailedPrecondition || billingErrorDetail(t, removeErr).GetReason() != "SUBSCRIPTION_NEEDS_METHOD" {
		t.Errorf("subscription removal refusal = %v", removeErr)
	}
}

func TestSubscriptionFailuresHaveStableCodesAndReasons(t *testing.T) {
	tests := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{billing.ErrTierNotSubscribable, connect.CodeInvalidArgument, "TIER_NOT_SUBSCRIBABLE"},
		{billing.ErrSubscriptionExists, connect.CodeFailedPrecondition, "SUBSCRIPTION_EXISTS"},
		{billing.ErrPaymentMethodRequired, connect.CodeFailedPrecondition, "PAYMENT_METHOD_REQUIRED"},
		{billing.ErrChargeFailed, connect.CodeFailedPrecondition, "CHARGE_FAILED"},
	}
	for _, test := range tests {
		err := subscriptionError("alice", test.err)
		if connect.CodeOf(err) != test.code || billingErrorDetail(t, err).GetReason() != test.reason {
			t.Errorf("%s = %v", test.reason, err)
		}
	}
}

func TestSubscriptionChangeFailuresHaveStableCodesAndReasons(t *testing.T) {
	tests := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{billing.ErrSubscriptionRequired, connect.CodeFailedPrecondition, "SUBSCRIPTION_REQUIRED"},
		{billing.ErrNoChange, connect.CodeFailedPrecondition, "NO_CHANGE"},
		{billing.ErrNoScheduledChange, connect.CodeFailedPrecondition, "NO_SCHEDULED_CHANGE"},
		{billing.ErrChangeUnsupported, connect.CodeInvalidArgument, "CHANGE_UNSUPPORTED"},
	}
	for _, test := range tests {
		err := changeError("alice", test.err)
		if connect.CodeOf(err) != test.code || billingErrorDetail(t, err).GetReason() != test.reason {
			t.Errorf("%s = %v", test.reason, err)
		}
	}
}

func TestPurchaseFailuresHaveStableCodesAndReasons(t *testing.T) {
	tests := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{billing.ErrPaymentMethodRequired, connect.CodeFailedPrecondition, "PAYMENT_METHOD_REQUIRED"},
		{billing.ErrChargeFailed, connect.CodeFailedPrecondition, "CHARGE_FAILED"},
	}
	for _, test := range tests {
		err := purchaseError("alice", test.err)
		if connect.CodeOf(err) != test.code || billingErrorDetail(t, err).GetReason() != test.reason {
			t.Errorf("%s = %v", test.reason, err)
		}
	}
}

func TestBillingHandlerUsesActorAndMapsTheReadContract(t *testing.T) {
	now := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
	tier, term, amount := plan.Pro, billing.TermMonthly, 9_900
	store := handlerStore{
		subscription: &billing.Subscription{UserID: "alice", Tier: tier, Term: term, AnchorAt: now, TermStart: now, TermEnd: now.AddDate(0, 1, 0), NextGrantAt: now.AddDate(0, 1, 0), AutoRenew: true, Status: "active"},
		method:       &billing.PaymentMethod{UserID: "alice", BillingKey: "must-not-cross-rpc", CustomerKey: "server-only", CardLabel: "11 1234", RegisteredAt: now},
		events:       []billing.Event{{ID: 7, UserID: "alice", Kind: "charge", KRW: &amount, CreatedAt: now}},
		purchases:    []billing.Purchase{{ID: "purchase-1", Credits: 500, KRW: 7000, ChargedAt: now, Refundable: true}},
	}
	handler := NewHandler(billing.NewService(store, nil, nil, nil, nil, nil))
	response, err := handler.GetMyBilling(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.GetMyBillingRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetSubscription().GetPlan() != postpilotv1.Plan_PLAN_PRO || response.Msg.GetPaymentMethod().GetCardLabel() != "11 1234" || len(response.Msg.GetHistory()) != 1 || response.Msg.GetCustomerKey() != billing.CustomerKey("alice") || !response.Msg.GetPurchases()[0].GetRefundable() {
		t.Fatalf("response = %+v", response.Msg)
	}
	if response.Msg.GetPaymentMethod().GetRegisteredAt() != now.Format(time.RFC3339) {
		t.Fatalf("registered_at = %q", response.Msg.GetPaymentMethod().GetRegisteredAt())
	}
	// BILL-15: history and purchases carry the whole KRW the store holds, and nothing else.
	if krw := response.Msg.GetHistory()[0].GetKrw(); krw != 9_900 {
		t.Fatalf("history krw = %d", krw)
	}
	if purchase := response.Msg.GetPurchases()[0]; purchase.GetKrw() != 7000 || purchase.GetCredits() != 500 {
		t.Fatalf("purchase = %+v", purchase)
	}
}

func TestBillingHandlerAuthenticatesAndMapsQuoteFailures(t *testing.T) {
	disabled := NewHandler(billing.NewService(handlerStore{}, nil, nil, nil, nil, nil))
	request := connect.NewRequest(&postpilotv1.QuotePriceRequest{Plan: postpilotv1.Plan_PLAN_BASIC, Term: postpilotv1.Term_TERM_MONTHLY})
	if _, err := disabled.QuotePrice(context.Background(), request); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous quote = %v", err)
	}
	ctx := auth.WithUser(context.Background(), "alice")
	if _, err := disabled.QuotePrice(ctx, request); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("disabled quote = %v", err)
	}
	invalid := connect.NewRequest(&postpilotv1.QuotePriceRequest{Plan: postpilotv1.Plan_PLAN_FREE, Term: postpilotv1.Term_TERM_MONTHLY})
	if _, err := disabled.QuotePrice(ctx, invalid); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("free quote = %v", err)
	}
}

func TestBillingHandlerReturnsTheContractQuoteWithoutDerivingItInTheTransport(t *testing.T) {
	service := billing.NewService(handlerStore{}, handlerProvider{}, nil, nil, nil, nil)
	handler := NewHandler(service)
	response, err := handler.QuotePrice(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.QuotePriceRequest{Plan: postpilotv1.Plan_PLAN_BASIC, Term: postpilotv1.Term_TERM_ANNUAL}))
	if err != nil {
		t.Fatal(err)
	}
	offer, _ := plan.CommercialOffer(plan.Basic)
	if response.Msg.GetKrw() != int64(offer.AnnualKRW) {
		t.Fatalf("quote = %+v, want the %d KRW annual offer", response.Msg, offer.AnnualKRW)
	}
}

func TestBillingHandlerMapsAnUpgradeQuote(t *testing.T) {
	anchor := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	service := billing.NewService(handlerStore{subscription: &billing.Subscription{
		UserID: "alice", Tier: plan.Basic, Term: billing.TermMonthly, AnchorAt: anchor,
		TermStart: anchor, TermEnd: time.Date(2100, 1, 8, 0, 0, 0, 0, time.UTC),
		NextGrantAt: time.Date(2100, 1, 8, 0, 0, 0, 0, time.UTC), AutoRenew: true, Status: "active",
	}}, handlerProvider{}, nil, nil, nil, nil)
	handler := NewHandler(service)
	response, err := handler.QuoteChange(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.QuoteChangeRequest{
		Plan: postpilotv1.Plan_PLAN_MAX, Term: postpilotv1.Term_TERM_MONTHLY,
	}))
	if err != nil {
		t.Fatal(err)
	}
	// The prorated amount moves with the clock; the transport carries it with the quote's id.
	basic, _ := plan.CommercialOffer(plan.Basic)
	maxOffer, _ := plan.CommercialOffer(plan.Max)
	if krw := response.Msg.GetKrw(); krw <= 0 || krw > int64(maxOffer.MonthlyKRW-basic.MonthlyKRW) ||
		!response.Msg.GetAppliedNow() || response.Msg.GetEffectiveAt() == "" || response.Msg.GetQuoteId() == "" {
		t.Fatalf("quote = %+v", response.Msg)
	}
}

type handlerStore struct {
	noJournal
	noRefunds
	subscription *billing.Subscription
	method       *billing.PaymentMethod
	events       []billing.Event
	purchases    []billing.Purchase
}

func (s handlerStore) InWriteTx(ctx context.Context, fn func(billing.Store, billing.Credits, billing.Plans) error) error {
	return fn(s, nil, handlerPlans{})
}

// handlerPlans answers every account as free: these cases are about the wire mapping, and
// BILL-20's master refusal reads the tier through the transaction's port.
type handlerPlans struct{}

func (handlerPlans) AssignTier(context.Context, string, plan.Plan) error   { return nil }
func (handlerPlans) ReassignTier(context.Context, string, plan.Plan) error { return nil }
func (handlerPlans) TierOf(context.Context, string) (plan.Plan, error)     { return plan.Free, nil }
func (s handlerStore) Subscription(context.Context, string) (billing.Subscription, bool, error) {
	if s.subscription == nil {
		return billing.Subscription{}, false, nil
	}
	return *s.subscription, true, nil
}
func (s handlerStore) PaymentMethod(context.Context, string) (billing.PaymentMethod, bool, error) {
	if s.method == nil {
		return billing.PaymentMethod{}, false, nil
	}
	return *s.method, true, nil
}
func (s handlerStore) Events(context.Context, string, int) ([]billing.Event, error) {
	return s.events, nil
}
func (s handlerStore) Purchases(context.Context, string) ([]billing.Purchase, error) {
	return s.purchases, nil
}
func (handlerStore) Purchase(context.Context, string, string) (billing.Purchase, bool, error) {
	return billing.Purchase{}, false, nil
}
func (handlerStore) InsertProviderNotification(context.Context, billing.ProviderNotification) error {
	return nil
}
func (handlerStore) UpsertPaymentMethod(context.Context, billing.PaymentMethod) error { return nil }
func (handlerStore) DeletePaymentMethod(context.Context, string) error                { return nil }
func (handlerStore) InsertEvent(context.Context, billing.Event) error                 { return nil }
func (handlerStore) InsertPurchase(context.Context, billing.Purchase) error           { return nil }
func (handlerStore) MarkPurchaseRefunded(context.Context, string, string, time.Time) (bool, error) {
	return false, nil
}
func (handlerStore) UpsertSubscription(context.Context, billing.Subscription) error {
	return nil
}
func (handlerStore) DueSubscriptions(context.Context, time.Time) ([]billing.Subscription, error) {
	return nil, nil
}
func (handlerStore) AdvanceNextGrant(context.Context, string, time.Time, time.Time) (bool, error) {
	return false, nil
}

// noJournal is a checkout journal holding no order: these cases quote, read and record
// notifications, they never settle.
type noJournal struct{}

func (noJournal) PutQuote(context.Context, billing.QuoteRecord) error { return nil }
func (noJournal) Quote(context.Context, string) (billing.QuoteRecord, bool, error) {
	return billing.QuoteRecord{}, false, nil
}
func (noJournal) PurgeExpiredQuotes(context.Context, time.Time) (int, error) { return 0, nil }
func (noJournal) InsertIntent(context.Context, billing.Intent) error         { return nil }
func (noJournal) Intent(context.Context, string) (billing.Intent, bool, error) {
	return billing.Intent{}, false, nil
}
func (noJournal) PendingIntent(context.Context, string) (billing.Intent, bool, error) {
	return billing.Intent{}, false, nil
}
func (noJournal) DueIntents(context.Context, time.Time) ([]billing.Intent, error) { return nil, nil }
func (noJournal) MarkIntent(context.Context, string, string, string, string, time.Time) (bool, error) {
	return false, nil
}
func (noJournal) ReviewIntents(context.Context, int) ([]billing.Intent, error) { return nil, nil }
func (noJournal) FailReviewIntent(context.Context, string, string, time.Time) (bool, error) {
	return false, nil
}

// noRefunds is a refund ledger holding no request: these cases never refund, and the store
// carries no refund benefits.
type noRefunds struct{}

func (noRefunds) SetIntentFunding(context.Context, string, string, time.Time) error { return nil }
func (noRefunds) RefundPayment(context.Context, string, string) (billing.RefundPayment, bool, error) {
	return billing.RefundPayment{}, false, nil
}
func (noRefunds) RefundRequest(context.Context, string) (billing.RefundRequest, bool, error) {
	return billing.RefundRequest{}, false, nil
}
func (noRefunds) OpenRefundForOrder(context.Context, string) (bool, error) { return false, nil }
func (noRefunds) Refunds(context.Context, string) ([]billing.RefundRequest, error) {
	return nil, nil
}
func (noRefunds) ProcessingRefundIDs(context.Context, time.Time, int) ([]string, error) {
	return nil, nil
}
func (noRefunds) ReviewedEvidence(context.Context, string) (billing.RefundEvidence, bool, error) {
	return billing.RefundEvidence{}, false, nil
}
func (noRefunds) InsertRefundRequest(context.Context, billing.RefundRequest) error { return nil }
func (noRefunds) RecordRefundDecision(context.Context, billing.RefundRequest, billing.RefundDecision) error {
	return nil
}
func (noRefunds) RecordRefundProviderAttempt(context.Context, string, string) error { return nil }
func (noRefunds) RecordRefundOutcome(context.Context, billing.RefundRequest, billing.Payment, time.Time) error {
	return nil
}
func (noRefunds) FailRefund(context.Context, string, string, time.Time) error { return nil }
func (noRefunds) ConfirmedRefundTotal(context.Context, string) (int, error)   { return 0, nil }
func (noRefunds) HasUnresolvedDependentUpgrade(context.Context, billing.RefundPayment) (bool, error) {
	return false, nil
}
func (noRefunds) RefundBenefits() billing.RefundBenefits { return nil }

// noCancel is a provider that never answers a cancel: these cases return no money.
type noCancel struct{}

func (noCancel) CancelPayment(context.Context, string, int, string, string) (billing.Payment, error) {
	return billing.Payment{}, errors.New("no cancel answered")
}

type handlerProvider struct{ noCancel }

func (handlerProvider) IssueBillingKey(context.Context, string, string) (billing.BillingKey, error) {
	return billing.BillingKey{}, nil
}
func (handlerProvider) Charge(context.Context, billing.ChargeRequest) (billing.Payment, error) {
	return billing.Payment{}, nil
}
func (handlerProvider) PaymentByOrder(context.Context, string) (billing.Payment, bool, error) {
	return billing.Payment{}, false, nil
}
func (handlerProvider) ParseNotification([]byte) (billing.Notification, error) {
	return billing.Notification{}, nil
}

func billingErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || len(connectErr.Details()) != 1 {
		t.Fatalf("error = %T, details unavailable", err)
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatal(valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail = %T", value)
	}
	return detail
}

func (handlerStore) TierAt(context.Context, string, string, time.Time) (plan.Plan, error) {
	return plan.Basic, nil
}
func (handlerStore) InsertTierTransition(context.Context, string, string, time.Time, plan.Plan, string) error {
	return nil
}
func (handlerStore) SupportCoverage(context.Context, string) (billing.SupportCoverage, bool, error) {
	return billing.SupportCoverage{}, false, nil
}
func (handlerStore) UpsertSupportCoverage(context.Context, billing.SupportCoverage) error { return nil }
func (handlerStore) DeleteSupportCoverage(context.Context, string) error                  { return nil }
