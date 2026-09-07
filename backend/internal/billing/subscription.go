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
	if !s.Enabled() {
		return Subscription{}, ErrUnavailable
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

	next := plan.NextRenewal(now, now)
	subscription := Subscription{
		UserID: userID, Tier: tier, Term: term, AnchorAt: now, TermStart: now,
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
		return credits.OpenMonthlyLot(ctx, userID, tier, now, next)
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

// RunDue advances every subscription that was due when the pass started. Each account is
// caught up window by window; a failure is retained while the loop continues to later rows.
func (s *Service) RunDue(ctx context.Context, now time.Time) error {
	if !s.Enabled() {
		return ErrUnavailable
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
		return s.grantAnnualWindow(ctx, subscription, now)
	}
	if !subscription.AutoRenew {
		return s.lapseCancelled(ctx, subscription, now)
	}
	return s.renew(ctx, subscription, now)
}

func (s *Service) grantAnnualWindow(ctx context.Context, subscription Subscription, now time.Time) (Subscription, error) {
	start := subscription.NextGrantAt
	end := plan.NextRenewal(subscription.AnchorAt, start)
	updated := subscription
	updated.NextGrantAt = end
	updated.UpdatedAt = now
	creditsGranted := plan.MonthlyCredits(subscription.Tier)
	err := s.store.InWriteTx(ctx, func(tx Store, credits Credits, _ Plans) error {
		if err := tx.UpsertSubscription(ctx, updated); err != nil {
			return err
		}
		if err := credits.OpenMonthlyLot(ctx, subscription.UserID, subscription.Tier, start, end); err != nil {
			return err
		}
		return tx.InsertEvent(ctx, Event{UserID: subscription.UserID, Kind: "grant", Tier: &subscription.Tier, Term: &subscription.Term, Credits: &creditsGranted, CreatedAt: now})
	})
	return updated, err
}

func (s *Service) renew(ctx context.Context, subscription Subscription, now time.Time) (Subscription, error) {
	quote, err := s.quoteAt(ctx, subscription.Tier, subscription.Term, now)
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
		payment, err = s.chargeSubscription(ctx, method, subscription.Tier, subscription.Term, quote, orderID)
	} else {
		err = ErrPaymentMethodRequired
	}
	if err != nil {
		return s.lapseFailedRenewal(ctx, subscription, quote, orderID, now)
	}

	updated := subscription
	updated.TermStart = start
	updated.TermEnd = TermEnd(subscription.AnchorAt, start, subscription.Term)
	updated.NextGrantAt = plan.NextRenewal(subscription.AnchorAt, start)
	updated.UpdatedAt = now
	err = s.store.InWriteTx(ctx, func(tx Store, credits Credits, _ Plans) error {
		if err := tx.UpsertSubscription(ctx, updated); err != nil {
			return err
		}
		if err := tx.InsertEvent(ctx, chargeEvent(subscription.UserID, subscription.Tier, subscription.Term, quote, payment, orderID, now)); err != nil {
			return err
		}
		return credits.OpenMonthlyLot(ctx, subscription.UserID, subscription.Tier, start, updated.NextGrantAt)
	})
	if err != nil {
		return subscription, err
	}
	if err := s.sendMail(ctx, subscription.UserID, RenewalMail(subscription.Tier, subscription.Term, quote)); err != nil {
		return updated, err
	}
	return updated, nil
}

func (s *Service) lapseFailedRenewal(ctx context.Context, subscription Subscription, quote Quote, orderID string, now time.Time) (Subscription, error) {
	updated := subscription
	updated.Status = "lapsed"
	updated.UpdatedAt = now
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		if err := tx.UpsertSubscription(ctx, updated); err != nil {
			return err
		}
		if err := plans.AssignTier(ctx, subscription.UserID, plan.Free); err != nil {
			return err
		}
		note := orderID
		return tx.InsertEvent(ctx, Event{UserID: subscription.UserID, Kind: "renewal_failed", Tier: &subscription.Tier, Term: &subscription.Term, USDCents: &quote.USDCents, KRWPerUSDE4: &quote.RatePerUSDE4, RateDate: &quote.RateDate, KRW: &quote.KRW, Note: &note, CreatedAt: now})
	})
	if err != nil {
		return subscription, err
	}
	if err := s.sendMail(ctx, subscription.UserID, RenewalFailedMail(subscription.Tier, subscription.Term, quote)); err != nil {
		return updated, err
	}
	return updated, nil
}

func (s *Service) lapseCancelled(ctx context.Context, subscription Subscription, now time.Time) (Subscription, error) {
	updated := subscription
	updated.Status = "lapsed"
	updated.UpdatedAt = now
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		if err := tx.UpsertSubscription(ctx, updated); err != nil {
			return err
		}
		if err := plans.AssignTier(ctx, subscription.UserID, plan.Free); err != nil {
			return err
		}
		return tx.InsertEvent(ctx, Event{UserID: subscription.UserID, Kind: "cancelled", Tier: &subscription.Tier, Term: &subscription.Term, CreatedAt: now})
	})
	if err != nil {
		return subscription, err
	}
	if err := s.sendMail(ctx, subscription.UserID, CancellationMail(subscription.Tier, subscription.Term)); err != nil {
		return updated, err
	}
	return updated, nil
}

func (s *Service) quoteAt(ctx context.Context, tier plan.Plan, term Term, now time.Time) (Quote, error) {
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
	return "sub:" + userID + ":" + start.In(seoul).Format(time.DateOnly)
}

func billableTier(tier plan.Plan) bool {
	return tier == plan.Basic || tier == plan.Pro || tier == plan.Max
}
