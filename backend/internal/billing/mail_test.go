package billing

import (
	"context"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/plan"
)

func TestBillingMailsAreKoreanFirstAndCarryChargeFacts(t *testing.T) {
	quote := Quote{USDCents: 500, KRW: 6_962, RatePerUSDE4: 13_925_000, RateDate: "2026-09-07"}
	for name, message := range map[string]MailMessage{
		"renewed": RenewalMail(plan.Pro, TermMonthly, quote),
		"failed":  RenewalFailedMail(plan.Pro, TermMonthly, quote),
	} {
		if !strings.HasPrefix(message.Text, "Postpilot pro 구독") {
			t.Errorf("%s is not Korean first: %q", name, message.Text)
		}
		for _, fact := range []string{"pro monthly", "$5.00", "6,962원", "1,392.50원/$"} {
			if !strings.Contains(message.Text, fact) {
				t.Errorf("%s missing %q: %s", name, fact, message.Text)
			}
		}
	}
	cancelled := CancellationMail(plan.Pro, TermAnnual)
	if !strings.HasPrefix(cancelled.Text, "Postpilot pro annual 구독") || !strings.Contains(cancelled.Text, "scheduled cancellation") {
		t.Fatalf("cancellation mail = %q", cancelled.Text)
	}
}

func TestPurchaseAndRefundMailsCarryBilingualPurchaseFacts(t *testing.T) {
	purchase := Purchase{Credits: 500, USDCents: 500, KRW: 6_963, RatePerUSDE4: 13_925_000, RateDate: "2026-09-07"}
	for name, message := range map[string]MailMessage{"purchase": PurchaseMail(purchase), "refund": RefundMail(purchase)} {
		if !strings.HasPrefix(message.Text, "Postpilot 크레딧 500개") {
			t.Errorf("%s is not Korean first: %q", name, message.Text)
		}
		for _, fact := range []string{"500 credits", "$5.00", "6,963원", "1,392.50원/$", "2026-09-07"} {
			if !strings.Contains(message.Text, fact) {
				t.Errorf("%s missing %q: %s", name, fact, message.Text)
			}
		}
	}
}

func TestBillingMailIsSkippedWithoutAVerifiedAddress(t *testing.T) {
	store := newSubscriptionStore()
	service := NewService(store, newSubscriptionProvider(), subscriptionRates{}, store.credits, store.plans, subscriptionAccounts{}, store.mailer)
	if err := service.sendMail(context.Background(), "alice", CancellationMail(plan.Pro, TermMonthly)); err != nil {
		t.Fatal(err)
	}
	if len(store.mailer.messages) != 0 {
		t.Fatalf("mail messages = %+v", store.mailer.messages)
	}
}
