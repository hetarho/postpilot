package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

const fixedQuoteLifetime = 10 * time.Minute

// quoteRetention is how long an expired quote is kept before the billing worker deletes it.
const quoteRetention = 24 * time.Hour

// purgeExpiredQuotes deletes the quotes that expired more than quoteRetention before now; a
// confirm naming a purged quote is refused as stale exactly as an expired one is.
func (s *Service) purgeExpiredQuotes(ctx context.Context, now time.Time) error {
	journals, err := s.intentStore()
	if err != nil {
		return err
	}
	_, err = journals.PurgeExpiredQuotes(ctx, now.Add(-quoteRetention))
	return err
}

// pendingSettleGrace keeps the billing worker off an order the request path created moments
// ago: that path charges and settles the order itself, and a worker that found no payment
// yet would race it to the provider.
const pendingSettleGrace = 2 * time.Minute

func (s *Service) intentStore() (IntentStore, error) {
	store, ok := s.store.(IntentStore)
	if !ok {
		return nil, ErrUnavailable
	}
	return store, nil
}

func (s *Service) persistFixedChangeQuote(ctx context.Context, userID string, subscription Subscription,
	tier plan.Plan, term Term, quote ChangeQuote, now time.Time) (ChangeQuote, error) {
	store, err := s.intentStore()
	if err != nil {
		return ChangeQuote{}, err
	}
	quote.ID = "q-" + s.newID()
	err = store.PutQuote(ctx, QuoteRecord{ID: quote.ID, UserID: userID, Tier: tier, Term: term,
		KRW: quote.KRW, AppliedNow: quote.AppliedNow, EffectiveAt: quote.EffectiveAt,
		SubscriptionUpdatedAt: subscription.UpdatedAt, QuotedAt: now,
		ExpiresAt: now.Add(fixedQuoteLifetime)})
	return quote, err
}

func (s *Service) fixedQuoteInTx(ctx context.Context, tx Store, userID string,
	tier plan.Plan, term Term, id string, now time.Time) (Subscription, QuoteRecord, error) {
	store, ok := tx.(IntentStore)
	if !ok || id == "" {
		return Subscription{}, QuoteRecord{}, ErrStaleQuote
	}
	quote, found, err := store.Quote(ctx, id)
	if err != nil {
		return Subscription{}, QuoteRecord{}, err
	}
	if !found || quote.UserID != userID || quote.Tier != tier || quote.Term != term ||
		now.Before(quote.QuotedAt) || !now.Before(quote.ExpiresAt) {
		return Subscription{}, QuoteRecord{}, ErrStaleQuote
	}
	subscription, found, err := tx.Subscription(ctx, userID)
	if err != nil {
		return Subscription{}, QuoteRecord{}, err
	}
	if !found || subscription.Status != "active" || !now.Before(subscription.TermEnd) ||
		!subscription.UpdatedAt.Equal(quote.SubscriptionUpdatedAt) {
		return Subscription{}, QuoteRecord{}, ErrStaleQuote
	}
	if quote.AppliedNow {
		quotedWindow, _ := plan.BenefitWindow(subscription.AnchorAt, quote.QuotedAt)
		currentWindow, _ := plan.BenefitWindow(subscription.AnchorAt, now)
		if !quotedWindow.Equal(currentWindow) {
			return Subscription{}, QuoteRecord{}, ErrStaleQuote
		}
	}
	return subscription, quote, nil
}

func (s *Service) changeFixed(ctx context.Context, userID string, tier plan.Plan, term Term,
	quoteID string) (Subscription, bool, error) {
	if !s.Enabled() {
		return Subscription{}, false, ErrUnavailable
	}
	now := s.now()
	var updated Subscription
	var intent Intent
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		if err := refuseMaster(ctx, plans, userID); err != nil {
			return err
		}
		subscription, quote, err := s.fixedQuoteInTx(ctx, tx, userID, tier, term, quoteID, now)
		if err != nil {
			return err
		}
		journals := tx.(IntentStore)
		if _, pending, err := journals.PendingIntent(ctx, userID); err != nil {
			return err
		} else if pending {
			return ErrPaymentPending
		}
		_, kind, err := s.classifyFixedSnapshot(subscription, tier, term)
		if err != nil {
			return err
		}
		if (kind == changeUpgrade) != quote.AppliedNow {
			return ErrStaleQuote
		}
		if kind == changeScheduled {
			updated = subscription
			updated.ScheduledTier, updated.ScheduledTerm = &tier, &term
			updated.AutoRenew, updated.UpdatedAt = true, now
			if err := tx.UpsertSubscription(ctx, updated); err != nil {
				return err
			}
			return tx.InsertEvent(ctx, Event{UserID: userID, Kind: "change_scheduled", Tier: &tier, Term: &term, CreatedAt: now})
		}
		method, found, err := tx.PaymentMethod(ctx, userID)
		if err != nil {
			return err
		}
		if !found {
			return ErrPaymentMethodRequired
		}
		updated = subscription
		intent = Intent{OrderID: "pp-upg-" + s.newID(), UserID: userID, Kind: "upgrade",
			Tier: tier, Term: term, KRW: quote.KRW, QuoteID: quote.ID,
			BillingKey: method.BillingKey, CustomerKey: method.CustomerKey,
			QuotedAt: quote.QuotedAt, SubscriptionUpdatedAt: subscription.UpdatedAt,
			EffectiveAt: quote.EffectiveAt, Status: "pending", CreatedAt: now, UpdatedAt: now}
		if intent.KRW <= 0 {
			return ErrStaleQuote
		}
		return journals.InsertIntent(ctx, intent)
	})
	if err != nil {
		return Subscription{}, false, err
	}
	if intent.OrderID == "" {
		return updated, false, nil
	}
	if err := s.processFixedIntent(ctx, intent); err != nil {
		return Subscription{}, false, err
	}
	result, found, err := s.store.Subscription(ctx, userID)
	if err != nil || !found {
		return Subscription{}, false, err
	}
	return result, true, nil
}

