package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

func (s *Service) Subscribe(ctx context.Context, userID string, tier plan.Plan, term Term) (Subscription, error) {
	if s.fixedKRW {
		return s.subscribeFixed(ctx, userID, tier, term)
	}
	if !s.Enabled() {
		return Subscription{}, ErrUnavailable
	}
	if master, err := s.masterAccount(ctx, userID); err != nil {
		return Subscription{}, err
	} else if master {
		return Subscription{}, ErrMasterAccount
	}
	if !billableTier(tier) {
		return Subscription{}, ErrTierNotSubscribable
	}
	if !term.Valid() {
		return Subscription{}, fmt.Errorf("invalid subscription term %q", term)
	}
	if current, found, err := s.store.Subscription(ctx, userID); err != nil {
		return Subscription{}, err
	} else if found && current.Status == "active" {
		return Subscription{}, ErrSubscriptionExists
	}
	method, found, err := s.store.PaymentMethod(ctx, userID)
	if err != nil {
		return Subscription{}, err
	}
	if !found {
		return Subscription{}, ErrPaymentMethodRequired
	}

	now := s.now()
	quote, err := s.quoteAt(ctx, tier, term, now)
	if err != nil {
		return Subscription{}, err
	}
	orderID := subscriptionOrderID(userID, now)
	payment, err := s.chargeSubscription(ctx, method, tier, term, quote, orderID)
	if err != nil {
		if eventErr := s.recordChargeFailure(ctx, userID, tier, term, quote, orderID, now); eventErr != nil {
			return Subscription{}, errors.Join(ErrChargeFailed, err, eventErr)
		}
		return Subscription{}, errors.Join(ErrChargeFailed, err)
	}

	_, next := plan.BenefitWindow(now, now)
	coverageID := "paid:" + userID + ":" + now.UTC().Format(time.RFC3339Nano)
	subscription := Subscription{
		UserID: userID, CoverageID: coverageID, Tier: tier, Term: term, AnchorAt: now, TermStart: now,
		TermEnd: TermEnd(now, now, term), NextGrantAt: next, AutoRenew: true,
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	err = s.store.InWriteTx(ctx, func(tx Store, credits Credits, plans Plans) error {
		if err := tx.UpsertSubscription(ctx, subscription); err != nil {
			return err
		}
		if err := tx.InsertEvent(ctx, chargeEvent(userID, tier, term, quote, payment, orderID, now)); err != nil {
			return err
		}
		if err := tx.InsertEvent(ctx, Event{UserID: userID, Kind: "tier_change", Tier: &tier, Term: &term, CreatedAt: now}); err != nil {
			return err
		}
		if err := plans.AssignTier(ctx, userID, tier); err != nil {
			return err
		}
		if err := tx.DeleteSupportCoverage(ctx, userID); err != nil {
			return err
		}
		if err := tx.InsertTierTransition(ctx, userID, coverageID, now, tier, orderID); err != nil {
			return err
		}
		return credits.OpenCoverage(ctx, userID, Coverage{ID: coverageID, Anchor: now,
			End: subscription.TermEnd, Tier: tier, DailyTier: tier}, now, orderID)
	})
	if err != nil {
		return Subscription{}, err
	}
	return subscription, nil
}

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
// A failed order or refund reconciliation is logged and never stops the renewals.
func (s *Service) RunDue(ctx context.Context, now time.Time) error {
	if !s.Enabled() {
		return ErrUnavailable
	}
	if s.fixedKRW {
		if err := s.ReconcilePending(ctx); err != nil {
			slog.Error("pending order reconciliation failed", "err", err)
		}
		if err := s.ReconcilePendingRefunds(ctx); err != nil {
			slog.Error("pending refund reconciliation failed", "err", err)
		}
		if err := s.purgeExpiredQuotes(ctx, now); err != nil {
			slog.Error("expired quote purge failed", "err", err)
		}
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

func (s *Service) renew(ctx context.Context, subscription Subscription, now time.Time) (Subscription, error) {
	if s.fixedKRW {
		return s.renewFixed(ctx, subscription, now)
	}
	tier, term := subscription.Tier, subscription.Term
	if subscription.ScheduledTier != nil {
		tier = *subscription.ScheduledTier
	}
	if subscription.ScheduledTerm != nil {
		term = *subscription.ScheduledTerm
	}
	quote, err := s.quoteAt(ctx, tier, term, now)
	if err != nil {
		return subscription, err
	}
	method, found, err := s.store.PaymentMethod(ctx, subscription.UserID)
	if err != nil {
		return subscription, err
	}
	start := subscription.TermEnd
	orderID := subscriptionOrderID(subscription.UserID, start)
	var payment Payment
	if found {
		payment, err = s.chargeSubscription(ctx, method, tier, term, quote, orderID)
	} else {
		err = ErrPaymentMethodRequired
	}
	if err != nil {
		return s.lapseFailedRenewal(ctx, subscription, tier, term, quote, orderID, now)
	}

	updated := subscription
	updated.Tier = tier
	updated.Term = term
	updated.TermStart = start
	updated.TermEnd = TermEnd(subscription.AnchorAt, start, term)
	_, updated.NextGrantAt = plan.BenefitWindow(subscription.AnchorAt, start)
	updated.ScheduledTier = nil
	updated.ScheduledTerm = nil
	updated.UpdatedAt = now
	err = s.store.InWriteTx(ctx, func(tx Store, credits Credits, plans Plans) error {
		_, supportAssigned, err := tx.SupportCoverage(ctx, subscription.UserID)
		if err != nil {
			return err
		}
		if err := tx.UpsertSubscription(ctx, updated); err != nil {
			return err
		}
		if err := tx.InsertEvent(ctx, chargeEvent(subscription.UserID, tier, term, quote, payment, orderID, now)); err != nil {
			return err
		}
		if tier != subscription.Tier {
			if err := tx.InsertEvent(ctx, Event{UserID: subscription.UserID, Kind: "tier_change", Tier: &tier, Term: &term, CreatedAt: now}); err != nil {
				return err
			}
			if !supportAssigned {
				if err := plans.AssignTier(ctx, subscription.UserID, tier); err != nil {
					return err
				}
			}
		}
		if supportAssigned {
			return nil
		}
		if tier != subscription.Tier {
			if err := tx.InsertTierTransition(ctx, subscription.UserID, subscription.CoverageID, start, tier, orderID); err != nil {
				return err
			}
		}
		dailyStart, _ := plan.DailyWindow(subscription.AnchorAt, start)
		dailyTier := subscription.Tier
		if !dailyStart.Before(start) {
			dailyTier = tier
		}
		return credits.OpenCoverage(ctx, subscription.UserID, Coverage{ID: subscription.CoverageID,
			Anchor: subscription.AnchorAt, End: updated.TermEnd, Tier: tier, DailyTier: dailyTier}, start, orderID)
	})
	if err != nil {
		return subscription, err
	}
	if err := s.sendMail(ctx, subscription.UserID, RenewalMail(tier, term, quote)); err != nil {
		return updated, err
	}
	return updated, nil
}

func (s *Service) lapseFailedRenewal(ctx context.Context, subscription Subscription, tier plan.Plan, term Term, quote Quote, orderID string, now time.Time) (Subscription, error) {
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
		note := orderID
		return tx.InsertEvent(ctx, Event{UserID: subscription.UserID, Kind: "renewal_failed", Tier: &tier, Term: &term, USDCents: &quote.USDCents, KRWPerUSDE4: &quote.RatePerUSDE4, RateDate: &quote.RateDate, KRW: &quote.KRW, Note: &note, CreatedAt: now})
	})
	if err != nil {
		return subscription, err
	}
	if err := s.sendMail(ctx, subscription.UserID, RenewalFailedMail(tier, term, quote)); err != nil {
		return updated, err
	}
	return updated, nil
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

func (s *Service) quoteAt(ctx context.Context, tier plan.Plan, term Term, now time.Time) (Quote, error) {
	if s.fixedKRW {
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
	rate, date, err := s.rateFor(ctx, now)
	if err != nil {
		return Quote{}, err
	}
	usd := PriceCents(tier, term)
	return Quote{USDCents: usd, KRW: KRWFor(usd, rate), RatePerUSDE4: rate, RateDate: date}, nil
}

func (s *Service) chargeSubscription(ctx context.Context, method PaymentMethod, tier plan.Plan, term Term, quote Quote, orderID string) (Payment, error) {
	if payment, found, err := s.provider.PaymentByOrder(ctx, orderID); err != nil {
		return Payment{}, err
	} else if found {
		if payment.Status == "DONE" {
			return payment, nil
		}
		return Payment{}, fmt.Errorf("order %s has provider status %s", orderID, payment.Status)
	}
	payment, err := s.provider.Charge(ctx, ChargeRequest{
		BillingKey: method.BillingKey, CustomerKey: method.CustomerKey, OrderID: orderID,
		KRW: quote.KRW, Name: fmt.Sprintf("Postpilot %s %s subscription", tier, term),
	})
	if err != nil {
		return Payment{}, err
	}
	if payment.Status != "DONE" {
		return Payment{}, fmt.Errorf("order %s has provider status %s", orderID, payment.Status)
	}
	return payment, nil
}

func (s *Service) recordChargeFailure(ctx context.Context, userID string, tier plan.Plan, term Term, quote Quote, orderID string, now time.Time) error {
	note := orderID
	return s.store.InsertEvent(ctx, Event{UserID: userID, Kind: "charge_failed", Tier: &tier, Term: &term, USDCents: &quote.USDCents, KRWPerUSDE4: &quote.RatePerUSDE4, RateDate: &quote.RateDate, KRW: &quote.KRW, Note: &note, CreatedAt: now})
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

func chargeEvent(userID string, tier plan.Plan, term Term, quote Quote, payment Payment, orderID string, now time.Time) Event {
	return Event{UserID: userID, Kind: "charge", Tier: &tier, Term: &term, USDCents: &quote.USDCents, KRWPerUSDE4: &quote.RatePerUSDE4, RateDate: &quote.RateDate, KRW: &quote.KRW, ProviderPaymentKey: &payment.PaymentKey, OrderID: &orderID, CreatedAt: now}
}

func subscriptionOrderID(userID string, start time.Time) string {
	return "sub:" + userID + ":" + start.UTC().Format(time.RFC3339Nano)
}

func billableTier(tier plan.Plan) bool {
	return tier == plan.Light || tier == plan.Basic || tier == plan.Pro || tier == plan.Max
}
