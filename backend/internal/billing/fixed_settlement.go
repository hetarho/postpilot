package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

// ReconcilePending resumes recorded orders after a timeout, process restart or
// a provider success followed by a failed local commit. It never invents an
// order or grants a benefit from a browser redirect.
func (s *Service) ReconcilePending(ctx context.Context) error {
	if !s.fixedKRW {
		return nil
	}
	journals, err := s.intentStore()
	if err != nil {
		return err
	}
	intents, err := journals.DueIntents(ctx)
	if err != nil {
		return err
	}
	var failures []error
	for _, intent := range intents {
		if err := s.processFixedIntent(ctx, intent); err != nil &&
			!errors.Is(err, ErrPaymentPending) && !errors.Is(err, ErrChargeFailed) {
			failures = append(failures, fmt.Errorf("order %s: %w", intent.OrderID, err))
		}
	}
	return errors.Join(failures...)
}

// ReconcileOrder is the authenticated-by-provider-read webhook path. Toss does
// not emit a completion webhook for automatic billing, so the direct server
// charge response and periodic order lookup use the same settlement function.
func (s *Service) ReconcileOrder(ctx context.Context, orderID string) error {
	if !s.fixedKRW {
		return nil
	}
	journals, err := s.intentStore()
	if err != nil {
		return err
	}
	intent, found, err := journals.Intent(ctx, orderID)
	if err != nil || !found {
		return err
	}
	if intent.Status == "applied" || intent.Status == "failed" || intent.Status == "review" {
		return nil
	}
	payment, found, err := s.provider.PaymentByOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if !found {
		return ErrPaymentPending
	}
	return s.reconcileFixedPayment(ctx, intent, payment)
}

func (s *Service) processFixedIntent(ctx context.Context, intent Intent) error {
	if intent.Status != "pending" {
		return ErrPaymentPending
	}
	payment, found, err := s.provider.PaymentByOrder(ctx, intent.OrderID)
	if err != nil {
		return errors.Join(ErrPaymentPending, err)
	}
	if !found {
		payment, err = s.provider.Charge(ctx, ChargeRequest{
			BillingKey: intent.BillingKey, CustomerKey: intent.CustomerKey,
			OrderID: intent.OrderID, KRW: intent.KRW, Name: fixedOrderName(intent),
		})
		if err != nil {
			// A typed provider rejection is final. A timeout or network error has
			// an unknown outcome and keeps the order pending for reconciliation.
			var providerErr *ProviderError
			if errors.As(err, &providerErr) && providerErr.HTTPStatus >= 400 && providerErr.HTTPStatus < 500 &&
				providerErr.HTTPStatus != 409 && providerErr.Code != "ALREADY_PROCESSED_PAYMENT" {
				if markErr := s.failFixedIntent(ctx, intent, "ABORTED"); markErr != nil {
					return errors.Join(ErrChargeFailed, err, markErr)
				}
				return errors.Join(ErrChargeFailed, err)
			}
			return errors.Join(ErrPaymentPending, err)
		}
		// A charge response is server-side evidence; an independent order read
		// also recovers a response lost after Toss captured the charge.
		payment, found, err = s.provider.PaymentByOrder(ctx, intent.OrderID)
		if err != nil || !found {
			return errors.Join(ErrPaymentPending, err)
		}
	}
	return s.reconcileFixedPayment(ctx, intent, payment)
}

func fixedOrderName(intent Intent) string {
	if intent.Kind == "pack" {
		return "Postpilot credit pack " + intent.PackID
	}
	return fmt.Sprintf("Postpilot %s %s subscription", intent.Tier, intent.Term)
}

