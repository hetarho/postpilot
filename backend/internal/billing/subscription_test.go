package billing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

func TestSubscribeRefusalsAndProviderFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)

	if _, err := service.Subscribe(ctx, "alice", plan.Free, TermMonthly); !errors.Is(err, ErrTierNotSubscribable) {
		t.Fatalf("free refusal = %v", err)
	}
	if _, err := service.Subscribe(ctx, "alice", plan.Master, TermMonthly); !errors.Is(err, ErrTierNotSubscribable) {
		t.Fatalf("master refusal = %v", err)
	}
	store.subscriptions["alice"] = Subscription{UserID: "alice", Tier: plan.Basic, Status: "active"}
	if _, err := service.Subscribe(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrSubscriptionExists) {
		t.Fatalf("existing refusal = %v", err)
	}
	delete(store.subscriptions, "alice")
	delete(store.methods, "alice")
	if _, err := service.Subscribe(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrPaymentMethodRequired) {
		t.Fatalf("method refusal = %v", err)
	}
	store.methods["alice"] = testMethod("alice")
	provider.chargeErr = errors.New("declined")
	if _, err := service.Subscribe(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrChargeFailed) {
		t.Fatalf("charge refusal = %v", err)
	}
	if len(store.subscriptions) != 0 || len(store.credits.windows) != 0 || store.plans.tiers["alice"] != plan.Free {
		t.Fatalf("failure wrote state: subscriptions=%+v lots=%+v tier=%s", store.subscriptions, store.credits.windows, store.plans.tiers["alice"])
	}
	if len(store.events) != 1 || store.events[0].Kind != "charge_failed" {
		t.Fatalf("failure events = %+v", store.events)
	}
}

func TestSubscribeWritesAnchorChargeTierAndMonthlyLot(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 1, 31, 9, 30, 0, 0, seoul)
	store := newSubscriptionStore()
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, now)

	subscription, err := service.Subscribe(ctx, "alice", plan.Pro, TermAnnual)
	if err != nil {
		t.Fatal(err)
	}
	if !subscription.AnchorAt.Equal(now) || !subscription.TermStart.Equal(now) || subscription.TermEnd.Month() != time.January || subscription.TermEnd.Year() != 2027 {
		t.Fatalf("subscription = %+v", subscription)
	}
	if !subscription.NextGrantAt.Equal(time.Date(2026, 2, 28, 0, 0, 0, 0, seoul)) {
		t.Fatalf("next grant = %s", subscription.NextGrantAt)
	}
	if store.plans.tiers["alice"] != plan.Pro || len(store.credits.windows) != 1 {
		t.Fatalf("tier=%s windows=%+v", store.plans.tiers["alice"], store.credits.windows)
	}
	window := store.credits.windows[0]
	if !window.start.Equal(now) || !window.end.Equal(subscription.NextGrantAt) || window.tier != plan.Pro {
		t.Fatalf("window = %+v", window)
	}
	if len(provider.requests) != 1 || provider.requests[0].KRW != 69_625 || provider.requests[0].OrderID != "sub:alice:2026-01-31" {
		t.Fatalf("charge requests = %+v", provider.requests)
	}
	if kinds(store.events) != "charge,tier_change" || store.events[0].USDCents == nil || *store.events[0].USDCents != 5_000 {
		t.Fatalf("events = %+v", store.events)
	}
}

func TestSubscribeRecoversAProcessedOrderWithoutChargingTwice(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	provider := newSubscriptionProvider()
	provider.failAfterCharge = true
	service := newSubscriptionService(store, provider, now)

	if _, err := service.Subscribe(ctx, "alice", plan.Basic, TermMonthly); !errors.Is(err, ErrChargeFailed) {
		t.Fatalf("first = %v", err)
	}
	if _, err := service.Subscribe(ctx, "alice", plan.Basic, TermMonthly); err != nil {
		t.Fatalf("retry = %v", err)
	}
	if len(provider.requests) != 1 || len(store.subscriptions) != 1 || len(store.credits.windows) != 1 {
		t.Fatalf("requests=%d subscriptions=%d windows=%d", len(provider.requests), len(store.subscriptions), len(store.credits.windows))
	}
}

