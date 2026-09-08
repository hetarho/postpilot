package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

const refundWindow = 7 * 24 * time.Hour

func newID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		panic("billing: cannot read random bytes for an id: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}

func (s *Service) QuotePurchase(ctx context.Context, usdCents int) (PurchaseQuote, error) {
	if !s.Enabled() {
		return PurchaseQuote{}, ErrUnavailable
	}
	if usdCents < 100 {
		return PurchaseQuote{}, ErrPurchaseTooSmall
	}
	rate, date, err := s.rateFor(ctx, s.now())
	if err != nil {
		return PurchaseQuote{}, err
	}
	return PurchaseQuote{
		Quote:   Quote{USDCents: usdCents, KRW: KRWFor(usdCents, rate), RatePerUSDE4: rate, RateDate: date},
		Credits: usdCents * plan.CreditsPerUSDCent,
	}, nil
}

func (s *Service) PurchaseCredits(ctx context.Context, userID string, usdCents int) (Purchase, error) {
	if !s.Enabled() {
		return Purchase{}, ErrUnavailable
	}
	if usdCents < 100 {
		return Purchase{}, ErrPurchaseTooSmall
	}
	method, found, err := s.store.PaymentMethod(ctx, userID)
	if err != nil {
		return Purchase{}, err
	}
	if !found {
		return Purchase{}, ErrPaymentMethodRequired
	}
	quote, err := s.QuotePurchase(ctx, usdCents)
	if err != nil {
		return Purchase{}, err
	}
	now := s.now()
	purchaseID := s.newID()
	orderID := "buy:" + purchaseID
	payment, err := s.chargePurchase(ctx, method, quote, orderID)
	if err != nil {
		if eventErr := s.recordPurchaseChargeFailure(ctx, userID, quote, orderID, now); eventErr != nil {
			return Purchase{}, errors.Join(ErrChargeFailed, err, eventErr)
		}
		return Purchase{}, errors.Join(ErrChargeFailed, err)
	}

	purchase := Purchase{
		ID: purchaseID, UserID: userID, Credits: quote.Credits,
		USDCents: quote.USDCents, KRW: quote.KRW, RatePerUSDE4: quote.RatePerUSDE4,
		RateDate: quote.RateDate, ProviderPaymentKey: payment.PaymentKey,
		OrderID: orderID, ChargedAt: now, Refundable: true,
	}
	err = s.store.InWriteTx(ctx, func(tx Store, credits Credits, _ Plans) error {
		lotID, err := credits.OpenPurchasedLot(ctx, userID, quote.Credits)
		if err != nil {
			return err
		}
		purchase.LotID = lotID
		if err := tx.InsertPurchase(ctx, purchase); err != nil {
			return err
		}
		return tx.InsertEvent(ctx, purchaseChargeEvent(userID, quote, payment, orderID, purchaseID, now))
	})
	if err != nil {
		return Purchase{}, err
	}
	if err := s.sendMail(ctx, userID, PurchaseMail(purchase)); err != nil {
		slog.Error("purchase mail failed", "user_id", userID, "purchase_id", purchase.ID, "err", err)
	}
	return purchase, nil
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
		if !errors.Is(err, usage.ErrLotTouched) {
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
// "DONE" is the only live status, the same reading chargePurchase and chargeSubscription
// already apply, so anything else means the money has left. An order the provider has no
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

func (s *Service) chargePurchase(ctx context.Context, method PaymentMethod, quote PurchaseQuote, orderID string) (Payment, error) {
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
		KRW: quote.KRW, Name: fmt.Sprintf("Postpilot %d credits", quote.Credits),
	})
	if err != nil {
		return Payment{}, err
	}
	if payment.Status != "DONE" {
		return Payment{}, fmt.Errorf("order %s has provider status %s", orderID, payment.Status)
	}
	return payment, nil
}

func (s *Service) recordPurchaseChargeFailure(ctx context.Context, userID string, quote PurchaseQuote, orderID string, now time.Time) error {
	note := orderID
	return s.store.InsertEvent(ctx, Event{
		UserID: userID, Kind: "charge_failed", Credits: &quote.Credits,
		USDCents: &quote.USDCents, KRWPerUSDE4: &quote.RatePerUSDE4, RateDate: &quote.RateDate,
		KRW: &quote.KRW, Note: &note, CreatedAt: now,
	})
}

func purchaseChargeEvent(userID string, quote PurchaseQuote, payment Payment, orderID, purchaseID string, now time.Time) Event {
	return Event{
		UserID: userID, Kind: "charge", Credits: &quote.Credits,
		USDCents: &quote.USDCents, KRWPerUSDE4: &quote.RatePerUSDE4, RateDate: &quote.RateDate,
		KRW: &quote.KRW, ProviderPaymentKey: &payment.PaymentKey, OrderID: &orderID,
		Note: &purchaseID, CreatedAt: now,
	}
}

func purchaseRefundEvent(purchase Purchase, now time.Time) Event {
	kind := "refund"
	return Event{
		UserID: purchase.UserID, Kind: kind, Credits: &purchase.Credits,
		USDCents: &purchase.USDCents, KRWPerUSDE4: &purchase.RatePerUSDE4,
		RateDate: &purchase.RateDate, KRW: &purchase.KRW,
		ProviderPaymentKey: &purchase.ProviderPaymentKey, Note: &purchase.ID, CreatedAt: now,
	}
}