func (s *Service) reconcileFixedPayment(ctx context.Context, intent Intent, payment Payment) error {
	if payment.OrderID != intent.OrderID {
		return ErrPaymentPending
	}
	if payment.Status == "DONE" {
		if payment.PaymentKey == "" || payment.AmountKRW != intent.KRW || payment.Currency != "KRW" {
			return s.reviewFixedIntent(ctx, intent, payment)
		}
		return s.applyFixedPayment(ctx, intent, payment)
	}
	if finalProviderFailure(payment.Status) {
		if err := s.failFixedIntent(ctx, intent, payment.Status); err != nil {
			return err
		}
		return ErrChargeFailed
	}
	return ErrPaymentPending
}

func (s *Service) reviewFixedIntent(ctx context.Context, intent Intent, payment Payment) error {
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		journals := tx.(IntentStore)
		_, err := journals.MarkIntent(ctx, intent.OrderID, "review", payment.Status, payment.PaymentKey, s.now())
		return err
	})
	if err != nil {
		return err
	}
	return ErrPaymentPending
}

func (s *Service) applyFixedPayment(ctx context.Context, intent Intent, payment Payment) error {
	now := s.now()
	var firstApply bool
	var review bool
	err := s.store.InWriteTx(ctx, func(tx Store, credits Credits, plans Plans) error {
		journals := tx.(IntentStore)
		current, found, err := journals.Intent(ctx, intent.OrderID)
		if err != nil {
			return err
		}
		if !found {
			return ErrPaymentPending
		}
		if current.Status == "applied" {
			return nil
		}
		if current.Status != "pending" {
			return ErrPaymentPending
		}
		if err := s.applyFixedEntitlement(ctx, tx, credits, plans, current, payment, now); err != nil {
			if errors.Is(err, ErrStaleQuote) {
				_, markErr := journals.MarkIntent(ctx, current.OrderID, "review", payment.Status, payment.PaymentKey, now)
				review = markErr == nil
				return markErr
			}
			return err
		}
		if current.Kind != "pack" {
			sub, found, err := tx.Subscription(ctx, current.UserID)
			if err != nil {
				return err
			}
			if !found || sub.CoverageID == "" {
				return ErrPaymentPending
			}
			if recorder, ok := tx.(interface {
				SetIntentFunding(context.Context, string, string, time.Time) error
			}); ok {
				if err := recorder.SetIntentFunding(ctx, current.OrderID, sub.CoverageID, sub.TermEnd); err != nil {
					return err
				}
			}
		}
		marked, err := journals.MarkIntent(ctx, current.OrderID, "applied", payment.Status, payment.PaymentKey, now)
		if err != nil {
			return err
		}
		if !marked {
			return ErrPaymentPending
		}
		firstApply = true
		return nil
	})
	if err != nil {
		return err
	}
	if review {
		return ErrPaymentPending
	}
	if !firstApply {
		return nil
	}
	if intent.Kind == "renew" {
		if err := s.sendMail(ctx, intent.UserID, RenewalMail(intent.Tier, intent.Term, Quote{KRW: intent.KRW})); err != nil {
			slog.Error("renewal mail failed", "order_id", intent.OrderID, "err", err)
		}
	}
	if intent.Kind == "pack" {
		purchase, found, err := s.store.Purchase(ctx, intent.UserID, intent.OrderID)
		if err == nil && found {
			if mailErr := s.sendMail(ctx, intent.UserID, PurchaseMail(purchase)); mailErr != nil {
				slog.Error("purchase mail failed", "order_id", intent.OrderID, "err", mailErr)
			}
		}
	}
	return nil
}

