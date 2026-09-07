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

func TestCreditsPerUSDCentIsAtPar(t *testing.T) {
	if plan.CreditsPerUSDCent != 1 || 100*plan.CreditsPerUSDCent != 100 {
		t.Fatalf("credits per USD cent = %d", plan.CreditsPerUSDCent)
	}
}

func TestQuotePurchaseUsesWholeCentParAndDailyRate(t *testing.T) {
	service := newSubscriptionService(newSubscriptionStore(), newSubscriptionProvider(), time.Date(2026, 9, 8, 12, 0, 0, 0, seoul))
	if _, err := service.QuotePurchase(context.Background(), 99); !errors.Is(err, ErrPurchaseTooSmall) {
		t.Fatalf("small quote = %v", err)
	}
	quote, err := service.QuotePurchase(context.Background(), 500)
	if err != nil {
		t.Fatal(err)
	}
	if quote.Credits != 500 || quote.KRW != 6_963 || quote.RatePerUSDE4 != 13_925_000 || quote.RateDate != "2026-09-07" {
		t.Fatalf("quote = %+v", quote)
	}
}

func TestPurchaseCreditsRequiresMethodAndWritesNoLotOnChargeFailure(t *testing.T) {
	store := newSubscriptionStore()
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, time.Date(2026, 9, 8, 12, 0, 0, 0, seoul))
	delete(store.methods, "alice")
	if _, err := service.PurchaseCredits(context.Background(), "alice", 500); !errors.Is(err, ErrPaymentMethodRequired) {
		t.Fatalf("missing method = %v", err)
	}
	store.methods["alice"] = testMethod("alice")
	provider.chargeErr = errors.New("declined")
	if _, err := service.PurchaseCredits(context.Background(), "alice", 500); !errors.Is(err, ErrChargeFailed) {
		t.Fatalf("declined charge = %v", err)
	}
	if len(store.credits.lots) != 0 || len(store.purchases) != 0 || kinds(store.events) != "charge_failed" {
		t.Fatalf("lots=%+v purchases=%+v events=%+v", store.credits.lots, store.purchases, store.events)
	}
}

func TestPurchaseCreditsDoesNotReadOrChangeSubscription(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermMonthly, now, now.AddDate(0, 1, 0), now.AddDate(0, 1, 0), true)
	before := store.subscriptions["alice"]
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)
	service.newID = func() string { return "purchase-1" }

	purchase, err := service.PurchaseCredits(context.Background(), "alice", 500)
	if err != nil {
		t.Fatal(err)
	}
	if store.subscriptionReads != 0 || !reflect.DeepEqual(store.subscriptions["alice"], before) {
		t.Fatalf("subscription reads=%d before=%+v after=%+v", store.subscriptionReads, before, store.subscriptions["alice"])
	}
	if purchase.ID != "purchase-1" || purchase.OrderID != "buy:purchase-1" || purchase.Credits != 500 || purchase.KRW != 6_963 || !purchase.Refundable {
		t.Fatalf("purchase = %+v", purchase)
	}
	if len(provider.requests) != 1 || provider.requests[0].OrderID != "buy:purchase-1" || provider.requests[0].Name != "Postpilot 500 credits" {
		t.Fatalf("charge requests = %+v", provider.requests)
	}
	if kinds(store.events) != "charge" || len(store.purchases) != 1 || store.credits.lots[purchase.LotID].remaining != 500 {
		t.Fatalf("events=%+v purchases=%+v lots=%+v", store.events, store.purchases, store.credits.lots)
	}
	if len(store.mailer.messages) != 1 || !strings.Contains(store.mailer.messages[0].text, "500 credits") {
		t.Fatalf("mail = %+v", store.mailer.messages)
	}
}

func TestRefundPurchaseEnforcesWindowAndUntouchedLot(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)

	t.Run("strict seven-day boundary", func(t *testing.T) {
		store, provider, service, purchase := purchasedFixture(t, now)
		service.now = func() time.Time { return now.Add(refundWindow) }
		if _, err := service.RefundPurchase(ctx, "alice", purchase.ID); !errors.Is(err, ErrRefundWindowClosed) {
			t.Fatalf("boundary = %v", err)
		}
		if len(provider.refunds) != 0 || store.credits.lots[purchase.LotID].remaining != purchase.Credits {
			t.Fatalf("refunds=%+v lot=%+v", provider.refunds, store.credits.lots[purchase.LotID])
		}
	})

	t.Run("one spent credit", func(t *testing.T) {
		store, provider, service, purchase := purchasedFixture(t, now)
		store.credits.lots[purchase.LotID].remaining--
		if _, err := service.RefundPurchase(ctx, "alice", purchase.ID); !errors.Is(err, ErrPurchaseSpent) {
			t.Fatalf("spent = %v", err)
		}
		if len(provider.refunds) != 0 {
			t.Fatalf("provider refunds = %+v", provider.refunds)
		}
	})

	t.Run("provider failure restores lot", func(t *testing.T) {
		store, provider, service, purchase := purchasedFixture(t, now)
		provider.refundErr = errors.New("provider unavailable")
		if _, err := service.RefundPurchase(ctx, "alice", purchase.ID); !errors.Is(err, ErrRefundFailed) {
			t.Fatalf("provider failure = %v", err)
		}
		if store.credits.lots[purchase.LotID].remaining != purchase.Credits || store.purchases[purchase.ID].RefundedAt != nil {
			t.Fatalf("lot=%+v purchase=%+v", store.credits.lots[purchase.LotID], store.purchases[purchase.ID])
		}
	})

	t.Run("success preserves original charge facts", func(t *testing.T) {
		store, provider, service, purchase := purchasedFixture(t, now)
		service.now = func() time.Time { return now.Add(refundWindow - time.Second) }
		refunded, err := service.RefundPurchase(ctx, "alice", purchase.ID)
		if err != nil {
			t.Fatal(err)
		}
		if refunded.RefundedAt == nil || refunded.Refundable || store.credits.lots[purchase.LotID].remaining != 0 || len(provider.refunds) != 1 || provider.refunds[0] != purchase.ProviderPaymentKey+":purchase refund" {
			t.Fatalf("refunded=%+v lot=%+v provider=%+v", refunded, store.credits.lots[purchase.LotID], provider.refunds)
		}
		refund := store.events[len(store.events)-1]
		if refund.Kind != "refund" || *refund.USDCents != purchase.USDCents || *refund.KRW != purchase.KRW || *refund.KRWPerUSDE4 != purchase.RatePerUSDE4 || *refund.RateDate != purchase.RateDate {
			t.Fatalf("refund event = %+v", refund)
		}
	})
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

func purchasedFixture(t *testing.T, now time.Time) (*subscriptionStore, *subscriptionProvider, *Service, Purchase) {
	t.Helper()
	store := newSubscriptionStore()
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)
	service.newID = func() string { return "purchase-1" }
	purchase, err := service.PurchaseCredits(context.Background(), "alice", 500)
	if err != nil {
		t.Fatal(err)
	}
	return store, provider, service, purchase
}
