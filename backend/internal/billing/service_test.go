package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

func TestMoneyAndTermRules(t *testing.T) {
	// BILL-2, BILL-4: the fixed VAT-inclusive KRW offer, an annual term at ten monthly prices.
	for _, tc := range []struct {
		tier                     plan.Plan
		monthly, annual, credits int
	}{
		{plan.Light, 1_900, 19_000, 0}, {plan.Basic, 4_900, 49_000, 330}, {plan.Pro, 9_900, 99_000, 1_150}, {plan.Max, 29_900, 299_000, 2_400},
	} {
		monthly, err := offerQuote(tc.tier, TermMonthly)
		if err != nil || monthly.KRW != tc.monthly {
			t.Errorf("%s monthly=%+v err=%v", tc.tier, monthly, err)
		}
		annual, err := offerQuote(tc.tier, TermAnnual)
		if err != nil || annual.KRW != tc.annual || annual.KRW != 10*monthly.KRW {
			t.Errorf("%s annual=%+v err=%v", tc.tier, annual, err)
		}
		if got := plan.MonthlyCredits(tc.tier); got != tc.credits {
			t.Errorf("%s credits=%d", tc.tier, got)
		}
	}
	for _, tier := range []plan.Plan{plan.Free, plan.Master} {
		if _, err := offerQuote(tier, TermMonthly); !errors.Is(err, ErrTierNotSubscribable) {
			t.Errorf("%s quote err=%v, want ErrTierNotSubscribable", tier, err)
		}
	}
	anchor := time.Date(2026, 1, 31, 0, 0, 0, 0, seoul)
	if got := TermEnd(anchor, anchor, TermMonthly); got.Day() != 28 || got.Month() != time.February {
		t.Fatalf("monthly end=%s", got)
	}
	if got := TermEnd(anchor, anchor, TermAnnual); !got.Equal(time.Date(2027, 1, 31, 0, 0, 0, 0, seoul)) {
		t.Fatalf("annual end=%s", got)
	}
	if a, b := CustomerKey("alice"), CustomerKey("alice"); a != b || len(a) != 46 || a[:3] != "pp_" || a == CustomerKey("bob") {
		t.Fatalf("unstable customer key %q", a)
	}
}

func TestRegisterPaymentMethodGatesBeforeProviderAndWritesNothingOnProviderFailure(t *testing.T) {
	ctx := context.Background()
	store := newRegistrationStore()
	provider := &registrationProvider{}
	svc := NewService(store, provider, store.credits, nil, registrationAccounts{}, nil)

	_, err := svc.RegisterPaymentMethod(ctx, "unverified", "auth", CustomerKey("unverified"))
	if !errors.Is(err, ErrEmailVerificationRequired) || provider.calls != 0 {
		t.Fatalf("email gate err=%v provider calls=%d", err, provider.calls)
	}
	_, err = svc.RegisterPaymentMethod(ctx, "alice", "auth", CustomerKey("mallory"))
	if !errors.Is(err, ErrCustomerKeyMismatch) || provider.calls != 0 {
		t.Fatalf("key gate err=%v provider calls=%d", err, provider.calls)
	}
	provider.err = errors.New("provider down")
	_, err = svc.RegisterPaymentMethod(ctx, "alice", "auth", CustomerKey("alice"))
	if err == nil || provider.calls != 1 || store.txCalls != 0 || store.method != nil || len(store.events) != 0 {
		t.Fatalf("provider failure err=%v provider=%d tx=%d method=%+v events=%+v", err, provider.calls, store.txCalls, store.method, store.events)
	}
}

func TestRegisterPaymentMethodReplacesCardWithoutGrantingCredits(t *testing.T) {
	ctx := context.Background()
	store := newRegistrationStore()
	provider := &registrationProvider{}
	svc := NewService(store, provider, store.credits, nil, registrationAccounts{}, nil)
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	first, err := svc.RegisterPaymentMethod(ctx, "alice", "auth-1", CustomerKey("alice"))
	if err != nil || first.BonusGranted || store.method == nil || store.method.CardLabel != "11 1234" || len(store.events) != 1 {
		t.Fatalf("first=%+v method=%+v events=%+v err=%v", first, store.method, store.events, err)
	}
	if store.events[0].Kind != "method_registered" {
		t.Fatalf("first events=%+v", store.events)
	}
	provider.cardLabel = "22 9876"
	second, err := svc.RegisterPaymentMethod(ctx, "alice", "auth-2", CustomerKey("alice"))
	if err != nil || second.BonusGranted || store.method.CardLabel != "22 9876" || len(store.events) != 2 || len(store.credits.grants) != 0 {
		t.Fatalf("second=%+v method=%+v events=%+v grants=%+v err=%v", second, store.method, store.events, store.credits.grants, err)
	}
}