func (s *Service) applyFixedEntitlement(ctx context.Context, tx Store, credits Credits, plans Plans,
	intent Intent, payment Payment, now time.Time) error {
	subscription, found, err := tx.Subscription(ctx, intent.UserID)
	if err != nil {
		return err
	}
	if intent.Kind != "pack" {
		if intent.Kind == "subscribe" {
			if found && (subscription.Status == "active" || !subscription.UpdatedAt.Equal(intent.SubscriptionUpdatedAt)) {
				return ErrStaleQuote
			}
		} else if !found || subscription.Status != "active" ||
			!subscription.UpdatedAt.Equal(intent.SubscriptionUpdatedAt) {
			return ErrStaleQuote
		}
	}
	switch intent.Kind {
	case "subscribe":
		return s.applyFixedSubscription(ctx, tx, credits, plans, intent, payment, now)
	case "renew":
		if !subscription.TermEnd.Equal(intent.EffectiveAt) {
			return ErrStaleQuote
		}
		return s.applyFixedRenewal(ctx, tx, credits, plans, subscription, intent, payment, now)
	case "upgrade":
		if !now.Before(subscription.TermEnd) || subscription.Tier.Rank() >= intent.Tier.Rank() ||
			subscription.Term != intent.Term {
			return ErrStaleQuote
		}
		return s.applyFixedUpgrade(ctx, tx, credits, plans, subscription, intent, payment, now)
	case "pack":
		return s.applyFixedPack(ctx, tx, credits, intent, payment, now)
	default:
		return fmt.Errorf("unknown billing intent kind %q", intent.Kind)
	}
}

