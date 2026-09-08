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
	newID    func() string

	rateMu    sync.Mutex
	rateCache map[string]int64
}

func NewService(store Store, provider Provider, rates Rates, credits Credits, plans Plans, accounts Accounts, mailer Mailer) *Service {
	return &Service{
		store: store, provider: provider, rates: rates, credits: credits,
		plans: plans, accounts: accounts, mailer: mailer, now: time.Now,
		newID:     newID,
		rateCache: make(map[string]int64),
	}
}

func (s *Service) Enabled() bool { return s.provider != nil && s.rates != nil }

// markRefundable fills in the refund button's state for a whole screen.
//
// One query for every in-window purchase rather than one per purchase: the per-purchase read
// runs on the single writer (ARCH-10), so a read-only screen used to queue N statements ahead
// of every concurrent generation hold. An account with nothing in window asks nothing.
func (s *Service) markRefundable(ctx context.Context, purchases []Purchase, now time.Time) error {
	if s.credits == nil {
		return nil
	}
	candidates := make([]string, 0, len(purchases))
	for _, purchase := range purchases {
		if refundableWindow(purchase, now) {
			candidates = append(candidates, purchase.LotID)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	untouched, err := s.credits.UntouchedLots(ctx, candidates)
	if err != nil {
		return err
	}
	for index := range purchases {
		purchase := &purchases[index]
		purchase.Refundable = refundableWindow(*purchase, now) && untouched[purchase.LotID]
	}
	return nil
}

// refundableWindow is everything about refundability that the purchase row alone answers:
// it has not been refunded and it is still inside the seven days.
func refundableWindow(purchase Purchase, now time.Time) bool {
	return purchase.RefundedAt == nil && now.Before(purchase.ChargedAt.Add(refundWindow))
}

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
	if err := s.markRefundable(ctx, purchases, s.now()); err != nil {
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

func (s *Service) RegisterPaymentMethod(ctx context.Context, userID, authKey, customerKey string) (PaymentMethodRegistration, error) {
	if !s.Enabled() {
		return PaymentMethodRegistration{}, ErrUnavailable
	}
	email, verified, err := s.accounts.VerifiedEmail(ctx, userID)
	if err != nil {
		return PaymentMethodRegistration{}, err
	}
	if !verified || email == "" {
		return PaymentMethodRegistration{}, ErrEmailVerificationRequired
	}
	expectedKey := CustomerKey(userID)
	if customerKey != expectedKey {
		return PaymentMethodRegistration{}, ErrCustomerKeyMismatch
	}
	issued, err := s.provider.IssueBillingKey(ctx, authKey, customerKey)
	if err != nil {
		return PaymentMethodRegistration{}, err
	}
	if issued.CustomerKey != expectedKey {
		return PaymentMethodRegistration{}, ErrCustomerKeyMismatch
	}
	now := s.now()
	method := PaymentMethod{
		UserID: userID, Provider: "toss", BillingKey: issued.Value,
		CustomerKey: expectedKey, CardLabel: issued.CardLabel, RegisteredAt: now,
	}
	result := PaymentMethodRegistration{PaymentMethod: method}
	err = s.store.InWriteTx(ctx, func(tx Store, credits Credits, _ Plans) error {
		if err := tx.UpsertPaymentMethod(ctx, method); err != nil {
			return err
		}
		note := issued.CardLabel
		if err := tx.InsertEvent(ctx, Event{UserID: userID, Kind: "method_registered", Note: &note, CreatedAt: now}); err != nil {
			return err
		}
		created, err := credits.GrantBonusOnce(
			ctx, "payment-method-bonus:"+userID, userID, plan.PaymentMethodBonusCredits,
		)
		if err != nil {
			return err
		}
		if created {
			amount := plan.PaymentMethodBonusCredits
			if err := tx.InsertEvent(ctx, Event{UserID: userID, Kind: "grant", Credits: &amount, CreatedAt: now}); err != nil {
				return err
			}
		}
		result.BonusGranted = created
		return nil
	})
	if err != nil {
		return PaymentMethodRegistration{}, err
	}
	return result, nil
}

func (s *Service) RemovePaymentMethod(ctx context.Context, userID string) error {
	if !s.Enabled() {
		return ErrUnavailable
	}
	subscription, found, err := s.store.Subscription(ctx, userID)
	if err != nil {
		return err
	}
	if found && subscription.Status == "active" && subscription.AutoRenew {
		return ErrSubscriptionNeedsMethod
	}
	return s.store.DeletePaymentMethod(ctx, userID)
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
	return s.quoteAt(ctx, tier, term, s.now())
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
