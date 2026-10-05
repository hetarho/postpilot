package billing

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// subscribedStore holds alice on an active paid subscription, which a pack needs (BILL-9).
func subscribedStore(now time.Time) *subscriptionStore {
	store := newSubscriptionStore()
	store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermMonthly, now, now.AddDate(0, 1, 0), now.AddDate(0, 1, 0), true)
	return store
}

func TestPurchasePackRequiresMethodAndWritesNoLotOnChargeFailure(t *testing.T) {
	ctx := context.Background()
	store := subscribedStore(time.Date(2026, 9, 8, 12, 0, 0, 0, seoul))
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, time.Date(2026, 9, 8, 12, 0, 0, 0, seoul))
	delete(store.methods, "alice")
	if _, err := service.PurchasePack(ctx, "alice", "pack-1000"); !errors.Is(err, ErrPaymentMethodRequired) {
		t.Fatalf("missing method = %v", err)
	}
	store.methods["alice"] = testMethod("alice")
	provider.chargeErr = declined
	if _, err := service.PurchasePack(ctx, "alice", "pack-1000"); !errors.Is(err, ErrChargeFailed) {
		t.Fatalf("declined charge = %v", err)
	}
	if len(store.credits.lots) != 0 || len(store.purchases) != 0 || kinds(store.events) != "charge_failed" {
		t.Fatalf("lots=%+v purchases=%+v events=%+v", store.credits.lots, store.purchases, store.events)
	}
}

// BILL-9: a pack changes no tier, anchor, renewal or subscription row.
func TestPurchasePackChangesNoSubscription(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)
	store := subscribedStore(now)
	store.plans.tiers["alice"] = plan.Pro
	before := store.subscriptions["alice"]
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)
	service.newID = func() string { return "purchase-1" }

	purchase, err := service.PurchasePack(context.Background(), "alice", "pack-1000")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.subscriptions["alice"], before) || store.plans.tiers["alice"] != plan.Pro {
		t.Fatalf("before=%+v after=%+v tier=%s", before, store.subscriptions["alice"], store.plans.tiers["alice"])
	}
	if purchase.ID != "pp-buy-purchase-1" || purchase.OrderID != "pp-buy-purchase-1" || purchase.PackID != "pack-1000" || purchase.Credits != 1000 || purchase.KRW != 3000 {
		t.Fatalf("purchase = %+v", purchase)
	}
	if len(provider.requests) != 1 || provider.requests[0].OrderID != "pp-buy-purchase-1" || provider.requests[0].KRW != 3000 || provider.requests[0].Name != "Postpilot credit pack pack-1000" {
		t.Fatalf("charge requests = %+v", provider.requests)
	}
	if kinds(store.events) != "charge" || len(store.purchases) != 1 || store.credits.lots[purchase.LotID].remaining != 1000 {
		t.Fatalf("events=%+v purchases=%+v lots=%+v", store.events, store.purchases, store.credits.lots)
	}
	if len(store.mailer.messages) != 1 || !strings.Contains(store.mailer.messages[0].text, "1000 credits") {
		t.Fatalf("mail = %+v", store.mailer.messages)
	}
}

func TestGetMyBillingComputesRefundableFromWindowAndLot(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)
	store, _, service, purchase := purchasedFixture(t, now)
	view, err := service.GetMyBilling(context.Background(), "alice")
	if err != nil || len(view.Purchases) != 1 || !view.Purchases[0].Refundable {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	store.credits.lots[purchase.LotID].remaining--
	view, err = service.GetMyBilling(context.Background(), "alice")
	if err != nil || view.Purchases[0].Refundable {
		t.Fatalf("spent view=%+v err=%v", view, err)
	}
}

// A screenful of purchases is one refundability read, not one per row: the per-purchase
// answer used to come off the single writer, so a read-only screen queued N statements ahead
// of every concurrent hold (review/diff-260908 F4).
func TestGetMyBillingResolvesEveryPurchaseInOneRead(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)
	store := subscribedStore(now)
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)

	ids := []string{"whole", "spent", "refunded", "expired"}
	for _, id := range ids {
		service.newID = func() string { return id }
		if _, err := service.PurchasePack(ctx, "alice", "pack-1000"); err != nil {
			t.Fatal(err)
		}
	}
	store.credits.lots[store.purchases["pp-buy-spent"].LotID].remaining--
	if marked, err := store.MarkPurchaseRefunded(ctx, "alice", "pp-buy-refunded", now); err != nil || !marked {
		t.Fatalf("mark refunded = %t, %v", marked, err)
	}
	expired := store.purchases["pp-buy-expired"]
	expired.ChargedAt = now.Add(-refundWindow)
	store.purchases["pp-buy-expired"] = expired
	store.credits.untouchedReads = 0

	view, err := service.GetMyBilling(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	refundable := map[string]bool{}
	for _, purchase := range view.Purchases {
		refundable[purchase.ID] = purchase.Refundable
	}
	if !refundable["pp-buy-whole"] || refundable["pp-buy-spent"] || refundable["pp-buy-refunded"] || refundable["pp-buy-expired"] {
		t.Fatalf("refundability = %+v", refundable)
	}
	if store.credits.untouchedReads != 1 {
		t.Fatalf("refundability reads = %d, want exactly one for four purchases", store.credits.untouchedReads)
	}

	// Nothing in window, nothing to ask: neither an empty purchase list nor an all-stale one
	// issues a query.
	store.credits.untouchedReads = 0
	store.purchases = map[string]Purchase{}
	if _, err := service.GetMyBilling(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if store.credits.untouchedReads != 0 {
		t.Fatalf("refundability reads with no purchases = %d, want none", store.credits.untouchedReads)
	}
}

func purchasedFixture(t *testing.T, now time.Time) (*subscriptionStore, *subscriptionProvider, *Service, Purchase) {
	t.Helper()
	store := subscribedStore(now)
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)
	service.newID = func() string { return "purchase-1" }
	purchase, err := service.PurchasePack(context.Background(), "alice", "pack-1000")
	if err != nil {
		t.Fatal(err)
	}
	return store, provider, service, purchase
}
