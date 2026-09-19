package rpc

import (
	"io"
	"net/http"
	"time"

	"github.com/postpilot/backend/internal/billing"
)

// The most of a notification body this edge will read. Toss's envelopes are a few hundred
// bytes; the cap is what keeps a public POST from being a memory bill.
const maxNotificationBytes = 1 << 20

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
	// The transport stops here: the provider port is handed the body it has to read, never
	// the request it arrived in (ARCH-7). The cap is the adapter's too — a domain port
	// cannot decide how much of a socket to trust.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxNotificationBytes))
	if err != nil {
		http.Error(w, "invalid notification", http.StatusBadRequest)
		return
	}
	notification, err := h.provider.ParseNotification(body)
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
