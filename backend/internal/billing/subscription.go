package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

func (s *Service) AnchorFor(ctx context.Context, userID string) (time.Time, bool, error) {
	subscription, found, err := s.store.Subscription(ctx, userID)
	if err != nil || !found || subscription.Status != "active" {
		return time.Time{}, false, err
	}
	return subscription.AnchorAt, true, nil
}

// CoverageAt is the subscription owner's answer for a paid benefit instant. The daily
// tier is resolved at that day's original boundary, not from the current user row.
func (s *Service) CoverageAt(ctx context.Context, userID string, at time.Time) (Coverage, bool, error) {
	support, assigned, err := s.store.SupportCoverage(ctx, userID)
	if err != nil {
		return Coverage{}, false, err
	}
	if assigned && !at.Before(support.Anchor) {
		dailyStart, _ := plan.DailyWindow(support.Anchor, at)
		dailyTier, err := s.store.TierAt(ctx, userID, support.ID, dailyStart)
		if err != nil {
			return Coverage{}, false, err
		}
		return Coverage{ID: support.ID, Anchor: support.Anchor,
			Tier: support.Tier, DailyTier: dailyTier}, true, nil
	}
	sub, found, err := s.store.Subscription(ctx, userID)
	if err != nil {
		return Coverage{}, false, err
	}
	if !found || sub.Status != "active" || at.Before(sub.AnchorAt) || !at.Before(sub.TermEnd) {
		return Coverage{}, false, nil
	}
	dailyStart, _ := plan.DailyWindow(sub.AnchorAt, at)
	dailyTier, err := s.store.TierAt(ctx, userID, sub.CoverageID, dailyStart)
	if err != nil {
		return Coverage{}, false, err
	}
	return Coverage{ID: sub.CoverageID, Anchor: sub.AnchorAt, End: sub.TermEnd,
		Tier: sub.Tier, DailyTier: dailyTier}, true, nil
}

// RunDue advances every subscription that was due when the pass started. Each account is
// caught up window by window; a failure is retained while the loop continues to later rows.
// A failed order reconciliation, unapplied-capture refund or refund reconciliation is logged
// and never stops the renewals. Captures are refunded before the renewals, so an account whose
// order left review renews on the same pass.
func (s *Service) RunDue(ctx context.Context, now time.Time) error {
	if !s.Enabled() {
		return ErrUnavailable
	}
	if err := s.ReconcilePending(ctx); err != nil {
		slog.Error("pending order reconciliation failed", "err", err)
	}
	if err := s.RefundUnappliedCaptures(ctx); err != nil {
		slog.Error("unapplied capture refund failed", "err", err)
	}
	if err := s.ReconcilePendingRefunds(ctx); err != nil {
		slog.Error("pending refund reconciliation failed", "err", err)
	}
	if err := s.purgeExpiredQuotes(ctx, now); err != nil {
		slog.Error("expired quote purge failed", "err", err)
	}
	due, err := s.store.DueSubscriptions(ctx, now)
	if err != nil {
		return err
	}
	var failures []error
	for _, subscription := range due {
		for subscription.Status == "active" && !subscription.NextGrantAt.After(now) {
			updated, stepErr := s.runDueStep(ctx, subscription, now)
			if stepErr != nil {
				if errors.Is(stepErr, ErrPaymentPending) {
					break
				}
				failures = append(failures, fmt.Errorf("%s: %w", subscription.UserID, stepErr))
				break
			}
			subscription = updated
		}
	}
	return errors.Join(failures...)
}

func (s *Service) runDueStep(ctx context.Context, subscription Subscription, now time.Time) (Subscription, error) {
	if subscription.TermEnd.After(now) {
		return s.grantAnnualWindow(ctx, subscription)
	}
	// A master account is never charged (BILL-20): a subscription it still holds from before
	// its promotion ends here as a cancellation would, whatever its auto-renew flag says.
	master, err := s.masterAccount(ctx, subscription.UserID)
	if err != nil {
		return subscription, err
	}
	if master {
		return s.lapseCancelled(ctx, subscription, now, OperatorSubscriptionEndedMail(subscription.Tier, subscription.Term))
	}
	if !subscription.AutoRenew {
		return s.lapseCancelled(ctx, subscription, now, CancellationMail(subscription.Tier, subscription.Term))
	}
	return s.renew(ctx, subscription, now)
}

