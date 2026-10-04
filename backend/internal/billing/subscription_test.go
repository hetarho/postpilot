package billing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// seoul is the product's home zone, the clock these cases are written in.
var seoul = time.FixedZone("Asia/Seoul", 9*60*60)

// declined is the card's own refusal, the one charge answer that fails an order for good.
var declined = &ProviderError{Code: "REJECT_CARD_PAYMENT", HTTPStatus: 403}

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
	if len(store.intents) != 0 {
		t.Fatalf("a refusal recorded orders: %+v", store.intents)
	}
	store.methods["alice"] = testMethod("alice")
	provider.chargeErr = declined
	if _, err := service.Subscribe(ctx, "alice", plan.Pro, TermMonthly); !errors.Is(err, ErrChargeFailed) {
		t.Fatalf("charge refusal = %v", err)
	}
	if len(store.subscriptions) != 0 || len(store.credits.windows) != 0 || store.plans.tiers["alice"] != plan.Free {
		t.Fatalf("failure wrote state: subscriptions=%+v lots=%+v tier=%s", store.subscriptions, store.credits.windows, store.plans.tiers["alice"])
	}
	if len(store.events) != 1 || store.events[0].Kind != "charge_failed" {
		t.Fatalf("failure events = %+v", store.events)
	}
	// The refused order is closed, so it blocks no later purchase.
	if _, pending, err := store.PendingIntent(ctx, "alice"); err != nil || pending {
		t.Fatalf("refused order still open: pending=%v err=%v", pending, err)
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
	if !subscription.NextGrantAt.Equal(time.Date(2026, 2, 28, 9, 30, 0, 0, seoul)) {
		t.Fatalf("next grant = %s", subscription.NextGrantAt)
	}
	if store.plans.tiers["alice"] != plan.Pro || len(store.credits.windows) != 1 {
		t.Fatalf("tier=%s windows=%+v", store.plans.tiers["alice"], store.credits.windows)
	}
	window := store.credits.windows[0]
	if !window.start.Equal(now) || !window.end.Equal(subscription.NextGrantAt) || window.tier != plan.Pro {
		t.Fatalf("window = %+v", window)
	}
	offer, _ := plan.CommercialOffer(plan.Pro)
	if len(provider.requests) != 1 || provider.requests[0].KRW != offer.AnnualKRW || !strings.HasPrefix(provider.requests[0].OrderID, "pp-sub-") {
		t.Fatalf("charge requests = %+v", provider.requests)
	}
	// BILL-15: the charge records the fixed KRW amount.
	charge := store.events[0]
	if kinds(store.events) != "charge,tier_change" || charge.KRW == nil || *charge.KRW != offer.AnnualKRW {
		t.Fatalf("events = %+v", store.events)
	}
}

