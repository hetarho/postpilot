package rpc

import (
	"net/http"
	"time"

	"github.com/postpilot/backend/internal/billing"
)

type WebhookHandler struct {
	provider billing.Provider
	store    billing.Store
	now      func() time.Time
}

func NewWebhookHandler(provider billing.Provider, store billing.Store) *WebhookHandler {
	return &WebhookHandler{provider: provider, store: store, now: time.Now}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.provider == nil {
		http.Error(w, "billing unavailable", http.StatusServiceUnavailable)
		return
	}
	notification, err := h.provider.ParseNotification(r)
	if err != nil {
		http.Error(w, "invalid notification", http.StatusBadRequest)
		return
	}

	// Ordinary Toss payment webhooks are unsigned. Re-read by order id and persist only the
	// provider server's answer, never payment facts supplied by this public POST.
	payment, found, err := h.provider.PaymentByOrder(r.Context(), notification.OrderID)
	if err != nil {
		http.Error(w, "provider verification failed", http.StatusBadGateway)
		return
	}
	if !found {
		http.Error(w, "payment not found", http.StatusBadGateway)
		return
	}
	if err := h.store.InsertProviderNotification(r.Context(), billing.ProviderNotification{
		Provider: "toss", EventType: notification.EventType,
		PaymentKey: payment.PaymentKey, OrderID: payment.OrderID, Status: payment.Status,
		Payload: string(notification.Raw), ReceivedAt: h.now(),
	}); err != nil {
		http.Error(w, "notification store failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
