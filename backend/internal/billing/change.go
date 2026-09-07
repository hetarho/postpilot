package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

type changeKind int

const (
	changeNone changeKind = iota
	changeUpgrade
	changeScheduled
)

func (s *Service) QuoteChange(ctx context.Context, userID string, tier plan.Plan, term Term) (ChangeQuote, error) {
	if !s.Enabled() {
		return ChangeQuote{}, ErrUnavailable
	}
	now := s.now()
	subscription, kind, err := s.classifyChange(ctx, userID, tier, term, now)
	if err != nil {
		return ChangeQuote{}, err
	}
	return s.quoteChangeAt(ctx, subscription, tier, term, kind, now)
}

func (s *Service) ChangeSubscription(ctx context.Context, userID string, tier plan.Plan, term Term) (Subscription, bool, error) {
	if !s.Enabled() {
		return Subscription{}, false, ErrUnavailable
	}
	now := s.now()
	subscription, kind, err := s.classifyChange(ctx, userID, tier, term, now)
	if err != nil {
		return Subscription{}, false, err
	}
	if kind == changeScheduled {
		updated := subscription
		updated.ScheduledTier = &tier
		updated.ScheduledTerm = &term
		updated.AutoRenew = true
		updated.UpdatedAt = now
		err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
			if err := tx.UpsertSubscription(ctx, updated); err != nil {
				return err
			}
			return tx.InsertEvent(ctx, Event{UserID: userID, Kind: "change_scheduled", Tier: &tier, Term: &term, CreatedAt: now})
		})
		return updated, false, err
	}

	quote, err := s.quoteChangeAt(ctx, subscription, tier, term, kind, now)
	if err != nil {
		return Subscription{}, false, err
	}
	orderID := upgradeOrderID(userID, now)
	var payment Payment
	if quote.USDCents > 0 {
		method, found, err := s.store.PaymentMethod(ctx, userID)
		if err != nil {
			return Subscription{}, false, err
		}
		if !found {
			return Subscription{}, false, ErrPaymentMethodRequired
		}
		payment, err = s.chargeSubscription(ctx, method, tier, term, quote.Quote, orderID)
		if err != nil {
			if eventErr := s.recordChargeFailure(ctx, userID, tier, term, quote.Quote, orderID, now); eventErr != nil {
				return Subscription{}, false, errors.Join(ErrChargeFailed, err, eventErr)
			}
			return Subscription{}, false, errors.Join(ErrChargeFailed, err)
		}
	}

	updated := subscription
	updated.Tier = tier
	updated.ScheduledTier = nil
	updated.ScheduledTerm = nil
	updated.UpdatedAt = now
	delta := plan.MonthlyCredits(tier) - plan.MonthlyCredits(subscription.Tier)
	err = s.store.InWriteTx(ctx, func(tx Store, credits Credits, plans Plans) error {
		if err := tx.UpsertSubscription(ctx, updated); err != nil {
			return err
		}
		if quote.USDCents > 0 {
			if err := tx.InsertEvent(ctx, chargeEvent(userID, tier, term, quote.Quote, payment, orderID, now)); err != nil {
				return err
			}
		}
		if err := tx.InsertEvent(ctx, Event{UserID: userID, Kind: "tier_change", Tier: &tier, Term: &term, CreatedAt: now}); err != nil {
			return err
		}
		if err := plans.AssignTier(ctx, userID, tier); err != nil {
			return err
		}
		return credits.RaiseMonthlyLot(ctx, userID, delta)
	})
	return updated, true, err
}

func (s *Service) CancelScheduledChange(ctx context.Context, userID string) (Subscription, error) {
	now := s.now()
	subscription, err := s.requireActiveSubscription(ctx, userID, now)
	if err != nil {
		return Subscription{}, err
	}
	if subscription.ScheduledTier == nil && subscription.ScheduledTerm == nil {
		return Subscription{}, ErrNoScheduledChange
	}
	subscription.ScheduledTier = nil
	subscription.ScheduledTerm = nil
	subscription.UpdatedAt = now
	return subscription, s.store.UpsertSubscription(ctx, subscription)
}