func TestRunDueCoversAnnualGrantRenewalFailureAndCancellation(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	due := time.Date(2026, 2, 15, 0, 0, 0, 0, seoul)

	t.Run("annual mid-term grant", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermAnnual, anchor, time.Date(2027, 1, 15, 0, 0, 0, 0, seoul), due, true)
		service := newSubscriptionService(store, newSubscriptionProvider(), due)
		if err := service.RunDue(ctx, due); err != nil {
			t.Fatal(err)
		}
		if len(store.credits.windows) != 1 || kinds(store.events) != "grant" || store.subscriptions["alice"].NextGrantAt.Month() != time.March {
			t.Fatalf("windows=%+v events=%+v subscription=%+v", store.credits.windows, store.events, store.subscriptions["alice"])
		}
	})

	t.Run("successful monthly renewal", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermMonthly, anchor, due, due, true)
		provider := newSubscriptionProvider()
		service := newSubscriptionService(store, provider, due)
		if err := service.RunDue(ctx, due); err != nil {
			t.Fatal(err)
		}
		updated := store.subscriptions["alice"]
		if !updated.TermStart.Equal(due) || updated.TermEnd.Month() != time.March || len(store.credits.windows) != 1 || kinds(store.events) != "charge" || len(store.mailer.messages) != 1 {
			t.Fatalf("subscription=%+v lots=%+v events=%+v mail=%+v", updated, store.credits.windows, store.events, store.mailer.messages)
		}
	})

	t.Run("failed renewal lapses once", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermMonthly, anchor, due, due, true)
		provider := newSubscriptionProvider()
		provider.chargeErr = errors.New("declined")
		service := newSubscriptionService(store, provider, due)
		if err := service.RunDue(ctx, due); err != nil {
			t.Fatal(err)
		}
		if store.subscriptions["alice"].Status != "lapsed" || store.plans.tiers["alice"] != plan.Free || kinds(store.events) != "renewal_failed" || len(store.mailer.messages) != 1 {
			t.Fatalf("subscription=%+v tier=%s events=%+v mail=%+v", store.subscriptions["alice"], store.plans.tiers["alice"], store.events, store.mailer.messages)
		}
		if err := service.RunDue(ctx, due.Add(time.Hour)); err != nil || len(provider.requests) != 1 || len(store.events) != 1 {
			t.Fatalf("lapsed retry: requests=%d events=%d err=%v", len(provider.requests), len(store.events), err)
		}
	})

	t.Run("scheduled cancellation lapses without charge", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, TermMonthly, anchor, due, due, false)
		provider := newSubscriptionProvider()
		service := newSubscriptionService(store, provider, due)
		if err := service.RunDue(ctx, due); err != nil {
			t.Fatal(err)
		}
		if store.subscriptions["alice"].Status != "lapsed" || store.plans.tiers["alice"] != plan.Free || kinds(store.events) != "cancelled" || len(provider.requests) != 0 || len(store.mailer.messages) != 1 {
			t.Fatalf("subscription=%+v tier=%s events=%+v requests=%+v mail=%+v", store.subscriptions["alice"], store.plans.tiers["alice"], store.events, provider.requests, store.mailer.messages)
		}
	})
}

func TestAnnualSubscriptionOpensTwelveMonthlyLotsForOneCharge(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 8, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	provider := newSubscriptionProvider()
	service := newSubscriptionService(store, provider, anchor)
	if _, err := service.Subscribe(ctx, "alice", plan.Max, TermAnnual); err != nil {
		t.Fatal(err)
	}
	for month := 1; month < 12; month++ {
		subscription := store.subscriptions["alice"]
		service.now = func() time.Time { return subscription.NextGrantAt }
		if err := service.RunDue(ctx, subscription.NextGrantAt); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.credits.windows) != 12 || len(provider.requests) != 1 {
		t.Fatalf("windows=%d charge requests=%d", len(store.credits.windows), len(provider.requests))
	}
}