func (s *Service) classifyFixedSnapshot(subscription Subscription, tier plan.Plan, term Term) (Subscription, changeKind, error) {
	if !billableTier(tier) {
		return Subscription{}, changeNone, ErrTierNotSubscribable
	}
	if !term.Valid() {
		return Subscription{}, changeNone, ErrChangeUnsupported
	}
	if tier == subscription.Tier && term == subscription.Term {
		return Subscription{}, changeNone, ErrNoChange
	}
	if term == subscription.Term && tier.Rank() > subscription.Tier.Rank() {
		return subscription, changeUpgrade, nil
	}
	return subscription, changeScheduled, nil
}

func (s *Service) subscribeFixed(ctx context.Context, userID string, tier plan.Plan, term Term) (Subscription, error) {
	if !s.Enabled() {
		return Subscription{}, ErrUnavailable
	}
	if !billableTier(tier) {
		return Subscription{}, ErrTierNotSubscribable
	}
	if !term.Valid() {
		return Subscription{}, ErrChangeUnsupported
	}
	now := s.now()
	quote, err := s.quoteAt(ctx, tier, term, now)
	if err != nil {
		return Subscription{}, err
	}
	intent := Intent{OrderID: "pp-sub-" + s.newID(), UserID: userID, Kind: "subscribe",
		Tier: tier, Term: term, KRW: quote.KRW, QuotedAt: now,
		Status: "pending", CreatedAt: now, UpdatedAt: now}
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		if err := refuseMaster(ctx, plans, userID); err != nil {
			return err
		}
		journals, ok := tx.(IntentStore)
		if !ok {
			return ErrUnavailable
		}
		if _, pending, err := journals.PendingIntent(ctx, userID); err != nil {
			return err
		} else if pending {
			return ErrPaymentPending
		}
		current, found, err := tx.Subscription(ctx, userID)
		if err != nil {
			return err
		}
		if found && current.Status == "active" {
			return ErrSubscriptionExists
		}
		if found {
			intent.SubscriptionUpdatedAt = current.UpdatedAt
		}
		method, found, err := tx.PaymentMethod(ctx, userID)
		if err != nil {
			return err
		}
		if !found {
			return ErrPaymentMethodRequired
		}
		intent.BillingKey, intent.CustomerKey = method.BillingKey, method.CustomerKey
		return journals.InsertIntent(ctx, intent)
	})
	if err != nil {
		return Subscription{}, err
	}
	if err := s.processFixedIntent(ctx, intent); err != nil {
		return Subscription{}, err
	}
	result, found, err := s.store.Subscription(ctx, userID)
	if err != nil || !found {
		return Subscription{}, err
	}
	return result, nil
}

func fixedRenewOrderID(userID string, end time.Time) string {
	sum := sha256.Sum256([]byte(userID + ":" + end.UTC().Format(time.RFC3339Nano)))
	return "pp-ren-" + hex.EncodeToString(sum[:16])
}