func (s *Service) CancelSubscription(ctx context.Context, userID string) (Subscription, error) {
	now := s.now()
	subscription, err := s.requireActiveSubscription(ctx, userID, now)
	if err != nil {
		return Subscription{}, err
	}
	if !subscription.AutoRenew {
		return Subscription{}, ErrNoChange
	}
	subscription.AutoRenew = false
	subscription.ScheduledTier = nil
	subscription.ScheduledTerm = nil
	subscription.UpdatedAt = now
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		if err := tx.UpsertSubscription(ctx, subscription); err != nil {
			return err
		}
		return tx.InsertEvent(ctx, Event{UserID: userID, Kind: "cancel_scheduled", Tier: &subscription.Tier, Term: &subscription.Term, CreatedAt: now})
	})
	return subscription, err
}

func (s *Service) ResumeSubscription(ctx context.Context, userID string) (Subscription, error) {
	now := s.now()
	subscription, err := s.requireActiveSubscription(ctx, userID, now)
	if err != nil {
		return Subscription{}, err
	}
	if subscription.AutoRenew {
		return Subscription{}, ErrNoChange
	}
	subscription.AutoRenew = true
	subscription.UpdatedAt = now
	return subscription, s.store.UpsertSubscription(ctx, subscription)
}

func (s *Service) classifyChange(ctx context.Context, userID string, tier plan.Plan, term Term, now time.Time) (Subscription, changeKind, error) {
	if !billableTier(tier) {
		return Subscription{}, changeNone, ErrTierNotSubscribable
	}
	if !term.Valid() {
		return Subscription{}, changeNone, fmt.Errorf("invalid subscription term %q", term)
	}
	subscription, err := s.requireActiveSubscription(ctx, userID, now)
	if err != nil {
		return Subscription{}, changeNone, err
	}
	tierChanged := subscription.Tier != tier
	termChanged := subscription.Term != term
	switch {
	case !tierChanged && !termChanged:
		return Subscription{}, changeNone, ErrNoChange
	case tierChanged && termChanged:
		return Subscription{}, changeNone, ErrChangeUnsupported
	case tierChanged && tier.Rank() > subscription.Tier.Rank():
		return subscription, changeUpgrade, nil
	default:
		return subscription, changeScheduled, nil
	}
}

func (s *Service) requireActiveSubscription(ctx context.Context, userID string, now time.Time) (Subscription, error) {
	subscription, found, err := s.store.Subscription(ctx, userID)
	if err != nil {
		return Subscription{}, err
	}
	if !found || subscription.Status != "active" || !now.Before(subscription.TermEnd) {
		return Subscription{}, ErrSubscriptionRequired
	}
	return subscription, nil
}

func (s *Service) quoteChangeAt(ctx context.Context, subscription Subscription, tier plan.Plan, term Term, kind changeKind, now time.Time) (ChangeQuote, error) {
	quote, err := s.quoteAt(ctx, tier, term, now)
	if err != nil {
		return ChangeQuote{}, err
	}
	result := ChangeQuote{Quote: quote, AppliedNow: kind == changeUpgrade, EffectiveAt: subscription.TermEnd}
	if kind != changeUpgrade {
		return result, nil
	}
	difference := plan.MonthlyPriceCents(tier) - plan.MonthlyPriceCents(subscription.Tier)
	if term == TermAnnual {
		difference *= remainingWholeMonths(subscription.AnchorAt, subscription.TermEnd, now)
	}
	result.USDCents = difference
	result.KRW = KRWFor(difference, result.RatePerUSDE4)
	result.EffectiveAt = now
	return result, nil
}

func remainingWholeMonths(anchor, termEnd, now time.Time) int {
	months := 0
	for boundary := plan.NextRenewal(anchor, now); boundary.Before(termEnd); boundary = plan.NextRenewal(anchor, boundary) {
		months++
	}
	return months
}

func upgradeOrderID(userID string, now time.Time) string {
	return "upg:" + userID + ":" + now.UTC().Format(time.RFC3339)
}
