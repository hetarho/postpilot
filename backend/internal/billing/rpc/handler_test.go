package rpc

import (
	"context"
	"errors"
	"net/http"
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
		{billing.ErrPurchaseTooSmall, connect.CodeInvalidArgument, "PURCHASE_TOO_SMALL"},
		{billing.ErrPaymentMethodRequired, connect.CodeFailedPrecondition, "PAYMENT_METHOD_REQUIRED"},
		{billing.ErrChargeFailed, connect.CodeFailedPrecondition, "CHARGE_FAILED"},
		{billing.ErrPurchaseNotFound, connect.CodeNotFound, "PURCHASE_NOT_FOUND"},
		{billing.ErrRefundWindowClosed, connect.CodeFailedPrecondition, "REFUND_WINDOW_CLOSED"},
		{billing.ErrPurchaseSpent, connect.CodeFailedPrecondition, "PURCHASE_SPENT"},
		{billing.ErrRefundFailed, connect.CodeFailedPrecondition, "REFUND_FAILED"},
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
	tier, term, amount := plan.Pro, billing.TermMonthly, 500
	store := handlerStore{
		subscription: &billing.Subscription{UserID: "alice", Tier: tier, Term: term, AnchorAt: now, TermStart: now, TermEnd: now.AddDate(0, 1, 0), NextGrantAt: now.AddDate(0, 1, 0), AutoRenew: true, Status: "active"},
		method:       &billing.PaymentMethod{UserID: "alice", BillingKey: "must-not-cross-rpc", CustomerKey: "server-only", CardLabel: "11 1234", RegisteredAt: now},
		events:       []billing.Event{{ID: 7, UserID: "alice", Kind: "charge", USDCents: &amount, CreatedAt: now}},
		purchases:    []billing.Purchase{{ID: "purchase-1", Credits: 500, USDCents: 500, KRW: 7000, ChargedAt: now, Refundable: true}},
	}
	handler := NewHandler(billing.NewService(store, nil, nil, nil, nil, nil, nil))
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
}

func TestBillingHandlerAuthenticatesAndMapsQuoteFailures(t *testing.T) {
	disabled := NewHandler(billing.NewService(handlerStore{}, nil, nil, nil, nil, nil, nil))
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
	service := billing.NewService(handlerStore{}, handlerProvider{}, handlerRates{}, nil, nil, nil, nil)
	handler := NewHandler(service)
	response, err := handler.QuotePrice(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.QuotePriceRequest{Plan: postpilotv1.Plan_PLAN_BASIC, Term: postpilotv1.Term_TERM_ANNUAL}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetUsdCents() != 2000 || response.Msg.GetKrw() != 27850 || response.Msg.GetKrwPerUsdE4() != 13925000 || response.Msg.GetRateDate() == "" {
		t.Fatalf("quote = %+v", response.Msg)
	}
}

func TestBillingHandlerMapsAnUpgradeQuote(t *testing.T) {
	anchor := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	service := billing.NewService(handlerStore{subscription: &billing.Subscription{
		UserID: "alice", Tier: plan.Basic, Term: billing.TermMonthly, AnchorAt: anchor,
		TermStart: anchor, TermEnd: time.Date(2100, 1, 8, 0, 0, 0, 0, time.UTC),
		NextGrantAt: time.Date(2100, 1, 8, 0, 0, 0, 0, time.UTC), AutoRenew: true, Status: "active",
	}}, handlerProvider{}, handlerRates{}, nil, nil, nil, nil)
	handler := NewHandler(service)
	response, err := handler.QuoteChange(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.QuoteChangeRequest{
		Plan: postpilotv1.Plan_PLAN_MAX, Term: postpilotv1.Term_TERM_MONTHLY,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetUsdCents() != 800 || response.Msg.GetKrw() != 11_140 || !response.Msg.GetAppliedNow() || response.Msg.GetEffectiveAt() == "" {
		t.Fatalf("quote = %+v", response.Msg)
	}
}

type handlerStore struct {
	subscription *billing.Subscription
	method       *billing.PaymentMethod
	events       []billing.Event
	purchases    []billing.Purchase
}

func (s handlerStore) InWriteTx(ctx context.Context, fn func(billing.Store, billing.Credits, billing.Plans) error) error {
	return fn(s, nil, nil)
}
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

type handlerProvider struct{}

func (handlerProvider) IssueBillingKey(context.Context, string, string) (billing.BillingKey, error) {
	return billing.BillingKey{}, nil
}
func (handlerProvider) Charge(context.Context, billing.ChargeRequest) (billing.Payment, error) {
	return billing.Payment{}, nil
}
func (handlerProvider) PaymentByOrder(context.Context, string) (billing.Payment, bool, error) {
	return billing.Payment{}, false, nil
}
func (handlerProvider) Refund(context.Context, string, string) error { return nil }
func (handlerProvider) ParseNotification(*http.Request) (billing.Notification, error) {
	return billing.Notification{}, nil
}

type handlerRates struct{}

func (handlerRates) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 13_925_000, true, nil
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