func TestRemovePaymentMethodRefusesOnlyAnActiveRenewingSubscription(t *testing.T) {
	ctx := context.Background()
	store := newRegistrationStore()
	store.method = &PaymentMethod{UserID: "alice"}
	store.subscription = &Subscription{UserID: "alice", Status: "active", AutoRenew: true}
	svc := NewService(store, &registrationProvider{}, store.credits, nil, registrationAccounts{}, nil)
	if err := svc.RemovePaymentMethod(ctx, "alice"); !errors.Is(err, ErrSubscriptionNeedsMethod) || store.method == nil {
		t.Fatalf("renewing removal err=%v method=%+v", err, store.method)
	}
	store.subscription.AutoRenew = false
	if err := svc.RemovePaymentMethod(ctx, "alice"); err != nil || store.method != nil || len(store.credits.grants) != 0 {
		t.Fatalf("cancelled removal err=%v method=%+v grants=%+v", err, store.method, store.credits.grants)
	}
}

type registrationStore struct {
	noIntents
	noRefunds
	method       *PaymentMethod
	subscription *Subscription
	events       []Event
	txCalls      int
	credits      *registrationCredits
}

// noIntents is a checkout journal holding no order, for the cases that only need the account
// to have no payment in flight.
type noIntents struct{}

func (noIntents) PutQuote(context.Context, QuoteRecord) error { return nil }
func (noIntents) Quote(context.Context, string) (QuoteRecord, bool, error) {
	return QuoteRecord{}, false, nil
}
func (noIntents) PurgeExpiredQuotes(context.Context, time.Time) (int, error) { return 0, nil }
func (noIntents) InsertIntent(context.Context, Intent) error                 { return nil }
func (noIntents) Intent(context.Context, string) (Intent, bool, error)       { return Intent{}, false, nil }
func (noIntents) PendingIntent(context.Context, string) (Intent, bool, error) {
	return Intent{}, false, nil
}
func (noIntents) DueIntents(context.Context, time.Time) ([]Intent, error) { return nil, nil }
func (noIntents) MarkIntent(context.Context, string, string, string, string, time.Time) (bool, error) {
	return false, nil
}
func (noIntents) ReviewIntents(context.Context, int) ([]Intent, error) { return nil, nil }
func (noIntents) FailReviewIntent(context.Context, string, string, time.Time) (bool, error) {
	return false, nil
}

// noRefunds is a refund ledger holding no request, for the cases that never refund: an applied
// payment's funding goes nowhere and the store carries no refund benefits.
type noRefunds struct{}

func (noRefunds) SetIntentFunding(context.Context, string, string, time.Time) error { return nil }
func (noRefunds) RefundPayment(context.Context, string, string) (RefundPayment, bool, error) {
	return RefundPayment{}, false, nil
}
func (noRefunds) RefundRequest(context.Context, string) (RefundRequest, bool, error) {
	return RefundRequest{}, false, nil
}
func (noRefunds) OpenRefundForOrder(context.Context, string) (bool, error) { return false, nil }
func (noRefunds) Refunds(context.Context, string) ([]RefundRequest, error) { return nil, nil }
func (noRefunds) ProcessingRefundIDs(context.Context, time.Time, int) ([]string, error) {
	return nil, nil
}
func (noRefunds) ReviewedEvidence(context.Context, string) (RefundEvidence, bool, error) {
	return RefundEvidence{}, false, nil
}
func (noRefunds) InsertRefundRequest(context.Context, RefundRequest) error { return nil }
func (noRefunds) RecordRefundDecision(context.Context, RefundRequest, RefundDecision) error {
	return nil
}
func (noRefunds) RecordRefundProviderAttempt(context.Context, string, string) error { return nil }
func (noRefunds) RecordRefundOutcome(context.Context, RefundRequest, Payment, time.Time) error {
	return nil
}
func (noRefunds) FailRefund(context.Context, string, string, time.Time) error { return nil }
func (noRefunds) ConfirmedRefundTotal(context.Context, string) (int, error)   { return 0, nil }
func (noRefunds) HasUnresolvedDependentUpgrade(context.Context, RefundPayment) (bool, error) {
	return false, nil
}
func (noRefunds) RefundBenefits() RefundBenefits { return nil }