func TestRunDueContinuesAfterOneSubscriptionFails(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	due := time.Date(2026, 2, 15, 0, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermAnnual, anchor, time.Date(2027, 1, 15, 0, 0, 0, 0, seoul), due, true)
	store.subscriptions["bob"] = activeSubscription("bob", plan.Basic, TermAnnual, anchor, time.Date(2027, 1, 15, 0, 0, 0, 0, seoul), due, true)
	store.upsertFailureUser = "alice"
	service := newSubscriptionService(store, newSubscriptionProvider(), due)

	if err := service.RunDue(ctx, due); err == nil || !strings.Contains(err.Error(), "alice") {
		t.Fatalf("RunDue error = %v", err)
	}
	if got := store.subscriptions["bob"].NextGrantAt; got.Month() != time.March {
		t.Fatalf("bob next grant = %s", got)
	}
	if len(store.credits.windows) != 1 || store.credits.windows[0].userID != "bob" {
		t.Fatalf("windows = %+v", store.credits.windows)
	}
}

type subscriptionStore struct {
	subscriptions     map[string]Subscription
	subscriptionReads int
	methods           map[string]PaymentMethod
	events            []Event
	purchases         map[string]Purchase
	credits           *subscriptionCredits
	plans             *subscriptionPlans
	mailer            *subscriptionMailer
	upsertFailureUser string
	// markRefundedErr fails the step that marks a purchase refunded, which is the crash a
	// resumable refund has to survive.
	markRefundedErr error
}

func newSubscriptionStore() *subscriptionStore {
	return &subscriptionStore{
		subscriptions: map[string]Subscription{},
		purchases:     map[string]Purchase{},
		methods:       map[string]PaymentMethod{"alice": testMethod("alice")},
		credits:       &subscriptionCredits{},
		plans:         &subscriptionPlans{tiers: map[string]plan.Plan{"alice": plan.Free}},
		mailer:        &subscriptionMailer{},
	}
}

func (s *subscriptionStore) InWriteTx(ctx context.Context, fn func(Store, Credits, Plans) error) error {
	return fn(s, s.credits, s.plans)
}
func (s *subscriptionStore) Subscription(_ context.Context, userID string) (Subscription, bool, error) {
	s.subscriptionReads++
	value, ok := s.subscriptions[userID]
	return value, ok, nil
}
func (s *subscriptionStore) PaymentMethod(_ context.Context, userID string) (PaymentMethod, bool, error) {
	value, ok := s.methods[userID]
	return value, ok, nil
}
func (s *subscriptionStore) Events(context.Context, string, int) ([]Event, error) {
	return s.events, nil
}
func (s *subscriptionStore) Purchases(_ context.Context, userID string) ([]Purchase, error) {
	var result []Purchase
	for _, purchase := range s.purchases {
		if purchase.UserID == userID {
			result = append(result, purchase)
		}
	}
	return result, nil
}
func (s *subscriptionStore) Purchase(_ context.Context, userID, purchaseID string) (Purchase, bool, error) {
	purchase, found := s.purchases[purchaseID]
	return purchase, found && purchase.UserID == userID, nil
}
func (*subscriptionStore) InsertProviderNotification(context.Context, ProviderNotification) error {
	return nil
}
func (s *subscriptionStore) UpsertPaymentMethod(_ context.Context, value PaymentMethod) error {
	s.methods[value.UserID] = value
	return nil
}
func (s *subscriptionStore) DeletePaymentMethod(_ context.Context, userID string) error {
	delete(s.methods, userID)
	return nil
}
func (s *subscriptionStore) InsertEvent(_ context.Context, event Event) error {
	s.events = append(s.events, event)
	return nil
}
func (s *subscriptionStore) InsertPurchase(_ context.Context, purchase Purchase) error {
	// Refundable is derived, never stored: the real store has no such column, so a row read
	// back must not carry the flag the caller was handed.
	purchase.Refundable = false
	s.purchases[purchase.ID] = purchase
	return nil
}
func (s *subscriptionStore) MarkPurchaseRefunded(_ context.Context, userID, purchaseID string, at time.Time) (bool, error) {
	if s.markRefundedErr != nil {
		return false, s.markRefundedErr
	}
	purchase, found := s.purchases[purchaseID]
	if !found || purchase.UserID != userID || purchase.RefundedAt != nil {
		return false, nil
	}
	purchase.RefundedAt = &at
	s.purchases[purchaseID] = purchase
	return true, nil
}
func (s *subscriptionStore) UpsertSubscription(_ context.Context, subscription Subscription) error {
	if subscription.UserID == s.upsertFailureUser {
		return errors.New("subscription write failed")
	}
	s.subscriptions[subscription.UserID] = subscription
	return nil
}
func (s *subscriptionStore) DueSubscriptions(_ context.Context, at time.Time) ([]Subscription, error) {
	var due []Subscription
	for _, subscription := range s.subscriptions {
		if subscription.Status == "active" && !subscription.NextGrantAt.After(at) {
			due = append(due, subscription)
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].UserID < due[j].UserID })
	return due, nil
}

