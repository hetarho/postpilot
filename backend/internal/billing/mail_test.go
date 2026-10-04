package billing

import (
	"context"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/plan"
)

func TestBillingMailsAreKoreanFirstAndCarryChargeFacts(t *testing.T) {
	quote := Quote{KRW: 6_962}
	for name, message := range map[string]MailMessage{
		"renewed": RenewalMail(plan.Pro, TermMonthly, quote),
		"failed":  RenewalFailedMail(plan.Pro, TermMonthly, quote),
	} {
		if !strings.HasPrefix(message.Text, "Postpilot pro 구독") {
			t.Errorf("%s is not Korean first: %q", name, message.Text)
		}
		for _, fact := range []string{"pro monthly", "6,962원"} {
			if !strings.Contains(message.Text, fact) {
				t.Errorf("%s missing %q: %s", name, fact, message.Text)
			}
		}
		// QUOTA-65: a mail never states a dollar amount or an exchange rate.
		for _, leak := range []string{"$", "원/$", "1,392"} {
			if strings.Contains(message.Text, leak) {
				t.Errorf("%s carries %q: %s", name, leak, message.Text)
			}
		}
	}
	cancelled := CancellationMail(plan.Pro, TermAnnual)
	if !strings.HasPrefix(cancelled.Text, "Postpilot pro annual 구독") || !strings.Contains(cancelled.Text, "scheduled cancellation") {
		t.Fatalf("cancellation mail = %q", cancelled.Text)
	}
}

func TestPurchaseMailCarriesBilingualPurchaseFacts(t *testing.T) {
	message := PurchaseMail(Purchase{Credits: 500, KRW: 6_963})
	if !strings.HasPrefix(message.Text, "Postpilot 크레딧 500개") {
		t.Errorf("purchase mail is not Korean first: %q", message.Text)
	}
	for _, fact := range []string{"500 credits", "6,963원"} {
		if !strings.Contains(message.Text, fact) {
			t.Errorf("purchase mail missing %q: %s", fact, message.Text)
		}
	}
	for _, leak := range []string{"$", "원/$"} {
		if strings.Contains(message.Text, leak) {
			t.Errorf("purchase mail carries %q: %s", leak, message.Text)
		}
	}
}

func TestBillingMailIsSkippedWithoutAVerifiedAddress(t *testing.T) {
	store := newSubscriptionStore()
	service := NewService(store, newSubscriptionProvider(), store.credits, store.plans, subscriptionAccounts{}, store.mailer)
	if err := service.sendMail(context.Background(), "alice", CancellationMail(plan.Pro, TermMonthly)); err != nil {
		t.Fatal(err)
	}
	if len(store.mailer.messages) != 0 {
		t.Fatalf("mail messages = %+v", store.mailer.messages)
	}
}
