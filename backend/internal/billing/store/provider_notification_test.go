package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/platform/db"
)

// An ordinary Toss payment webhook carries no event id, so a redelivery used to append a
// second identical row to a table with no key at all (review/diff-260908 F6).
func TestProviderNotificationsKeepOneRowPerProviderState(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	store := billingstore.New(handle.Writer, handle.Reader)

	received := time.Now().UTC()
	notification := billing.ProviderNotification{
		Provider: "toss", EventType: "PAYMENT_STATUS_CHANGED",
		PaymentKey: "payment-1", OrderID: "buy:purchase-1", Status: "DONE",
		Payload: `{"first":true}`, ReceivedAt: received,
	}
	if err := store.InsertProviderNotification(ctx, notification); err != nil {
		t.Fatal(err)
	}
	// The same transition delivered again, with a later arrival and a different body: still
	// the same fact, and the first arrival is the one that matters.
	repeat := notification
	repeat.ReceivedAt = received.Add(time.Minute)
	repeat.Payload = `{"redelivered":true}`
	if err := store.InsertProviderNotification(ctx, repeat); err != nil {
		t.Fatalf("a redelivery must be tolerated, not rejected: %v", err)
	}

	// A genuinely new state for the same order is a new row.
	refunded := notification
	refunded.Status = "CANCELED"
	if err := store.InsertProviderNotification(ctx, refunded); err != nil {
		t.Fatal(err)
	}

	var rows int
	var payload string
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT count(*) FROM provider_notifications WHERE order_id = ?", "buy:purchase-1").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx,
		"SELECT payload FROM provider_notifications WHERE order_id = ? AND status = ?",
		"buy:purchase-1", "DONE").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if rows != 2 || payload != `{"first":true}` {
		t.Fatalf("rows = %d, kept payload = %s, want the two states with the first arrival kept", rows, payload)
	}
}
