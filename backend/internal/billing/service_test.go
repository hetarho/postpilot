package billing

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

func TestMoneyAndTermRules(t *testing.T) {
	if got := KRWFor(200, 13_925_000); got != 2_785 {
		t.Fatalf("KRWFor=%d", got)
	}
	if got := KRWFor(1, 13_955_000); got != 14 {
		t.Fatalf("half-up KRWFor=%d", got)
	}
	for _, tc := range []struct {
		tier                     plan.Plan
		monthly, annual, credits int
	}{
		{plan.Basic, 200, 2_000, 220}, {plan.Pro, 500, 5_000, 575}, {plan.Max, 1_000, 10_000, 1_200},
	} {
		if got := PriceCents(tc.tier, TermMonthly); got != tc.monthly {
			t.Errorf("%s monthly=%d", tc.tier, got)
		}
		if got := PriceCents(tc.tier, TermAnnual); got != tc.annual {
			t.Errorf("%s annual=%d", tc.tier, got)
		}
		if got := plan.MonthlyCredits(tc.tier); got != tc.credits {
			t.Errorf("%s credits=%d", tc.tier, got)
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
	svc := NewService(store, provider, &fakeRates{}, store.credits, nil, registrationAccounts{}, nil)

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

func TestRegisterPaymentMethodReplacesCardAndGrantsBonusOnlyOnce(t *testing.T) {
	ctx := context.Background()
	store := newRegistrationStore()
	provider := &registrationProvider{}
	svc := NewService(store, provider, &fakeRates{}, store.credits, nil, registrationAccounts{}, nil)
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	first, err := svc.RegisterPaymentMethod(ctx, "alice", "auth-1", CustomerKey("alice"))
	if err != nil || !first.BonusGranted || store.method == nil || store.method.CardLabel != "11 1234" || len(store.events) != 2 {
		t.Fatalf("first=%+v method=%+v events=%+v err=%v", first, store.method, store.events, err)
	}
	if store.events[0].Kind != "method_registered" || store.events[1].Kind != "grant" || store.events[1].Credits == nil || *store.events[1].Credits != plan.PaymentMethodBonusCredits {
		t.Fatalf("first events=%+v", store.events)
	}
	provider.cardLabel = "22 9876"
	second, err := svc.RegisterPaymentMethod(ctx, "alice", "auth-2", CustomerKey("alice"))
	if err != nil || second.BonusGranted || store.method.CardLabel != "22 9876" || len(store.events) != 3 || len(store.credits.grants) != 1 {
		t.Fatalf("second=%+v method=%+v events=%+v grants=%+v err=%v", second, store.method, store.events, store.credits.grants, err)
	}
}

func TestRemovePaymentMethodRefusesOnlyAnActiveRenewingSubscription(t *testing.T) {
	ctx := context.Background()
	store := newRegistrationStore()
	store.method = &PaymentMethod{UserID: "alice"}
	store.subscription = &Subscription{UserID: "alice", Status: "active", AutoRenew: true}
	svc := NewService(store, &registrationProvider{}, &fakeRates{}, store.credits, nil, registrationAccounts{}, nil)
	if err := svc.RemovePaymentMethod(ctx, "alice"); !errors.Is(err, ErrSubscriptionNeedsMethod) || store.method == nil {
		t.Fatalf("renewing removal err=%v method=%+v", err, store.method)
	}
	store.subscription.AutoRenew = false
	if err := svc.RemovePaymentMethod(ctx, "alice"); err != nil || store.method != nil || len(store.credits.grants) != 0 {
		t.Fatalf("cancelled removal err=%v method=%+v grants=%+v", err, store.method, store.credits.grants)
	}
}

type registrationStore struct {
	method       *PaymentMethod
	subscription *Subscription
	events       []Event
	txCalls      int
	credits      *registrationCredits
}

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

type registrationCredits struct{ grants map[string]bool }

type registrationPlans struct{}

func (registrationPlans) AssignTier(context.Context, string, plan.Plan) error { return nil }
func (registrationPlans) TierOf(context.Context, string) (plan.Plan, error)   { return plan.Free, nil }

func (*registrationCredits) StartMonthlyWindow(context.Context, string, plan.Plan, time.Time, time.Time) error {
	return nil
}

func (*registrationCredits) OpenMonthlyLot(context.Context, string, plan.Plan, time.Time, time.Time) error {
	return nil
}
func (*registrationCredits) RaiseMonthlyLot(context.Context, string, int) error { return nil }
func (*registrationCredits) OpenPurchasedLot(context.Context, string, int) (string, error) {
	return "", nil
}
func (*registrationCredits) VoidUntouchedLot(context.Context, string) error { return nil }
func (*registrationCredits) UntouchedLots(context.Context, []string) (map[string]bool, error) {
	return nil, nil
}

func (*registrationCredits) RestoreLot(context.Context, string, int) error { return nil }
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

type fakeRates struct {
	calls  []string
	values map[string]int64
}

func (f *fakeRates) KRWPerUSD(_ context.Context, date time.Time) (int64, bool, error) {
	key := date.Format(time.DateOnly)
	f.calls = append(f.calls, key)
	value, ok := f.values[key]
	return value, ok, nil
}

func TestRateForWalksBackFromMondayToFridayAndCaches(t *testing.T) {
	rates := &fakeRates{values: map[string]int64{"2026-09-04": 13_925_000}}
	svc := NewService(emptyStore{}, stubProvider{}, rates, nil, nil, nil, nil)
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, seoul)
	rate, day, err := svc.rateFor(context.Background(), now)
	if err != nil || rate != 13_925_000 || day != "2026-09-04" {
		t.Fatalf("rate=%d day=%s err=%v", rate, day, err)
	}
	if len(rates.calls) != 3 {
		t.Fatalf("calls=%v", rates.calls)
	}
	if _, _, err := svc.rateFor(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if len(rates.calls) != 5 {
		t.Fatalf("Friday cache should avoid third provider call: %v", rates.calls)
	}
}

func TestRateForRefusesElevenUnpublishedDays(t *testing.T) {
	rates := &fakeRates{values: map[string]int64{}}
	svc := NewService(emptyStore{}, stubProvider{}, rates, nil, nil, nil, nil)
	_, _, err := svc.rateFor(context.Background(), time.Date(2026, 9, 7, 12, 0, 0, 0, seoul))
	if err == nil || len(rates.calls) != 11 {
		t.Fatalf("calls=%d err=%v", len(rates.calls), err)
	}
}

func TestDisabledServiceStillReadsButWillNotQuote(t *testing.T) {
	svc := NewService(emptyStore{}, nil, nil, nil, nil, nil, nil)
	view, err := svc.GetMyBilling(context.Background(), "alice")
	if err != nil || view.CustomerKey != CustomerKey("alice") {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	if _, err := svc.QuotePrice(context.Background(), plan.Basic, TermMonthly); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

type emptyStore struct{}

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

type stubProvider struct{}

func (stubProvider) IssueBillingKey(context.Context, string, string) (BillingKey, error) {
	return BillingKey{}, nil
}
func (stubProvider) Charge(context.Context, ChargeRequest) (Payment, error) { return Payment{}, nil }
func (stubProvider) PaymentByOrder(context.Context, string) (Payment, bool, error) {
	return Payment{}, false, nil
}
func (stubProvider) Refund(context.Context, string, string) error { return nil }
func (stubProvider) ParseNotification(*http.Request) (Notification, error) {
	return Notification{}, nil
}
