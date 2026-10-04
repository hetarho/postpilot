package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"
)

const refundWindow = 7 * 24 * time.Hour

func newID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		panic("billing: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}

func (s *Service) RefundPurchase(ctx context.Context, userID, purchaseID string) (Purchase, error) {
	if !s.Enabled() {
		return Purchase{}, ErrUnavailable
	}
	purchase, found, err := s.store.Purchase(ctx, userID, purchaseID)
	if err != nil {
		return Purchase{}, err
	}
	if !found || purchase.RefundedAt != nil {
		return Purchase{}, ErrPurchaseNotFound
	}
	now := s.now()
	if !now.Before(purchase.ChargedAt.Add(refundWindow)) {
		return Purchase{}, ErrRefundWindowClosed
	}
	if err := s.credits.VoidUntouchedLot(ctx, purchase.LotID); err != nil {
		if !errors.Is(err, ErrLotTouched) {
			return Purchase{}, err
		}
		// The lot is no longer untouched, which reads two ways: the credits were spent, or an
		// earlier refund voided the lot and failed before the row was marked. A refunded lot
		// holds `remaining = 0` with its grant intact, which is also what a fully spent one
		// holds, so the lot cannot tell them apart — the provider can, and it is the only
		// truth about whether the money left (ARCH-10 keeps that call outside any tx).
		resumable, err := s.paymentAlreadyRefunded(ctx, purchase.OrderID)
		if err != nil {
			return Purchase{}, err
		}
		if !resumable {
			return Purchase{}, ErrPurchaseSpent
		}
		// Deliberately no second Refund call: the money is already back.
		return s.finishRefund(ctx, purchase, now)
	}
	if err := s.provider.Refund(ctx, purchase.ProviderPaymentKey, "purchase refund"); err != nil {
		restoreErr := s.credits.RestoreLot(ctx, purchase.LotID, purchase.Credits)
		return Purchase{}, errors.Join(ErrRefundFailed, err, restoreErr)
	}
	return s.finishRefund(ctx, purchase, now)
}

// paymentAlreadyRefunded asks the provider whether the order is still a live charge.
//
// "DONE" is the only live status, the same reading order settlement applies
// (reconcileFixedPayment), so anything else means the money has left. An order the provider has no
// record of is not evidence of a refund and answers false: marking a purchase refunded on
// no evidence would hand back credits nobody paid back.
func (s *Service) paymentAlreadyRefunded(ctx context.Context, orderID string) (bool, error) {
	payment, found, err := s.provider.PaymentByOrder(ctx, orderID)
	if err != nil {
		return false, err
	}
	return found && payment.Status != "DONE", nil
}

// finishRefund is everything after the money has moved: the row, the ledger event and the
// mail. It is reached both by a refund that just ran and by one being resumed, which is what
// makes the operation retryable — a failure here leaves state the next call completes.
func (s *Service) finishRefund(ctx context.Context, purchase Purchase, now time.Time) (Purchase, error) {
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		marked, err := tx.MarkPurchaseRefunded(ctx, purchase.UserID, purchase.ID, now)
		if err != nil {
			return err
		}
		if !marked {
			return ErrPurchaseNotFound
		}
		return tx.InsertEvent(ctx, purchaseRefundEvent(purchase, now))
	})
	if err != nil {
		return Purchase{}, err
	}
	purchase.RefundedAt = &now
	purchase.Refundable = false
	if err := s.sendMail(ctx, purchase.UserID, RefundMail(purchase)); err != nil {
		slog.Error("purchase refund mail failed", "user_id", purchase.UserID, "purchase_id", purchase.ID, "err", err)
	}
	return purchase, nil
}

func purchaseRefundEvent(purchase Purchase, now time.Time) Event {
	kind := "refund"
	event := Event{
		UserID: purchase.UserID, Kind: kind, Credits: &purchase.Credits,
		KRW:                &purchase.KRW,
		ProviderPaymentKey: &purchase.ProviderPaymentKey, Note: &purchase.ID, CreatedAt: now,
	}
	if purchase.RatePerUSDE4 > 0 {
		event.USDCents, event.KRWPerUSDE4, event.RateDate =
			&purchase.USDCents, &purchase.RatePerUSDE4, &purchase.RateDate
	}
	return event
}
