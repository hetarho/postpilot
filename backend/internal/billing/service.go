package billing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

var ErrUnavailable = errors.New("billing unavailable")

var seoul = time.FixedZone("Asia/Seoul", 9*60*60)

type Service struct {
	store    Store
	provider Provider
	rates    Rates
	credits  Credits
	plans    Plans
	accounts Accounts
	mailer   Mailer
	now      func() time.Time

	rateMu    sync.Mutex
	rateCache map[string]int64
}

func NewService(store Store, provider Provider, rates Rates, credits Credits, plans Plans, accounts Accounts, mailer Mailer) *Service {
	return &Service{
		store: store, provider: provider, rates: rates, credits: credits,
		plans: plans, accounts: accounts, mailer: mailer, now: time.Now,
		rateCache: make(map[string]int64),
	}
}

func (s *Service) Enabled() bool { return s.provider != nil && s.rates != nil }

func (s *Service) GetMyBilling(ctx context.Context, userID string) (AccountBilling, error) {
	subscription, hasSubscription, err := s.store.Subscription(ctx, userID)
	if err != nil {
		return AccountBilling{}, err
	}
	method, hasMethod, err := s.store.PaymentMethod(ctx, userID)
	if err != nil {
		return AccountBilling{}, err
	}
	history, err := s.store.Events(ctx, userID, 100)
	if err != nil {
		return AccountBilling{}, err
	}
	purchases, err := s.store.Purchases(ctx, userID)
	if err != nil {
		return AccountBilling{}, err
	}
	result := AccountBilling{History: history, Purchases: purchases, CustomerKey: CustomerKey(userID)}
	if hasSubscription {
		result.Subscription = &subscription
	}
	if hasMethod {
		result.PaymentMethod = &method
	}
	return result, nil
}

func (s *Service) QuotePrice(ctx context.Context, tier plan.Plan, term Term) (Quote, error) {
	if !s.Enabled() {
		return Quote{}, ErrUnavailable
	}
	if tier != plan.Basic && tier != plan.Pro && tier != plan.Max {
		return Quote{}, fmt.Errorf("tier %q is not billable", tier)
	}
	if !term.Valid() {
		return Quote{}, fmt.Errorf("term %q is invalid", term)
	}
	rate, date, err := s.rateFor(ctx, s.now())
	if err != nil {
		return Quote{}, err
	}
	usd := PriceCents(tier, term)
	return Quote{USDCents: usd, KRW: KRWFor(usd, rate), RatePerUSDE4: rate, RateDate: date}, nil
}

// rateFor starts at the previous Seoul date because today's reference rate may not be final.
// Ten fallbacks means eleven candidate dates in total: yesterday plus ten older dates.
func (s *Service) rateFor(ctx context.Context, now time.Time) (int64, string, error) {
	if s.rates == nil {
		return 0, "", ErrUnavailable
	}
	day := now.In(seoul)
	day = time.Date(day.Year(), day.Month(), day.Day()-1, 0, 0, 0, 0, seoul)
	for attempts := 0; attempts <= 10; attempts++ {
		key := day.Format(time.DateOnly)
		s.rateMu.Lock()
		cached, ok := s.rateCache[key]
		s.rateMu.Unlock()
		if ok {
			return cached, key, nil
		}
		rate, published, err := s.rates.KRWPerUSD(ctx, day)
		if err != nil {
			return 0, "", fmt.Errorf("read exchange rate for %s: %w", key, err)
		}
		if published {
			if rate <= 0 {
				return 0, "", fmt.Errorf("exchange rate for %s is not positive", key)
			}
			s.rateMu.Lock()
			s.rateCache[key] = rate
			s.rateMu.Unlock()
			return rate, key, nil
		}
		day = day.AddDate(0, 0, -1)
	}
	return 0, "", errors.New("no published KRW/USD rate in the previous 11 calendar days")
}