func newRegistrationStore() *registrationStore {
	return &registrationStore{credits: &registrationCredits{grants: map[string]bool{}}}
}
func (s *registrationStore) InWriteTx(ctx context.Context, fn func(Store, Credits, Plans) error) error {
	s.txCalls++
	return fn(s, s.credits, registrationPlans{})
}
func (s *registrationStore) Subscription(context.Context, string) (Subscription, bool, error) {
	if s.subscription == nil {
		return Subscription{}, false, nil
	}
	return *s.subscription, true, nil
}
func (s *registrationStore) PaymentMethod(context.Context, string) (PaymentMethod, bool, error) {
	if s.method == nil {
		return PaymentMethod{}, false, nil
	}
	return *s.method, true, nil
}
func (s *registrationStore) Events(context.Context, string, int) ([]Event, error) {
	return s.events, nil
}
func (*registrationStore) Purchases(context.Context, string) ([]Purchase, error) { return nil, nil }
func (*registrationStore) Purchase(context.Context, string, string) (Purchase, bool, error) {
	return Purchase{}, false, nil
}
func (*registrationStore) InsertProviderNotification(context.Context, ProviderNotification) error {
	return nil
}
func (s *registrationStore) UpsertPaymentMethod(_ context.Context, method PaymentMethod) error {
	s.method = &method
	return nil
}
func (s *registrationStore) DeletePaymentMethod(context.Context, string) error {
	s.method = nil
	return nil
}
func (s *registrationStore) InsertEvent(_ context.Context, event Event) error {
	s.events = append(s.events, event)
	return nil
}
func (*registrationStore) InsertPurchase(context.Context, Purchase) error { return nil }
func (*registrationStore) MarkPurchaseRefunded(context.Context, string, string, time.Time) (bool, error) {
	return false, nil
}
func (s *registrationStore) UpsertSubscription(_ context.Context, subscription Subscription) error {
	s.subscription = &subscription
	return nil
}
func (*registrationStore) DueSubscriptions(context.Context, time.Time) ([]Subscription, error) {
	return nil, nil
}
func (*registrationStore) AdvanceNextGrant(context.Context, string, time.Time, time.Time) (bool, error) {
	return false, nil
}

type registrationCredits struct{ grants map[string]bool }

type registrationPlans struct{}

func (registrationPlans) AssignTier(context.Context, string, plan.Plan) error   { return nil }
func (registrationPlans) ReassignTier(context.Context, string, plan.Plan) error { return nil }
func (registrationPlans) TierOf(context.Context, string) (plan.Plan, error)     { return plan.Free, nil }

func (*registrationCredits) OpenPurchasedLot(context.Context, string, int) (string, error) {
	return "", nil
}
func (*registrationCredits) UntouchedLots(context.Context, []string) (map[string]bool, error) {
	return nil, nil
}
func (c *registrationCredits) GrantBonusOnce(_ context.Context, id, _ string, _ int) (bool, error) {
	if c.grants[id] {
		return false, nil
	}
	c.grants[id] = true
	return true, nil
}

type registrationAccounts struct{}

func (registrationAccounts) VerifiedEmail(_ context.Context, userID string) (string, bool, error) {
	if userID == "unverified" {
		return "", false, nil
	}
	return "alice@example.com", true, nil
}

type registrationProvider struct {
	stubProvider
	calls     int
	err       error
	cardLabel string
}

func (p *registrationProvider) IssueBillingKey(_ context.Context, _, customerKey string) (BillingKey, error) {
	p.calls++
	if p.err != nil {
		return BillingKey{}, p.err
	}
	label := p.cardLabel
	if label == "" {
		label = "11 1234"
	}
	return BillingKey{Value: "billing-key", CustomerKey: customerKey, CardLabel: label}, nil
}

