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
		// BILL-15: the refund records the KRW the purchase charged and no exchange rate.
		refund := store.events[len(store.events)-1]
		if refund.Kind != "refund" || *refund.KRW != purchase.KRW || *refund.Credits != purchase.Credits || refund.USDCents != nil || refund.KRWPerUSDE4 != nil || refund.RateDate != nil {
			t.Fatalf("refund event = %+v", refund)
		}
	})
}

// A refund is three steps against two systems — void the lot, refund the card, mark the row
// — and a failure at the last one used to leave the money back, the credits gone and the
// purchase reported as PURCHASE_SPENT forever (review/diff-260908 F3).
func TestRefundPurchaseResumesAfterAFailedMark(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)

	cases := []struct {
		name string
		// arrange runs on a fresh fixture, before the refund under test.
		arrange func(store *subscriptionStore, provider *subscriptionProvider, service *Service, purchase Purchase)
		wantErr error
		// wantRefunds is how many times the provider was asked to refund, counting any call
		// arrange made.
		wantRefunds int
		wantMarked  bool
	}{
		{
			name:        "happy path refunds once and marks the row",
			arrange:     func(*subscriptionStore, *subscriptionProvider, *Service, Purchase) {},
			wantRefunds: 1,
			wantMarked:  true,
		},
		{
			name: "a mark failure is finished by the retry",
			arrange: func(store *subscriptionStore, _ *subscriptionProvider, service *Service, purchase Purchase) {
				store.markRefundedErr = errors.New("write failed")
				if _, err := service.RefundPurchase(context.Background(), "alice", purchase.ID); err == nil {
					t.Fatal("first attempt should have failed at the mark")
				}
				store.markRefundedErr = nil
			},
			wantRefunds: 1,
			wantMarked:  true,
		},
		{
			name: "a live charge behind a touched lot is a spent purchase",
			arrange: func(store *subscriptionStore, _ *subscriptionProvider, _ *Service, purchase Purchase) {
				store.credits.lots[purchase.LotID].remaining--
			},
			wantErr:     ErrPurchaseSpent,
			wantRefunds: 0,
		},
		{
			name: "a voided lot with no provider record is not evidence of a refund",
			arrange: func(store *subscriptionStore, provider *subscriptionProvider, _ *Service, purchase Purchase) {
				store.credits.lots[purchase.LotID].remaining = 0
				delete(provider.payments, purchase.OrderID)
			},
			wantErr:     ErrPurchaseSpent,
			wantRefunds: 0,
		},
		{
			name: "a purchase already marked refunded is not refundable again",
			arrange: func(store *subscriptionStore, _ *subscriptionProvider, service *Service, purchase Purchase) {
				if _, err := service.RefundPurchase(context.Background(), "alice", purchase.ID); err != nil {
					t.Fatal(err)
				}
			},
			wantErr:     ErrPurchaseNotFound,
			wantRefunds: 1,
			wantMarked:  true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			store, provider, service, purchase := purchasedFixture(t, now)
			testCase.arrange(store, provider, service, purchase)

			refunded, err := service.RefundPurchase(ctx, "alice", purchase.ID)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("refund err = %v, want %v", err, testCase.wantErr)
			}
			if len(provider.refunds) != testCase.wantRefunds {
				t.Fatalf("provider refunds = %+v, want %d", provider.refunds, testCase.wantRefunds)
			}
			stored := store.purchases[purchase.ID]
			if (stored.RefundedAt != nil) != testCase.wantMarked {
				t.Fatalf("stored purchase = %+v, want marked=%v", stored, testCase.wantMarked)
			}
			if testCase.wantErr != nil {
				return
			}
			if refunded.RefundedAt == nil || refunded.Refundable {
				t.Fatalf("returned purchase = %+v", refunded)
			}
			if store.credits.lots[purchase.LotID].remaining != 0 {
				t.Fatalf("lot = %+v, want voided", store.credits.lots[purchase.LotID])
			}
			var refunds int
			for _, event := range store.events {
				if event.Kind == "refund" {
					refunds++
				}
			}
			if refunds != 1 {
				t.Fatalf("refund ledger rows = %d, want exactly one", refunds)
			}
			view, err := service.GetMyBilling(ctx, "alice")
			if err != nil || len(view.Purchases) != 1 || view.Purchases[0].RefundedAt == nil || view.Purchases[0].Refundable {
				t.Fatalf("billing view = %+v, err=%v", view.Purchases, err)
			}
		})
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
	if _, err := service.RefundPurchase(ctx, "alice", "pp-buy-refunded"); err != nil {
		t.Fatal(err)
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
