package billing

import (
	"context"
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
	if master, err := s.masterAccount(ctx, userID); err != nil {
		return ChangeQuote{}, err
	} else if master {
		return ChangeQuote{}, ErrMasterAccount
	}
	now := s.now()
	subscription, kind, err := s.classifyChange(ctx, userID, tier, term, now)
	if err != nil {
		return ChangeQuote{}, err
	}
	quote, err := s.quoteChangeAt(subscription, tier, term, kind, now)
	if err != nil {
		return quote, err
	}
	return s.persistFixedChangeQuote(ctx, userID, subscription, tier, term, quote, now)
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
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		if err := s.requireNoPending(ctx, tx, userID); err != nil {
			return err
		}
		current, found, err := tx.Subscription(ctx, userID)
		if err != nil {
			return err
		}
		if !found || !current.UpdatedAt.Equal(subscription.UpdatedAt) {
			return ErrStaleQuote
		}
		current.ScheduledTier, current.ScheduledTerm, current.UpdatedAt = nil, nil, now
		subscription = current
		return tx.UpsertSubscription(ctx, current)
	})
	return subscription, err
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
	// The row this cancel read, before the copy below is stamped with the cancel's own time.
	read := subscription.UpdatedAt
	subscription.AutoRenew = false
	subscription.ScheduledTier = nil
	subscription.ScheduledTerm = nil
	subscription.UpdatedAt = now
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		if err := s.requireNoPending(ctx, tx, userID); err != nil {
			return err
		}
		current, found, err := tx.Subscription(ctx, userID)
		if err != nil {
			return err
		}
		if !found || !current.UpdatedAt.Equal(read) {
			return ErrStaleQuote
		}
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
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		if err := s.requireNoPending(ctx, tx, userID); err != nil {
			return err
		}
		current, found, err := tx.Subscription(ctx, userID)
		if err != nil {
			return err
		}
		if !found || !current.UpdatedAt.Equal(subscription.UpdatedAt) {
			return ErrStaleQuote
		}
		current.AutoRenew, current.UpdatedAt = true, now
		subscription = current
		return tx.UpsertSubscription(ctx, current)
	})
	return subscription, err
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
		return subscription, changeScheduled, nil
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

// quoteChangeAt prices a change at now: a scheduled change quotes the full price it will
// charge at the term end, an upgrade its prorated difference (BILL-5, BILL-18).
func (s *Service) quoteChangeAt(subscription Subscription, tier plan.Plan, term Term, kind changeKind, now time.Time) (ChangeQuote, error) {
	quote, err := offerQuote(tier, term)
	if err != nil {
		return ChangeQuote{}, err
	}
	result := ChangeQuote{Quote: quote, AppliedNow: kind == changeUpgrade, EffectiveAt: subscription.TermEnd}
	if kind != changeUpgrade {
		return result, nil
	}
	benefitStart, benefitEnd := plan.BenefitWindow(subscription.AnchorAt, now)
	amounts, err := plan.QuoteUpgrade(subscription.Tier, tier, term == TermAnnual,
		subscription.TermStart, subscription.TermEnd, benefitStart, benefitEnd, now)
	if err != nil {
		return ChangeQuote{}, err
	}
	result.KRW = int(amounts.ChargeKRW)
	result.EffectiveAt = now
	return result, nil
}