// A charge the provider captured but whose answer was lost keeps its order open: the request
// path reports it pending, a retry starts no second order, and the next billing pass applies
// it once.
func TestSubscribeAppliesACapturedChargeWhoseAnswerWasLostOnce(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, seoul)
	store := newSubscriptionStore()
	provider := newSubscriptionProvider()
	provider.failAfterCharge = true
	service := newSubscriptionService(store, provider, now)

	if _, err := service.Subscribe(ctx, "alice", plan.Basic, TermMonthly); !errors.Is(err, ErrPaymentPending) {
		t.Fatalf("first = %v", err)
	}
	if _, err := service.Subscribe(ctx, "alice", plan.Basic, TermMonthly); !errors.Is(err, ErrPaymentPending) {
		t.Fatalf("retry while the order is open = %v", err)
	}
	if len(store.subscriptions) != 0 {
		t.Fatalf("an unconfirmed charge granted %+v", store.subscriptions)
	}
	service.now = func() time.Time { return now.Add(pendingSettleGrace) }
	for pass := 0; pass < 2; pass++ {
		if err := service.ReconcilePending(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(provider.requests) != 1 || len(store.subscriptions) != 1 || len(store.credits.windows) != 1 || kinds(store.events) != "charge,tier_change" {
		t.Fatalf("requests=%d subscriptions=%d windows=%d events=%s", len(provider.requests), len(store.subscriptions), len(store.credits.windows), kinds(store.events))
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
		if len(store.credits.windows) != 0 || len(store.events) != 0 || store.subscriptions["alice"].NextGrantAt.Month() != time.March {
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
		offer, _ := plan.CommercialOffer(plan.Pro)
		if !updated.TermStart.Equal(due) || updated.TermEnd.Month() != time.March || len(store.credits.windows) != 1 || kinds(store.events) != "charge" || len(store.mailer.messages) != 1 {
			t.Fatalf("subscription=%+v lots=%+v events=%+v mail=%+v", updated, store.credits.windows, store.events, store.mailer.messages)
		}
		if len(provider.requests) != 1 || provider.requests[0].KRW != offer.MonthlyKRW {
			t.Fatalf("charge requests = %+v", provider.requests)
		}
	})

	// BILL-8: a finally refused renewal ends paid coverage at once, without a retry.
	t.Run("failed renewal lapses once", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermMonthly, anchor, due, due, true)
		provider := newSubscriptionProvider()
		provider.chargeErr = declined
		service := newSubscriptionService(store, provider, due)
		if err := service.RunDue(ctx, due); err == nil || !strings.Contains(err.Error(), "alice") {
			t.Fatalf("a refused renewal must be observable: %v", err)
		}
		if store.subscriptions["alice"].Status != "lapsed" || store.plans.tiers["alice"] != plan.Free || kinds(store.events) != "renewal_failed" || len(store.mailer.messages) != 1 {
			t.Fatalf("subscription=%+v tier=%s events=%+v mail=%+v", store.subscriptions["alice"], store.plans.tiers["alice"], store.events, store.mailer.messages)
		}
		if err := service.RunDue(ctx, due.Add(time.Hour)); err != nil || len(provider.requests) != 1 || len(store.events) != 1 {
			t.Fatalf("lapsed retry: requests=%d events=%d err=%v", len(provider.requests), len(store.events), err)
		}
	})

	// BILL-20: a subscription a master account still holds ends uncharged even with
	// auto-renew on, the tier stays master and the notice does not claim a drop to free.
	t.Run("master account lapses without charge and stays master", func(t *testing.T) {
		store := newSubscriptionStore()
		store.plans.tiers["alice"] = plan.Master
		store.subscriptions["alice"] = activeSubscription("alice", plan.Basic, TermMonthly, anchor, due, due, true)
		provider := newSubscriptionProvider()
		service := newSubscriptionService(store, provider, due)
		if err := service.RunDue(ctx, due); err != nil {
			t.Fatal(err)
		}
		if store.subscriptions["alice"].Status != "lapsed" || store.plans.tiers["alice"] != plan.Master || len(provider.requests) != 0 {
			t.Fatalf("subscription=%+v tier=%s requests=%+v", store.subscriptions["alice"], store.plans.tiers["alice"], provider.requests)
		}
		if len(store.mailer.messages) != 1 || !strings.Contains(store.mailer.messages[0].text, "운영자 계정이라") || strings.Contains(store.mailer.messages[0].text, "free") {
			t.Fatalf("mail = %+v", store.mailer.messages)
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

func TestAnnualSubscriptionDoesNotPreGrantTwelveMonthlyBenefits(t *testing.T) {
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
	if len(store.credits.windows) != 1 || len(provider.requests) != 1 {
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
	if len(store.credits.windows) != 0 {
		t.Fatalf("windows = %+v", store.credits.windows)
	}
}

// F38: the annual benefit step never writes the snapshot the pass read. A write landing between
// DueSubscriptions and the step keeps every column it wrote; when that writer also moved the
// boundary, the step matches nothing and the loop continues from a fresh read.
func TestAnnualBenefitStepNeverRevertsAWriteLandingDuringThePass(t *testing.T) {
	ctx := context.Background()
	anchor := time.Date(2026, 1, 15, 0, 0, 0, 0, seoul)
	due := time.Date(2026, 2, 15, 0, 0, 0, 0, seoul)
	termEnd := time.Date(2027, 1, 15, 0, 0, 0, 0, seoul)
	later := due.Add(time.Minute)

	t.Run("cancel keeps auto-renew off and its updated_at", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermAnnual, anchor, termEnd, due, true)
		store.afterDue = func() {
			cancelled := store.subscriptions["alice"]
			cancelled.AutoRenew, cancelled.UpdatedAt = false, later
			store.subscriptions["alice"] = cancelled
		}
		service := newSubscriptionService(store, newSubscriptionProvider(), due)
		if err := service.RunDue(ctx, due); err != nil {
			t.Fatal(err)
		}
		after := store.subscriptions["alice"]
		if after.AutoRenew || !after.UpdatedAt.Equal(later) || after.NextGrantAt.Month() != time.March {
			t.Fatalf("subscription = %+v", after)
		}
	})

	t.Run("refund lapse that moved the boundary stays lapsed", func(t *testing.T) {
		store := newSubscriptionStore()
		store.subscriptions["alice"] = activeSubscription("alice", plan.Pro, TermAnnual, anchor, termEnd, due, true)
		store.afterDue = func() {
			lapsed := store.subscriptions["alice"]
			lapsed.Status, lapsed.AutoRenew = "lapsed", false
			lapsed.TermEnd, lapsed.NextGrantAt, lapsed.UpdatedAt = later, later, later
			store.subscriptions["alice"] = lapsed
		}
		service := newSubscriptionService(store, newSubscriptionProvider(), due)
		if err := service.RunDue(ctx, due); err != nil {
			t.Fatal(err)
		}
		after := store.subscriptions["alice"]
		if after.Status != "lapsed" || !after.NextGrantAt.Equal(later) || !after.TermEnd.Equal(later) {
			t.Fatalf("subscription = %+v", after)
		}
	})
}

type subscriptionStore struct {
	subscriptions     map[string]Subscription
	methods           map[string]PaymentMethod
	events            []Event
	purchases         map[string]Purchase
	credits           *subscriptionCredits
	plans             *subscriptionPlans
	mailer            *subscriptionMailer
	upsertFailureUser string
	// quotes and intents are the checkout journal every payment runs through.
	quotes  map[string]QuoteRecord
	intents []Intent
	// afterDue runs once DueSubscriptions has handed out its snapshot: a write it makes is one
	// landing between the pass's read and its steps.
	afterDue func()
}

func newSubscriptionStore() *subscriptionStore {
	return &subscriptionStore{
		subscriptions: map[string]Subscription{},
		purchases:     map[string]Purchase{},
		methods:       map[string]PaymentMethod{"alice": testMethod("alice")},
		credits:       &subscriptionCredits{},
		plans:         &subscriptionPlans{tiers: map[string]plan.Plan{"alice": plan.Free}},
		mailer:        &subscriptionMailer{},
		quotes:        map[string]QuoteRecord{},
	}
}

func (s *subscriptionStore) InWriteTx(ctx context.Context, fn func(Store, Credits, Plans) error) error {
	return fn(s, s.credits, s.plans)
}
func (s *subscriptionStore) Subscription(_ context.Context, userID string) (Subscription, bool, error) {
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
	if s.afterDue != nil {
		s.afterDue()
	}
	return due, nil
}
func (s *subscriptionStore) AdvanceNextGrant(_ context.Context, userID string, from, to time.Time) (bool, error) {
	if userID == s.upsertFailureUser {
		return false, errors.New("subscription write failed")
	}
	current, found := s.subscriptions[userID]
	if !found || !current.NextGrantAt.Equal(from) {
		return false, nil
	}
	current.NextGrantAt = to
	s.subscriptions[userID] = current
	return true, nil
}

// The journal keeps the real store's transitions: an order leaves pending (or review) once,
// and a pending or in-review order blocks the account's next payment.
func (s *subscriptionStore) PutQuote(_ context.Context, quote QuoteRecord) error {
	s.quotes[quote.ID] = quote
	return nil
}
func (s *subscriptionStore) Quote(_ context.Context, id string) (QuoteRecord, bool, error) {
	quote, found := s.quotes[id]
	return quote, found, nil
}
func (s *subscriptionStore) PurgeExpiredQuotes(_ context.Context, expiredBefore time.Time) (int, error) {
	purged := 0
	for id, quote := range s.quotes {
		if quote.ExpiresAt.Before(expiredBefore) {
			delete(s.quotes, id)
			purged++
		}
	}
	return purged, nil
}
func (s *subscriptionStore) InsertIntent(_ context.Context, intent Intent) error {
	for _, existing := range s.intents {
		if existing.OrderID == intent.OrderID {
			return fmt.Errorf("order %s already recorded", intent.OrderID)
		}
	}
	s.intents = append(s.intents, intent)
	return nil
}
func (s *subscriptionStore) Intent(_ context.Context, orderID string) (Intent, bool, error) {
	for _, intent := range s.intents {
		if intent.OrderID == orderID {
			return intent, true, nil
		}
	}
	return Intent{}, false, nil
}
func (s *subscriptionStore) PendingIntent(_ context.Context, userID string) (Intent, bool, error) {
	for _, intent := range s.intents {
		if intent.UserID == userID && (intent.Status == "pending" || intent.Status == "review") {
			return intent, true, nil
		}
	}
	return Intent{}, false, nil
}
func (s *subscriptionStore) DueIntents(_ context.Context, createdBefore time.Time) ([]Intent, error) {
	var due []Intent
	for _, intent := range s.intents {
		if intent.Status == "pending" && !intent.CreatedAt.After(createdBefore) {
			due = append(due, intent)
		}
	}
	return due, nil
}
func (s *subscriptionStore) MarkIntent(_ context.Context, orderID, status, providerStatus, paymentKey string, at time.Time) (bool, error) {
	for index := range s.intents {
		intent := &s.intents[index]
		if intent.OrderID == orderID && intent.Status == "pending" {
			intent.Status, intent.ProviderStatus, intent.PaymentKey, intent.UpdatedAt = status, providerStatus, paymentKey, at
			return true, nil
		}
	}
	return false, nil
}
func (s *subscriptionStore) ReviewIntents(_ context.Context, limit int) ([]Intent, error) {
	var review []Intent
	for _, intent := range s.intents {
		if intent.Status == "review" && len(review) < limit {
			review = append(review, intent)
		}
	}
	return review, nil
}
func (s *subscriptionStore) FailReviewIntent(_ context.Context, orderID, providerStatus string, at time.Time) (bool, error) {
	for index := range s.intents {
		intent := &s.intents[index]
		if intent.OrderID == orderID && intent.Status == "review" {
			intent.Status, intent.ProviderStatus, intent.UpdatedAt = "failed", providerStatus, at
			return true, nil
		}
	}
	return false, nil
}

type coverageWindow struct {
	userID     string
	tier       plan.Plan
	start, end time.Time
}
type subscriptionCredits struct {
	windows []coverageWindow
	raises  []int
	lots    map[string]*purchaseLot
	lotSeq  int
	// untouchedReads counts the plural refundability reads: a screen must ask once, and an
	// account with nothing in window must not ask at all.
	untouchedReads int
}

type purchaseLot struct{ granted, remaining int }

func (c *subscriptionCredits) OpenPurchasedLot(_ context.Context, _ string, credits int) (string, error) {
	if c.lots == nil {
		c.lots = map[string]*purchaseLot{}
	}
	c.lotSeq++
	id := fmt.Sprintf("purchased:lot-%d", c.lotSeq)
	c.lots[id] = &purchaseLot{granted: credits, remaining: credits}
	return id, nil
}
func (c *subscriptionCredits) UntouchedLots(_ context.Context, lotIDs []string) (map[string]bool, error) {
	c.untouchedReads++
	untouched := map[string]bool{}
	for _, id := range lotIDs {
		lot, found := c.lots[id]
		untouched[id] = found && lot.granted > 0 && lot.remaining == lot.granted
	}
	return untouched, nil
}
func (*subscriptionCredits) GrantBonusOnce(context.Context, string, string, int) (bool, error) {
	return false, nil
}

type subscriptionPlans struct{ tiers map[string]plan.Plan }

// AssignTier mirrors auth's master hold (QUOTA-63), so billing tests see the tier the real
// port would leave.
func (p *subscriptionPlans) AssignTier(_ context.Context, userID string, tier plan.Plan) error {
	if p.tiers[userID] == plan.Master {
		return nil
	}
	p.tiers[userID] = tier
	return nil
}
func (p *subscriptionPlans) ReassignTier(_ context.Context, userID string, tier plan.Plan) error {
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
}

func newSubscriptionProvider() *subscriptionProvider {
	return &subscriptionProvider{payments: map[string]Payment{}}
}
func (*subscriptionProvider) IssueBillingKey(context.Context, string, string) (BillingKey, error) {
	return BillingKey{}, nil
}

// Charge captures the order's amount in KRW, the provider answer settlement checks against
// the order (BILL-16).
func (p *subscriptionProvider) Charge(_ context.Context, request ChargeRequest) (Payment, error) {
	p.requests = append(p.requests, request)
	if p.chargeErr != nil {
		return Payment{}, p.chargeErr
	}
	payment := Payment{PaymentKey: "payment-" + request.OrderID, OrderID: request.OrderID, Status: "DONE",
		AmountKRW: request.KRW, BalanceKRW: request.KRW, Currency: "KRW"}
	p.payments[request.OrderID] = payment
	if p.failAfterCharge {
		p.failAfterCharge = false
		return Payment{}, errors.New("response lost")
	}
	return payment, nil
}
func (p *subscriptionProvider) PaymentByOrder(_ context.Context, orderID string) (Payment, bool, error) {
	payment, ok := p.payments[orderID]
	return payment, ok, nil
}
func (*subscriptionProvider) ParseNotification([]byte) (Notification, error) {
	return Notification{}, nil
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
	service := NewService(store, provider, store.credits, store.plans, subscriptionAccounts{verified: true}, store.mailer)
	service.now = func() time.Time { return now }
	return service
}

func testMethod(userID string) PaymentMethod {
	return PaymentMethod{UserID: userID, BillingKey: "billing-key", CustomerKey: CustomerKey(userID), CardLabel: "11 1234"}
}

func activeSubscription(userID string, tier plan.Plan, term Term, anchor, termEnd, next time.Time, autoRenew bool) Subscription {
	return Subscription{UserID: userID, CoverageID: "paid:" + userID, Tier: tier, Term: term, AnchorAt: anchor, TermStart: anchor, TermEnd: termEnd, NextGrantAt: next, AutoRenew: autoRenew, Status: "active", CreatedAt: anchor, UpdatedAt: anchor}
}

func kinds(events []Event) string {
	values := make([]string, len(events))
	for index, event := range events {
		values[index] = event.Kind
	}
	return strings.Join(values, ",")
}

func (s *subscriptionStore) TierAt(_ context.Context, userID, _ string, _ time.Time) (plan.Plan, error) {
	if value, found := s.subscriptions[userID]; found {
		return value.Tier, nil
	}
	return plan.Basic, nil
}
func (*subscriptionStore) InsertTierTransition(context.Context, string, string, time.Time, plan.Plan, string) error {
	return nil
}
func (*subscriptionStore) SupportCoverage(context.Context, string) (SupportCoverage, bool, error) {
	return SupportCoverage{}, false, nil
}
func (*subscriptionStore) UpsertSupportCoverage(context.Context, SupportCoverage) error { return nil }
func (*subscriptionStore) DeleteSupportCoverage(context.Context, string) error          { return nil }
func (c *subscriptionCredits) OpenCoverage(_ context.Context, userID string, coverage Coverage, at time.Time, _ string) error {
	_, end := plan.BenefitWindow(coverage.Anchor, at)
	c.windows = append(c.windows, coverageWindow{userID: userID, tier: coverage.Tier, start: at, end: end})
	return nil
}
func (c *subscriptionCredits) AddUpgradeBonus(_ context.Context, _ string, _ Coverage, _ time.Time, credits, _ int, _ string) error {
	c.raises = append(c.raises, credits)
	return nil
}
