package tosspay

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/billing"
)

func TestTossProviderEndpointsBodiesAndErrorMapping(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.Path+" "+string(body))
		if got := r.Header.Get("Authorization"); got != "Basic "+base64.StdEncoding.EncodeToString([]byte("test_sk:")) {
			t.Errorf("auth=%q", got)
		}
		switch r.URL.Path {
		case "/v1/billing/authorizations/issue":
			_, _ = io.WriteString(w, `{"billingKey":"billing-1","customerKey":"customer-1","card":{"issuerCode":"11","number":"433012******1234"}}`)
		case "/v1/billing/billing-1":
			_, _ = io.WriteString(w, `{"paymentKey":"pay-1","orderId":"order-1","status":"DONE"}`)
		case "/v1/payments/orders/order-1":
			_, _ = io.WriteString(w, `{"paymentKey":"pay-1","orderId":"order-1","status":"DONE"}`)
		case "/v1/payments/pay-1/cancel":
			_, _ = io.WriteString(w, `{}`)
		case "/v1/payments/orders/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"code":"NOT_FOUND_PAYMENT","message":"missing"}`)
		case "/v1/payments/orders/broken":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"code":"INVALID_REQUEST","message":"bad order"}`)
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := New("test_sk", server.Client())
	client.endpoint = server.URL
	key, err := client.IssueBillingKey(context.Background(), "auth-1", "customer-1")
	if err != nil || key.Value != "billing-1" || key.CardLabel != "11 1234" {
		t.Fatalf("key=%+v err=%v", key, err)
	}
	payment, err := client.Charge(context.Background(), billing.ChargeRequest{BillingKey: "billing-1", CustomerKey: "customer-1", OrderID: "order-1", KRW: 2785, Name: "Basic monthly"})
	if err != nil || payment.Status != "DONE" {
		t.Fatalf("payment=%+v err=%v", payment, err)
	}
	if _, found, err := client.PaymentByOrder(context.Background(), "order-1"); err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := client.Refund(context.Background(), "pay-1", "unused purchase"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := client.PaymentByOrder(context.Background(), "missing"); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	_, _, err = client.PaymentByOrder(context.Background(), "broken")
	var providerErr *billing.ProviderError
	if !errors.As(err, &providerErr) || providerErr.Code != "INVALID_REQUEST" {
		t.Fatalf("err=%#v", err)
	}

	joined := strings.Join(requests, "\n")
	for _, want := range []string{
		`POST /v1/billing/authorizations/issue {"authKey":"auth-1","customerKey":"customer-1"}`,
		`POST /v1/billing/billing-1 {"amount":2785,"customerKey":"customer-1","orderId":"order-1","orderName":"Basic monthly"}`,
		`POST /v1/payments/pay-1/cancel {"cancelReason":"unused purchase"}`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing request %s\n%s", want, joined)
		}
	}
}

func TestParseNotificationSupportsCurrentDataEnvelope(t *testing.T) {
	client := New("test", nil)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/toss", strings.NewReader(`{"eventType":"PAYMENT_STATUS_CHANGED","data":{"paymentKey":"pay-1","orderId":"order-1","status":"DONE"}}`))
	n, err := client.ParseNotification(req)
	if err != nil || n.EventType != "PAYMENT_STATUS_CHANGED" || n.PaymentKey != "pay-1" || n.OrderID != "order-1" || n.Status != "DONE" || len(n.Raw) == 0 {
		t.Fatalf("notification=%+v err=%v", n, err)
	}
}