func (s *Service) renewFixed(ctx context.Context, subscription Subscription, now time.Time) (Subscription, error) {
	var intent Intent
	var inReview Intent
	var failedTier plan.Plan
	var failedTerm Term
	var failedQuote Quote
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		journals, ok := tx.(IntentStore)
		if !ok {
			return ErrUnavailable
		}
		if blocking, pending, err := journals.PendingIntent(ctx, subscription.UserID); err != nil {
			return err
		} else if pending {
			if blocking.Status == "review" {
				inReview = blocking
			}
			return ErrPaymentPending
		}
		current, found, err := tx.Subscription(ctx, subscription.UserID)
		if err != nil {
			return err
		}
		if !found || current.Status != "active" || !current.TermEnd.Equal(subscription.TermEnd) ||
			!current.UpdatedAt.Equal(subscription.UpdatedAt) {
			return ErrStaleQuote
		}
		tier, term := current.Tier, current.Term
		if current.ScheduledTier != nil {
			tier = *current.ScheduledTier
		}
		if current.ScheduledTerm != nil {
			term = *current.ScheduledTerm
		}
		quote, err := s.quoteAt(ctx, tier, term, now)
		if err != nil {
			return err
		}
		method, found, err := tx.PaymentMethod(ctx, subscription.UserID)
		if err != nil {
			return err
		}
		if !found {
			failedTier, failedTerm, failedQuote = tier, term, quote
			return s.failRenewalInTx(ctx, tx, plans, current, tier, term, quote, now)
		}
		intent = Intent{OrderID: fixedRenewOrderID(subscription.UserID, current.TermEnd),
			UserID: subscription.UserID, Kind: "renew", Tier: tier, Term: term,
			KRW: quote.KRW, BillingKey: method.BillingKey, CustomerKey: method.CustomerKey,
			QuotedAt: now, SubscriptionUpdatedAt: current.UpdatedAt, EffectiveAt: current.TermEnd,
			Status: "pending", CreatedAt: now, UpdatedAt: now}
		return journals.InsertIntent(ctx, intent)
	})
	if err != nil {
		if inReview.OrderID != "" {
			// Until an operator resolves the order in review, the account renews on no pass.
			slog.Error("renewal skipped: an order is in review", "user_id", subscription.UserID,
				"order_id", inReview.OrderID, "term_end", subscription.TermEnd)
		}
		return subscription, err
	}
	if intent.OrderID == "" {
		if err := s.sendMail(ctx, subscription.UserID, RenewalFailedMail(failedTier, failedTerm, failedQuote)); err != nil {
			return subscription, err
		}
		return subscription, ErrChargeFailed
	}
	if err := s.processFixedIntent(ctx, intent); err != nil {
		return subscription, err
	}
	updated, found, err := s.store.Subscription(ctx, subscription.UserID)
	if err != nil || !found {
		return subscription, err
	}
	return updated, nil
}

func (s *Service) QuotePack(ctx context.Context, packID string) (PurchaseQuote, error) {
	if !s.fixedKRW || !s.Enabled() {
		return PurchaseQuote{}, ErrUnavailable
	}
	pack, found := plan.PackByID(packID)
	if !found {
		return PurchaseQuote{}, ErrInvalidPack
	}
	return PurchaseQuote{Quote: Quote{KRW: pack.PriceKRW}, PackID: pack.ID, Credits: pack.Credits}, nil
}

func (s *Service) PurchasePack(ctx context.Context, userID, packID string) (Purchase, error) {
	quote, err := s.QuotePack(ctx, packID)
	if err != nil {
		return Purchase{}, err
	}
	now := s.now()
	intent := Intent{OrderID: "pp-buy-" + s.newID(), UserID: userID, Kind: "pack",
		PackID: packID, KRW: quote.KRW, QuotedAt: now, Status: "pending",
		CreatedAt: now, UpdatedAt: now}
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		if err := refuseMaster(ctx, plans, userID); err != nil {
			return err
		}
		journals, ok := tx.(IntentStore)
		if !ok {
			return ErrUnavailable
		}
		if _, pending, err := journals.PendingIntent(ctx, userID); err != nil {
			return err
		} else if pending {
			return ErrPaymentPending
		}
		sub, found, err := tx.Subscription(ctx, userID)
		if err != nil {
			return err
		}
		if !found || sub.Status != "active" || !now.Before(sub.TermEnd) {
			return ErrSubscriptionRequired
		}
		intent.SubscriptionUpdatedAt = sub.UpdatedAt
		method, found, err := tx.PaymentMethod(ctx, userID)
		if err != nil {
			return err
		}
		if !found {
			return ErrPaymentMethodRequired
		}
		intent.BillingKey, intent.CustomerKey = method.BillingKey, method.CustomerKey
		return journals.InsertIntent(ctx, intent)
	})
	if err != nil {
		return Purchase{}, err
	}
	if err := s.processFixedIntent(ctx, intent); err != nil {
		return Purchase{}, err
	}
	purchase, found, err := s.store.Purchase(ctx, userID, intent.OrderID)
	if err != nil || !found {
		return Purchase{}, fmt.Errorf("confirmed pack purchase missing: %w", err)
	}
	return purchase, nil
}

func (s *Service) requireNoPending(ctx context.Context, tx Store, userID string) error {
	if !s.fixedKRW {
		return nil
	}
	journals, ok := tx.(IntentStore)
	if !ok {
		return ErrUnavailable
	}
	_, found, err := journals.PendingIntent(ctx, userID)
	if err != nil {
		return err
	}
	if found {
		return ErrPaymentPending
	}
	return nil
}

func finalProviderFailure(status string) bool {
	return status == "ABORTED" || status == "EXPIRED" || status == "CANCELED"
}
