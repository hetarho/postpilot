package rpc

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/billing"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
)

func TestBillingHandlerUsesActorAndMapsTheReadContract(t *testing.T) {
	now := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
	tier, term, amount := plan.Pro, billing.TermMonthly, 500
	store := handlerStore{
		subscription: &billing.Subscription{UserID: "alice", Tier: tier, Term: term, AnchorAt: now, TermStart: now, TermEnd: now.AddDate(0, 1, 0), NextGrantAt: now.AddDate(0, 1, 0), AutoRenew: true, Status: "active"},
		method:       &billing.PaymentMethod{UserID: "alice", BillingKey: "must-not-cross-rpc", CustomerKey: "server-only", CardLabel: "11 1234", RegisteredAt: now},
		events:       []billing.Event{{ID: 7, UserID: "alice", Kind: "charge", USDCents: &amount, CreatedAt: now}},
	}
	handler := NewHandler(billing.NewService(store, nil, nil, nil, nil, nil, nil))
	response, err := handler.GetMyBilling(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&postpilotv1.GetMyBillingRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetSubscription().GetPlan() != postpilotv1.Plan_PLAN_PRO || response.Msg.GetPaymentMethod().GetCardLabel() != "11 1234" || len(response.Msg.GetHistory()) != 1 || response.Msg.GetCustomerKey() != billing.CustomerKey("alice") {
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

type handlerStore struct {
	subscription *billing.Subscription
	method       *billing.PaymentMethod
	events       []billing.Event
}

func (s handlerStore) InWriteTx(ctx context.Context, fn func(billing.Store) error) error {
	return fn(s)
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
func (handlerStore) Purchases(context.Context, string) ([]billing.Purchase, error) { return nil, nil }
func (handlerStore) InsertProviderNotification(context.Context, billing.ProviderNotification) error {
	return nil
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
