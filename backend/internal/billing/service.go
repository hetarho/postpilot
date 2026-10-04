package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

var ErrUnavailable = errors.New("billing unavailable")

// Service charges the fixed VAT-inclusive KRW prices of the code-owned offer (BILL-2); FX
// never reaches a subscription or pack price.
type Service struct {
	store    Store
	provider Provider
	credits  Credits
	plans    Plans
	accounts Accounts
	mailer   Mailer
	now      func() time.Time
	newID    func() string
}

// refuseMaster is BILL-20's gate, read through a transaction's Plans port so the check and
// the write it guards see one snapshot.
func refuseMaster(ctx context.Context, plans Plans, userID string) error {
	tier, err := plans.TierOf(ctx, userID)
	if err != nil {
		return err
	}
	if tier == plan.Master {
		return ErrMasterAccount
	}
	return nil
}

// masterAccount reads the account's tier for a path that writes nothing itself (a quote, a
// renewal decision). Every billing store supplies Plans to its transactions, so this needs
// no separately wired collaborator.
func (s *Service) masterAccount(ctx context.Context, userID string) (bool, error) {
	var master bool
	err := s.store.InWriteTx(ctx, func(_ Store, _ Credits, plans Plans) error {
		err := refuseMaster(ctx, plans, userID)
		master = errors.Is(err, ErrMasterAccount)
		if master {
			return nil
		}
		return err
	})
	return master, err
}

func NewService(store Store, provider Provider, credits Credits, plans Plans, accounts Accounts, mailer Mailer) *Service {
	return &Service{
		store: store, provider: provider, credits: credits,
		plans: plans, accounts: accounts, mailer: mailer, now: time.Now,
		newID: newID,
	}
}

// Enabled reports whether payments can start: without a provider the service still reads
// billing and assigns support tiers, but charges nothing.
func (s *Service) Enabled() bool { return s.provider != nil }

func (s *Service) WithClock(now func() time.Time) *Service {
	if now != nil {
		s.now = now
	}
	return s
}

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
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		if err := tx.UpsertPaymentMethod(ctx, method); err != nil {
			return err
		}
		note := issued.CardLabel
		if err := tx.InsertEvent(ctx, Event{UserID: userID, Kind: "method_registered", Note: &note, CreatedAt: now}); err != nil {
			return err
		}
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
	return s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		if err := s.requireNoPending(ctx, tx, userID); err != nil {
			return err
		}
		sub, found, err := tx.Subscription(ctx, userID)
		if err != nil {
			return err
		}
		if found && sub.Status == "active" && sub.AutoRenew {
			return ErrSubscriptionNeedsMethod
		}
		return tx.DeletePaymentMethod(ctx, userID)
	})
}

func (s *Service) QuotePrice(ctx context.Context, tier plan.Plan, term Term) (Quote, error) {
	if !s.Enabled() {
		return Quote{}, ErrUnavailable
	}
	if !billableTier(tier) {
		return Quote{}, fmt.Errorf("tier %q is not billable", tier)
	}
	if !term.Valid() {
		return Quote{}, fmt.Errorf("term %q is invalid", term)
	}
	return offerQuote(tier, term)
}