func (s *Service) applyFixedSubscription(ctx context.Context, tx Store, credits Credits, plans Plans,
	intent Intent, payment Payment, now time.Time) error {
	_, next := plan.BenefitWindow(now, now)
	coverageID := "paid:" + intent.UserID + ":" + now.UTC().Format(time.RFC3339Nano)
	sub := Subscription{UserID: intent.UserID, CoverageID: coverageID, Tier: intent.Tier,
		Term: intent.Term, AnchorAt: now, TermStart: now, TermEnd: TermEnd(now, now, intent.Term),
		NextGrantAt: next, AutoRenew: true, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := tx.UpsertSubscription(ctx, sub); err != nil {
		return err
	}
	if err := tx.InsertEvent(ctx, fixedChargeEvent(intent, payment, now)); err != nil {
		return err
	}
	if err := tx.InsertEvent(ctx, Event{UserID: intent.UserID, Kind: "tier_change",
		Tier: &intent.Tier, Term: &intent.Term, CreatedAt: now}); err != nil {
		return err
	}
	if err := plans.AssignTier(ctx, intent.UserID, intent.Tier); err != nil {
		return err
	}
	if err := tx.DeleteSupportCoverage(ctx, intent.UserID); err != nil {
		return err
	}
	if err := tx.InsertTierTransition(ctx, intent.UserID, coverageID, now, intent.Tier, intent.OrderID); err != nil {
		return err
	}
	return credits.OpenCoverage(ctx, intent.UserID, Coverage{ID: coverageID, Anchor: now,
		End: sub.TermEnd, Tier: intent.Tier, DailyTier: intent.Tier}, now, intent.OrderID)
}

func (s *Service) applyFixedRenewal(ctx context.Context, tx Store, credits Credits, plans Plans,
	subscription Subscription, intent Intent, payment Payment, now time.Time) error {
	start := subscription.TermEnd
	updated := subscription
	updated.Tier, updated.Term, updated.TermStart = intent.Tier, intent.Term, start
	updated.TermEnd = TermEnd(subscription.AnchorAt, start, intent.Term)
	_, updated.NextGrantAt = plan.BenefitWindow(subscription.AnchorAt, start)
	updated.ScheduledTier, updated.ScheduledTerm = nil, nil
	updated.UpdatedAt = now
	if err := tx.UpsertSubscription(ctx, updated); err != nil {
		return err
	}
	if err := tx.InsertEvent(ctx, fixedChargeEvent(intent, payment, now)); err != nil {
		return err
	}
	support, assigned, err := tx.SupportCoverage(ctx, intent.UserID)
	if err != nil {
		return err
	}
	if intent.Tier != subscription.Tier {
		if err := tx.InsertEvent(ctx, Event{UserID: intent.UserID, Kind: "tier_change",
			Tier: &intent.Tier, Term: &intent.Term, CreatedAt: now}); err != nil {
			return err
		}
		if !assigned {
			if err := plans.AssignTier(ctx, intent.UserID, intent.Tier); err != nil {
				return err
			}
			if err := tx.InsertTierTransition(ctx, intent.UserID, subscription.CoverageID,
				start, intent.Tier, intent.OrderID); err != nil {
				return err
			}
		}
	}
	if assigned {
		_ = support
		return nil
	}
	dailyStart, _ := plan.DailyWindow(subscription.AnchorAt, start)
	dailyTier := subscription.Tier
	if !dailyStart.Before(start) {
		dailyTier = intent.Tier
	}
	return credits.OpenCoverage(ctx, intent.UserID, Coverage{ID: subscription.CoverageID,
		Anchor: subscription.AnchorAt, End: updated.TermEnd, Tier: intent.Tier,
		DailyTier: dailyTier}, start, intent.OrderID)
}

func (s *Service) applyFixedUpgrade(ctx context.Context, tx Store, credits Credits, plans Plans,
	subscription Subscription, intent Intent, payment Payment, now time.Time) error {
	start, end := plan.BenefitWindow(subscription.AnchorAt, intent.QuotedAt)
	currentStart, _ := plan.BenefitWindow(subscription.AnchorAt, now)
	if !start.Equal(currentStart) {
		// A charge confirmed after the quoted benefit month needs review: the
		// frozen bonus must not be silently moved into a different month.
		return ErrStaleQuote
	}
	amounts, err := plan.QuoteUpgrade(subscription.Tier, intent.Tier, intent.Term == TermAnnual,
		subscription.TermStart, subscription.TermEnd, start, end, intent.QuotedAt)
	if err != nil {
		return err
	}
	if int(amounts.ChargeKRW) != intent.KRW {
		return ErrStaleQuote
	}
	dailyStart, _ := plan.DailyWindow(subscription.AnchorAt, now)
	dailyTier, err := tx.TierAt(ctx, intent.UserID, subscription.CoverageID, dailyStart)
	if err != nil {
		return err
	}
	old := Coverage{ID: subscription.CoverageID, Anchor: subscription.AnchorAt,
		End: subscription.TermEnd, Tier: subscription.Tier, DailyTier: dailyTier}
	updated := subscription
	updated.Tier, updated.ScheduledTier, updated.ScheduledTerm, updated.UpdatedAt = intent.Tier, nil, nil, now
	if err := tx.UpsertSubscription(ctx, updated); err != nil {
		return err
	}
	if err := tx.InsertEvent(ctx, fixedChargeEvent(intent, payment, now)); err != nil {
		return err
	}
	if err := tx.InsertEvent(ctx, Event{UserID: intent.UserID, Kind: "tier_change",
		Tier: &intent.Tier, Term: &intent.Term, CreatedAt: now}); err != nil {
		return err
	}
	if err := plans.AssignTier(ctx, intent.UserID, intent.Tier); err != nil {
		return err
	}
	if err := credits.AddUpgradeBonus(ctx, intent.UserID, old, now,
		int(amounts.BonusCredits), int(amounts.ServerExports), intent.OrderID); err != nil {
		return err
	}
	return tx.InsertTierTransition(ctx, intent.UserID, subscription.CoverageID, now, intent.Tier, intent.OrderID)
}

func (s *Service) applyFixedPack(ctx context.Context, tx Store, credits Credits,
	intent Intent, payment Payment, now time.Time) error {
	pack, found := plan.PackByID(intent.PackID)
	if !found || pack.PriceKRW != intent.KRW {
		return ErrInvalidPack
	}
	lotID, err := credits.OpenPurchasedLot(ctx, intent.UserID, pack.Credits)
	if err != nil {
		return err
	}
	purchase := Purchase{ID: intent.OrderID, PackID: intent.PackID, UserID: intent.UserID, LotID: lotID,
		Credits: pack.Credits, KRW: pack.PriceKRW, ProviderPaymentKey: payment.PaymentKey,
		OrderID: intent.OrderID, ChargedAt: now, Refundable: true}
	if err := tx.InsertPurchase(ctx, purchase); err != nil {
		return err
	}
	return tx.InsertEvent(ctx, fixedChargeEvent(intent, payment, now))
}

func fixedChargeEvent(intent Intent, payment Payment, now time.Time) Event {
	event := Event{UserID: intent.UserID, Kind: "charge", KRW: &intent.KRW,
		ProviderPaymentKey: &payment.PaymentKey, OrderID: &intent.OrderID, CreatedAt: now}
	if intent.Kind == "pack" {
		pack, _ := plan.PackByID(intent.PackID)
		event.Credits = &pack.Credits
		event.Note = &intent.OrderID
	} else {
		event.Tier, event.Term = &intent.Tier, &intent.Term
	}
	return event
}

func (s *Service) failFixedIntent(ctx context.Context, intent Intent, providerStatus string) error {
	now := s.now()
	var firstFailure bool
	var lapsed bool
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		journals := tx.(IntentStore)
		current, found, err := journals.Intent(ctx, intent.OrderID)
		if err != nil {
			return err
		}
		if !found || current.Status != "pending" {
			return nil
		}
		if current.Kind == "renew" {
			sub, found, err := tx.Subscription(ctx, current.UserID)
			if err != nil {
				return err
			}
			if found && sub.Status == "active" && sub.UpdatedAt.Equal(current.SubscriptionUpdatedAt) {
				if err := s.failRenewalInTx(ctx, tx, plans, sub, current.Tier, current.Term,
					Quote{KRW: current.KRW}, now); err != nil {
					return err
				}
				lapsed = true
			} else if err := tx.InsertEvent(ctx, Event{UserID: current.UserID, Kind: "charge_failed",
				Tier: &current.Tier, Term: &current.Term, KRW: &current.KRW,
				Note: &current.OrderID, CreatedAt: now}); err != nil {
				return err
			}
		} else {
			if err := tx.InsertEvent(ctx, Event{UserID: current.UserID, Kind: "charge_failed",
				Tier: optionalTier(current), Term: optionalTerm(current), KRW: &current.KRW,
				Note: &current.OrderID, CreatedAt: now}); err != nil {
				return err
			}
		}
		marked, err := journals.MarkIntent(ctx, current.OrderID, "failed", providerStatus, "", now)
		firstFailure = marked
		return err
	})
	if err != nil {
		return err
	}
	if firstFailure && lapsed {
		if err := s.sendMail(ctx, intent.UserID, RenewalFailedMail(intent.Tier, intent.Term, Quote{KRW: intent.KRW})); err != nil {
			slog.Error("renewal failure mail failed", "order_id", intent.OrderID, "err", err)
		}
	}
	return nil
}

func (s *Service) failRenewalInTx(ctx context.Context, tx Store, plans Plans, sub Subscription,
	tier plan.Plan, term Term, quote Quote, now time.Time) error {
	updated := sub
	updated.Status, updated.UpdatedAt = "lapsed", now
	if err := tx.UpsertSubscription(ctx, updated); err != nil {
		return err
	}
	if support, assigned, err := tx.SupportCoverage(ctx, sub.UserID); err != nil {
		return err
	} else if assigned {
		if err := plans.AssignTier(ctx, sub.UserID, support.Tier); err != nil {
			return err
		}
	} else if err := plans.AssignTier(ctx, sub.UserID, plan.Free); err != nil {
		return err
	}
	return tx.InsertEvent(ctx, Event{UserID: sub.UserID, Kind: "renewal_failed",
		Tier: &tier, Term: &term, KRW: &quote.KRW, CreatedAt: now})
}

func optionalTier(i Intent) *plan.Plan {
	if i.Tier == "" {
		return nil
	}
	return &i.Tier
}
func optionalTerm(i Intent) *Term {
	if i.Term == "" {
		return nil
	}
	return &i.Term
}
