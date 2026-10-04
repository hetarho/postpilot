package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// unappliedRefundBatch bounds the orders in review one billing pass refunds.
const unappliedRefundBatch = 100

// unappliedRefundKey is the cancel's idempotency key: stable per order, so a retry after a
// lost answer can never return the money twice.
func unappliedRefundKey(orderID string) string { return "unapplied:" + orderID }

// RefundUnappliedCaptures returns, in full, every payment the provider captured that the
// product could not apply — the orders in `review` (BILL-22). Each order is settled on its
// own, and every provider call runs outside a write transaction (ARCH-10). An unresolved
// answer, or a refusal the payment does not explain, leaves the order in review for the next
// pass; only a storage failure is returned.
func (s *Service) RefundUnappliedCaptures(ctx context.Context) error {
	if !s.fixedKRW {
		return nil
	}
	journals, err := s.intentStore()
	if err != nil {
		return err
	}
	orders, err := journals.ReviewIntents(ctx, unappliedRefundBatch)
	if err != nil || len(orders) == 0 {
		return err
	}
	provider, ok := s.provider.(RefundProvider)
	if !ok {
		return ErrUnavailable
	}
	var failures []error
	for _, intent := range orders {
		err := s.refundUnapplied(ctx, provider, intent)
		if err == nil || errors.Is(err, ErrRefundProviderPending) {
			continue
		}
		failures = append(failures, fmt.Errorf("order %s: %w", intent.OrderID, err))
	}
	return errors.Join(failures...)
}

func (s *Service) refundUnapplied(ctx context.Context, provider RefundProvider, intent Intent) error {
	captured, found, err := s.provider.PaymentByOrder(ctx, intent.OrderID)
	if err != nil || !found || captured.OrderID != intent.OrderID || captured.PaymentKey == "" {
		slog.Warn("unapplied order refund unresolved: captured payment not read; order stays in review",
			"user_id", intent.UserID, "order_id", intent.OrderID, "found", found, "err", err)
		return errors.Join(ErrRefundProviderPending, err)
	}
	observed, refusal, err := s.cancelOrReadBack(ctx, provider, captured.PaymentKey, intent.OrderID,
		captured.AmountKRW, "Postpilot could not apply this payment", unappliedRefundKey(intent.OrderID))
	if err != nil {
		slog.Warn("unapplied order refund unresolved; order stays in review",
			"user_id", intent.UserID, "order_id", intent.OrderID, "err", err)
		return err
	}
	if refusal == nil {
		// The provider's answer to the cancel is not the record: the payment read back is.
		observed, found, err = s.provider.PaymentByOrder(ctx, intent.OrderID)
		if err != nil || !found {
			return errors.Join(ErrRefundProviderPending, err)
		}
	}
	if !returnedInFull(observed, captured) {
		if refusal != nil {
			slog.Error("unapplied order refund refused; order stays in review", "user_id", intent.UserID,
				"order_id", intent.OrderID, "code", refusal.Code, "balance_krw", observed.BalanceKRW)
			return errors.Join(ErrRefundProviderPending, refusal)
		}
		return ErrRefundProviderPending
	}
	return s.completeUnappliedRefund(ctx, intent, observed, captured.AmountKRW)
}

// returnedInFull reports the payment read back as all of captured cancelled: no balance left
// and a settled cancel of the captured amount, whether this pass's, a lost-answer retry's or
// one made at the provider since.
func returnedInFull(observed, captured Payment) bool {
	return observed.PaymentKey == captured.PaymentKey && observed.OrderID == captured.OrderID &&
		observed.AmountKRW == captured.AmountKRW && observed.Currency == captured.Currency &&
		observed.BalanceKRW == 0 && matchingCancelTransaction(observed, captured.AmountKRW) != ""
}

// completeUnappliedRefund fails the order and records its refund in one transaction, which
// also lifts the account's pending-order lock; the mail follows the commit.
//
// A renewal whose payment went back is a finally failed renewal (BILL-8): the term it was to
// extend has ended unpaid, so the same transaction lapses the subscription to free as a refused
// renewal does, with no retry, and the renewal-failed mail follows the refund mail. An upgrade
// leaves the base subscription as it was, and a pack only refunds.
func (s *Service) completeUnappliedRefund(ctx context.Context, intent Intent, observed Payment, amount int) error {
	now := s.now()
	var refunded, lapsed bool
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		failed, err := tx.(IntentStore).FailReviewIntent(ctx, intent.OrderID, observed.Status, now)
		if err != nil || !failed {
			return err
		}
		key, note := observed.PaymentKey, intent.OrderID
		if err := tx.InsertEvent(ctx, Event{UserID: intent.UserID, Kind: "refund", KRW: &amount,
			ProviderPaymentKey: &key, Note: &note, CreatedAt: now}); err != nil {
			return err
		}
		refunded = true
		if intent.Kind != "renew" {
			return nil
		}
		sub, found, err := tx.Subscription(ctx, intent.UserID)
		if err != nil || !found || sub.Status != "active" || !sub.TermEnd.Equal(intent.EffectiveAt) {
			return err
		}
		lapsed = true
		return s.failRenewalInTx(ctx, tx, plans, sub, intent.Tier, intent.Term, Quote{KRW: intent.KRW}, now)
	})
	if err != nil {
		return err
	}
	if refunded {
		if err := s.sendMail(ctx, intent.UserID, unappliedRefundMail(amount)); err != nil {
			slog.Error("refund mail failed", "order_id", intent.OrderID, "err", err)
		}
	}
	if lapsed {
		if err := s.sendMail(ctx, intent.UserID, RenewalFailedMail(intent.Tier, intent.Term, Quote{KRW: intent.KRW})); err != nil {
			slog.Error("renewal failure mail failed", "order_id", intent.OrderID, "err", err)
		}
	}
	return nil
}

func unappliedRefundMail(amount int) MailMessage {
	return MailMessage{Subject: "Postpilot 결제를 적용하지 못해 환불했습니다 / Payment refunded: it could not be applied",
		Text: fmt.Sprintf("결제를 주문에 적용하지 못해 %d원 전액이 결제사에서 환불되었습니다.\n\nYour payment could not be applied to its order, so the full KRW %d was refunded by the payment provider.", amount, amount)}
}