// grantAnnualWindow moves a mid-term row's benefit boundary and nothing else. The row this
// pass read may be stale by now — a cancel, refund lapse or upgrade can land between
// DueSubscriptions and this step — so the write is conditional on the boundary it read and
// the loop continues from a fresh read rather than from the snapshot.
func (s *Service) grantAnnualWindow(ctx context.Context, subscription Subscription) (Subscription, error) {
	_, end := plan.BenefitWindow(subscription.AnchorAt, subscription.NextGrantAt)
	var advanced bool
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		var err error
		advanced, err = tx.AdvanceNextGrant(ctx, subscription.UserID, subscription.NextGrantAt, end)
		return err
	})
	if err != nil {
		return subscription, err
	}
	current, found, err := s.store.Subscription(ctx, subscription.UserID)
	if err != nil {
		return subscription, err
	}
	if !found {
		return Subscription{UserID: subscription.UserID}, nil
	}
	if !advanced && current.NextGrantAt.Equal(subscription.NextGrantAt) {
		// No writer moved the boundary yet the update matched nothing: stop here rather
		// than retry the same step forever.
		return current, fmt.Errorf("next grant of %s did not advance from %s", subscription.UserID, subscription.NextGrantAt)
	}
	return current, nil
}

// lapseCancelled ends a subscription at its term end without a charge. Its tier write goes
// through AssignTier, which leaves a master account on master (QUOTA-63).
func (s *Service) lapseCancelled(ctx context.Context, subscription Subscription, now time.Time, notice MailMessage) (Subscription, error) {
	updated := subscription
	updated.Status = "lapsed"
	updated.UpdatedAt = now
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		if err := tx.UpsertSubscription(ctx, updated); err != nil {
			return err
		}
		if support, assigned, err := tx.SupportCoverage(ctx, subscription.UserID); err != nil {
			return err
		} else if assigned {
			if err := plans.AssignTier(ctx, subscription.UserID, support.Tier); err != nil {
				return err
			}
		} else if err := plans.AssignTier(ctx, subscription.UserID, plan.Free); err != nil {
			return err
		}
		return tx.InsertEvent(ctx, Event{UserID: subscription.UserID, Kind: "cancelled", Tier: &subscription.Tier, Term: &subscription.Term, CreatedAt: now})
	})
	if err != nil {
		return subscription, err
	}
	if err := s.sendMail(ctx, subscription.UserID, notice); err != nil {
		return updated, err
	}
	return updated, nil
}

// offerQuote is the fixed VAT-inclusive KRW price of a tier and term (BILL-2, BILL-4).
func offerQuote(tier plan.Plan, term Term) (Quote, error) {
	offer, ok := plan.CommercialOffer(tier)
	if !ok || !billableTier(tier) || !term.Valid() {
		return Quote{}, ErrTierNotSubscribable
	}
	krw := offer.MonthlyKRW
	if term == TermAnnual {
		krw = offer.AnnualKRW
	}
	return Quote{KRW: krw}, nil
}

func (s *Service) sendMail(ctx context.Context, userID string, message MailMessage) error {
	if s.accounts == nil || s.mailer == nil {
		return nil
	}
	email, ok, err := s.accounts.VerifiedEmail(ctx, userID)
	if err != nil {
		return err
	}
	if !ok || email == "" {
		slog.Info("billing mail skipped without verified email", "user_id", userID, "subject", message.Subject)
		return nil
	}
	return s.mailer.Send(ctx, email, message.Subject, message.Text)
}

func billableTier(tier plan.Plan) bool {
	return tier == plan.Light || tier == plan.Basic || tier == plan.Pro || tier == plan.Max
}
