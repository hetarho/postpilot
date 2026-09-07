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
	if a, b := CustomerKey("alice"), CustomerKey("alice"); a != b || len(a) != 64 || a == CustomerKey("bob") {
		t.Fatalf("unstable customer key %q", a)
	}
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

func (emptyStore) InWriteTx(ctx context.Context, fn func(Store) error) error { return fn(emptyStore{}) }
func (emptyStore) Subscription(context.Context, string) (Subscription, bool, error) {
	return Subscription{}, false, nil
}
func (emptyStore) PaymentMethod(context.Context, string) (PaymentMethod, bool, error) {
	return PaymentMethod{}, false, nil
}
func (emptyStore) Events(context.Context, string, int) ([]Event, error)                   { return nil, nil }
func (emptyStore) Purchases(context.Context, string) ([]Purchase, error)                  { return nil, nil }
func (emptyStore) InsertProviderNotification(context.Context, ProviderNotification) error { return nil }

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
