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
)

// BILL-11: a reviewed refund the provider refused is named to the operator, also when a store
// failure followed it; every other refund refusal keeps its Connect code and stays untyped.
func TestRefundFailuresHaveStableCodesAndReasons(t *testing.T) {
	refusal := &billing.ProviderError{Code: "NOT_CANCELABLE_PAYMENT", HTTPStatus: 403}
	tests := []struct {
		name   string
		err    error
		code   connect.Code
		reason string
	}{
		{"provider refusal", errors.Join(billing.ErrRefundFailed, refusal), connect.CodeFailedPrecondition, "REFUND_FAILED"},
		{"refusal then a lost race", errors.Join(billing.ErrRefundFailed, refusal, billing.ErrRefundConflict), connect.CodeFailedPrecondition, "REFUND_FAILED"},
		{"conflict", billing.ErrRefundConflict, connect.CodeFailedPrecondition, "UNKNOWN_FAILURE"},
		{"not found", billing.ErrRefundNotFound, connect.CodeNotFound, "UNKNOWN_FAILURE"},
		{"amount", billing.ErrRefundAmount, connect.CodeInvalidArgument, "UNKNOWN_FAILURE"},
		{"provider pending", billing.ErrRefundProviderPending, connect.CodeUnavailable, "UNKNOWN_FAILURE"},
		{"billing disabled", billing.ErrUnavailable, connect.CodeFailedPrecondition, "UNKNOWN_FAILURE"},
	}
	for _, test := range tests {
		err := refundError(test.err)
		detail := billingErrorDetail(t, err)
		// The FE's REFUND_FAILED spec allows no params; one would turn it into UNKNOWN_FAILURE.
		if connect.CodeOf(err) != test.code || detail.GetReason() != test.reason || len(detail.GetParams()) != 0 {
			t.Errorf("%s: code=%s reason=%s params=%v", test.name, connect.CodeOf(err), detail.GetReason(), detail.GetParams())
		}
	}
}

// The operator's re-check of an approved refund whose cancel the provider refuses, the payment
// untouched, fails the request and answers REFUND_FAILED rather than the generic failure.
func TestReconcileRefundNamesTheProviderRefusal(t *testing.T) {
	failed := ""
	handler := NewHandler(billing.NewService(refusedRefundStore{failed: &failed}, refusingProvider{}, nil, nil, nil, nil))
	_, err := handler.ReconcileRefund(auth.WithUser(context.Background(), "operator"),
		connect.NewRequest(&postpilotv1.ReconcileRefundRequest{RequestId: "refund-1"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || billingErrorDetail(t, err).GetReason() != "REFUND_FAILED" {
		t.Fatalf("reconcile = %v", err)
	}
	if failed != "NOT_CANCELABLE_PAYMENT" {
		t.Fatalf("request failed with provider status %q, want the refusal's code", failed)
	}
}

// refusedRefundStore holds one approved pack refund, still processing.
type refusedRefundStore struct {
	handlerStore
	failed *string
}

func (s refusedRefundStore) InWriteTx(ctx context.Context, fn func(billing.Store, billing.Credits, billing.Plans) error) error {
	return fn(s, nil, handlerPlans{})
}
func (refusedRefundStore) RefundRequest(context.Context, string) (billing.RefundRequest, bool, error) {
	return billing.RefundRequest{ID: "refund-1", UserID: "alice", OrderID: "order-1", Status: "processing",
		ReviewedAmountKRW: 4900, ProviderBalanceBeforeKRW: 4900, IdempotencyKey: "refund:refund-1"}, true, nil
}
func (refusedRefundStore) RefundPayment(context.Context, string, string) (billing.RefundPayment, bool, error) {
	return billing.RefundPayment{OrderID: "order-1", UserID: "alice", Kind: "pack", PaymentKey: "pay-1", KRW: 4900}, true, nil
}
func (s refusedRefundStore) FailRefund(_ context.Context, _ string, providerStatus string, _ time.Time) error {
	*s.failed = providerStatus
	return nil
}

// refusingProvider refuses every cancel for good and reads the payment back whole.
type refusingProvider struct{ handlerProvider }

func (refusingProvider) CancelPayment(context.Context, string, int, string, string) (billing.Payment, error) {
	return billing.Payment{}, &billing.ProviderError{Code: "NOT_CANCELABLE_PAYMENT", HTTPStatus: 403}
}
func (refusingProvider) PaymentByOrder(context.Context, string) (billing.Payment, bool, error) {
	return billing.Payment{PaymentKey: "pay-1", OrderID: "order-1", Status: "DONE",
		AmountKRW: 4900, BalanceKRW: 4900, Currency: "KRW"}, true, nil
}
