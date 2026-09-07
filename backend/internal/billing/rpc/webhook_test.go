package rpc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
)

func TestWebhookReReadsAndStoresProviderPayment(t *testing.T) {
	store := &webhookStore{}
	handler := NewWebhookHandler(webhookProvider{}, store)
	handler.now = func() time.Time { return time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC) }
	req := httptest.NewRequest(http.MethodPost, "/webhooks/toss", strings.NewReader(`{"eventType":"PAYMENT_STATUS_CHANGED"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK || len(store.notifications) != 1 {
		t.Fatalf("status=%d rows=%+v", response.Code, store.notifications)
	}
	row := store.notifications[0]
	if row.PaymentKey != "verified-payment" || row.OrderID != "verified-order" || row.Status != "DONE" {
		t.Fatalf("stored unverified values: %+v", row)
	}
}

func TestWebhookRejectsUnparseableBody(t *testing.T) {
	handler := NewWebhookHandler(webhookProvider{parseErr: errors.New("bad")}, &webhookStore{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/webhooks/toss", strings.NewReader(`bad`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", response.Code)
	}
}

type webhookProvider struct{ parseErr error }

func (p webhookProvider) IssueBillingKey(context.Context, string, string) (billing.BillingKey, error) {
	return billing.BillingKey{}, nil
}
func (p webhookProvider) Charge(context.Context, billing.ChargeRequest) (billing.Payment, error) {
	return billing.Payment{}, nil
}
func (p webhookProvider) PaymentByOrder(context.Context, string) (billing.Payment, bool, error) {
	return billing.Payment{PaymentKey: "verified-payment", OrderID: "verified-order", Status: "DONE"}, true, nil
}
func (p webhookProvider) Refund(context.Context, string, string) error { return nil }
func (p webhookProvider) ParseNotification(*http.Request) (billing.Notification, error) {
	return billing.Notification{EventType: "PAYMENT_STATUS_CHANGED", OrderID: "untrusted", Raw: []byte(`raw`)}, p.parseErr
}

type webhookStore struct {
	notifications []billing.ProviderNotification
}

func (s *webhookStore) InWriteTx(ctx context.Context, fn func(billing.Store) error) error {
	return fn(s)
}
func (s *webhookStore) Subscription(context.Context, string) (billing.Subscription, bool, error) {
	return billing.Subscription{}, false, nil
}
func (s *webhookStore) PaymentMethod(context.Context, string) (billing.PaymentMethod, bool, error) {
	return billing.PaymentMethod{}, false, nil
}
func (s *webhookStore) Events(context.Context, string, int) ([]billing.Event, error) { return nil, nil }
func (s *webhookStore) Purchases(context.Context, string) ([]billing.Purchase, error) {
	return nil, nil
}
func (s *webhookStore) InsertProviderNotification(_ context.Context, notification billing.ProviderNotification) error {
	s.notifications = append(s.notifications, notification)
	return nil
}