type monthlyWindow struct {
	userID     string
	tier       plan.Plan
	start, end time.Time
	// started marks the window a first subscription charge opened (QUOTA-42) rather than a
	// renewal's absent-only one; both land in `windows` so a count still reads as windows.
	started bool
}
type subscriptionCredits struct {
	windows []monthlyWindow
	raises  []int
	lots    map[string]*purchaseLot
	lotSeq  int
}

type purchaseLot struct{ granted, remaining int }

func (c *subscriptionCredits) OpenMonthlyLot(_ context.Context, userID string, tier plan.Plan, start, end time.Time) error {
	c.windows = append(c.windows, monthlyWindow{userID: userID, tier: tier, start: start, end: end})
	return nil
}
func (c *subscriptionCredits) StartMonthlyWindow(_ context.Context, userID string, tier plan.Plan, start, end time.Time) error {
	c.windows = append(c.windows, monthlyWindow{userID: userID, tier: tier, start: start, end: end, started: true})
	return nil
}
func (c *subscriptionCredits) RaiseMonthlyLot(_ context.Context, _ string, credits int) error {
	c.raises = append(c.raises, credits)
	return nil
}
func (c *subscriptionCredits) OpenPurchasedLot(_ context.Context, _ string, credits int) (string, error) {
	if c.lots == nil {
		c.lots = map[string]*purchaseLot{}
	}
	c.lotSeq++
	id := fmt.Sprintf("purchased:lot-%d", c.lotSeq)
	c.lots[id] = &purchaseLot{granted: credits, remaining: credits}
	return id, nil
}
func (c *subscriptionCredits) VoidUntouchedLot(_ context.Context, lotID string) error {
	lot, found := c.lots[lotID]
	if !found || lot.remaining != lot.granted {
		return usage.ErrLotTouched
	}
	lot.remaining = 0
	return nil
}
func (c *subscriptionCredits) LotUntouched(_ context.Context, lotID string) (bool, error) {
	lot, found := c.lots[lotID]
	return found && lot.granted > 0 && lot.remaining == lot.granted, nil
}
func (c *subscriptionCredits) RestoreLot(_ context.Context, lotID string, credits int) error {
	lot, found := c.lots[lotID]
	if !found || lot.remaining+credits > lot.granted {
		return usage.ErrLotNotFound
	}
	lot.remaining += credits
	return nil
}
func (*subscriptionCredits) GrantBonusOnce(context.Context, string, string, int) (bool, error) {
	return false, nil
}

type subscriptionPlans struct{ tiers map[string]plan.Plan }