func TestDisabledServiceStillReadsButWillNotQuote(t *testing.T) {
	svc := NewService(emptyStore{}, nil, nil, nil, nil, nil)
	view, err := svc.GetMyBilling(context.Background(), "alice")
	if err != nil || view.CustomerKey != CustomerKey("alice") {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	if _, err := svc.QuotePrice(context.Background(), plan.Basic, TermMonthly); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

// emptyStore holds nothing, its checkout journal and refund ledger included.
type emptyStore struct {
	noIntents
	noRefunds
}

func (emptyStore) InWriteTx(ctx context.Context, fn func(Store, Credits, Plans) error) error {
	return fn(emptyStore{}, nil, nil)
}
func (emptyStore) Subscription(context.Context, string) (Subscription, bool, error) {
	return Subscription{}, false, nil
}
func (emptyStore) PaymentMethod(context.Context, string) (PaymentMethod, bool, error) {
	return PaymentMethod{}, false, nil
}
func (emptyStore) Events(context.Context, string, int) ([]Event, error)  { return nil, nil }
func (emptyStore) Purchases(context.Context, string) ([]Purchase, error) { return nil, nil }
func (emptyStore) Purchase(context.Context, string, string) (Purchase, bool, error) {
	return Purchase{}, false, nil
}
func (emptyStore) InsertProviderNotification(context.Context, ProviderNotification) error { return nil }
func (emptyStore) UpsertPaymentMethod(context.Context, PaymentMethod) error               { return nil }
func (emptyStore) DeletePaymentMethod(context.Context, string) error                      { return nil }
func (emptyStore) InsertEvent(context.Context, Event) error                               { return nil }
func (emptyStore) InsertPurchase(context.Context, Purchase) error                         { return nil }
func (emptyStore) MarkPurchaseRefunded(context.Context, string, string, time.Time) (bool, error) {
	return false, nil
}
func (emptyStore) UpsertSubscription(context.Context, Subscription) error { return nil }
func (emptyStore) DueSubscriptions(context.Context, time.Time) ([]Subscription, error) {
	return nil, nil
}
func (emptyStore) AdvanceNextGrant(context.Context, string, time.Time, time.Time) (bool, error) {
	return false, nil
}

type stubProvider struct{ noCancel }

// noCancel is a provider that never answers a cancel, for the cases that refund nothing: an
// unapplied capture stays in review as it would after a lost answer.
type noCancel struct{}

func (noCancel) CancelPayment(context.Context, string, int, string, string) (Payment, error) {
	return Payment{}, errors.New("no cancel answered")
}

func (stubProvider) IssueBillingKey(context.Context, string, string) (BillingKey, error) {
	return BillingKey{}, nil
}
func (stubProvider) Charge(context.Context, ChargeRequest) (Payment, error) { return Payment{}, nil }
func (stubProvider) PaymentByOrder(context.Context, string) (Payment, bool, error) {
	return Payment{}, false, nil
}
func (stubProvider) ParseNotification([]byte) (Notification, error) {
	return Notification{}, nil
}

func (*registrationStore) TierAt(context.Context, string, string, time.Time) (plan.Plan, error) {
	return plan.Basic, nil
}
func (*registrationStore) InsertTierTransition(context.Context, string, string, time.Time, plan.Plan, string) error {
	return nil
}
func (*registrationStore) SupportCoverage(context.Context, string) (SupportCoverage, bool, error) {
	return SupportCoverage{}, false, nil
}
func (*registrationStore) UpsertSupportCoverage(context.Context, SupportCoverage) error { return nil }
func (*registrationStore) DeleteSupportCoverage(context.Context, string) error          { return nil }
func (emptyStore) TierAt(context.Context, string, string, time.Time) (plan.Plan, error) {
	return plan.Basic, nil
}
func (emptyStore) InsertTierTransition(context.Context, string, string, time.Time, plan.Plan, string) error {
	return nil
}
func (emptyStore) SupportCoverage(context.Context, string) (SupportCoverage, bool, error) {
	return SupportCoverage{}, false, nil
}
func (emptyStore) UpsertSupportCoverage(context.Context, SupportCoverage) error { return nil }
func (emptyStore) DeleteSupportCoverage(context.Context, string) error          { return nil }
func (*registrationCredits) OpenCoverage(context.Context, string, Coverage, time.Time, string) error {
	return nil
}
func (*registrationCredits) AddUpgradeBonus(context.Context, string, Coverage, time.Time, int, int, string) error {
	return nil
}