func (p *subscriptionPlans) AssignTier(_ context.Context, userID string, tier plan.Plan) error {
	p.tiers[userID] = tier
	return nil
}
func (p *subscriptionPlans) TierOf(_ context.Context, userID string) (plan.Plan, error) {
	return p.tiers[userID], nil
}

type subscriptionProvider struct {
	requests        []ChargeRequest
	payments        map[string]Payment
	chargeErr       error
	failAfterCharge bool
	refunds         []string
	refundErr       error
}

func newSubscriptionProvider() *subscriptionProvider {
	return &subscriptionProvider{payments: map[string]Payment{}}
}
func (*subscriptionProvider) IssueBillingKey(context.Context, string, string) (BillingKey, error) {
	return BillingKey{}, nil
}
func (p *subscriptionProvider) Charge(_ context.Context, request ChargeRequest) (Payment, error) {
	p.requests = append(p.requests, request)
	if p.failAfterCharge {
		p.failAfterCharge = false
		p.payments[request.OrderID] = Payment{PaymentKey: "paid-after-timeout", OrderID: request.OrderID, Status: "DONE"}
		return Payment{}, errors.New("response lost")
	}
	if p.chargeErr != nil {
		return Payment{}, p.chargeErr
	}
	payment := Payment{PaymentKey: "payment-" + request.OrderID, OrderID: request.OrderID, Status: "DONE"}
	p.payments[request.OrderID] = payment
	return payment, nil
}
func (p *subscriptionProvider) PaymentByOrder(_ context.Context, orderID string) (Payment, bool, error) {
	payment, ok := p.payments[orderID]
	return payment, ok, nil
}
func (p *subscriptionProvider) Refund(_ context.Context, paymentKey, reason string) error {
	if p.refundErr != nil {
		return p.refundErr
	}
	p.refunds = append(p.refunds, paymentKey+":"+reason)
	// A refunded payment stops being a live charge. It is the only evidence a resumed refund
	// has that the money already left, so the fake has to report it the way the provider does.
	for orderID, payment := range p.payments {
		if payment.PaymentKey == paymentKey {
			payment.Status = "CANCELED"
			p.payments[orderID] = payment
		}
	}
	return nil
}
func (*subscriptionProvider) ParseNotification(*http.Request) (Notification, error) {
	return Notification{}, nil
}

type subscriptionRates struct{}

func (subscriptionRates) KRWPerUSD(context.Context, time.Time) (int64, bool, error) {
	return 13_925_000, true, nil
}

type subscriptionAccounts struct{ verified bool }

func (a subscriptionAccounts) VerifiedEmail(context.Context, string) (string, bool, error) {
	return "alice@example.com", a.verified, nil
}

type sentBillingMail struct{ to, subject, text string }
type subscriptionMailer struct{ messages []sentBillingMail }

func (m *subscriptionMailer) Send(_ context.Context, to, subject, text string) error {
	m.messages = append(m.messages, sentBillingMail{to: to, subject: subject, text: text})
	return nil
}

func newSubscriptionService(store *subscriptionStore, provider *subscriptionProvider, now time.Time) *Service {
	service := NewService(store, provider, subscriptionRates{}, store.credits, store.plans, subscriptionAccounts{verified: true}, store.mailer)
	service.now = func() time.Time { return now }
	return service
}

func testMethod(userID string) PaymentMethod {
	return PaymentMethod{UserID: userID, BillingKey: "billing-key", CustomerKey: CustomerKey(userID), CardLabel: "11 1234"}
}

func activeSubscription(userID string, tier plan.Plan, term Term, anchor, termEnd, next time.Time, autoRenew bool) Subscription {
	return Subscription{UserID: userID, Tier: tier, Term: term, AnchorAt: anchor, TermStart: anchor, TermEnd: termEnd, NextGrantAt: next, AutoRenew: autoRenew, Status: "active", CreatedAt: anchor, UpdatedAt: anchor}
}

func kinds(events []Event) string {
	values := make([]string, len(events))
	for index, event := range events {
		values[index] = event.Kind
	}
	return strings.Join(values, ",")
}
