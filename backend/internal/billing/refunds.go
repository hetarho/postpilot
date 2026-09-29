package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *Service) refundStore() (RefundStore, error) {
	store, ok := s.store.(RefundStore)
	if !ok {
		return nil, ErrUnavailable
	}
	return store, nil
}

// RequestRefund never moves money. The owner's reason and the funding payment
// become a durable review item even after seven days or after benefit use.
func (s *Service) RequestRefund(ctx context.Context, userID, orderID, reason string) (RefundRequest, error) {
	reason = strings.TrimSpace(reason)
	if userID == "" || orderID == "" || utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return RefundRequest{}, ErrInvalidRefundRequest
	}
	if _, err := s.refundStore(); err != nil {
		return RefundRequest{}, err
	}
	request := RefundRequest{ID: s.newID(), UserID: userID, OrderID: orderID, Reason: reason,
		Status: "requested", RequestedAt: s.now().UTC()}
	err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		ref := tx.(RefundStore)
		payment, found, err := ref.RefundPayment(ctx, userID, orderID)
		if err != nil {
			return err
		}
		if !found {
			return ErrRefundNotFound
		}
		if open, err := ref.OpenRefundForOrder(ctx, orderID); err != nil {
			return err
		} else if open {
			return ErrRefundConflict
		}
		prior, err := ref.ConfirmedRefundTotal(ctx, orderID)
		if err != nil {
			return err
		}
		if prior >= payment.KRW {
			return ErrRefundAmount
		}
		request.PriorRefundedKRW = prior
		if err := ref.InsertRefundRequest(ctx, request); err != nil {
			return err
		}
		request.Payment = payment
		benefits := ref.RefundBenefits()
		if benefits == nil {
			return ErrUnavailable
		}
		request.Evidence, err = benefits.Inspect(ctx, payment, request.RequestedAt)
		return err
	})
	return request, err
}

func (s *Service) RefundRequests(ctx context.Context, userID string) ([]RefundRequest, error) {
	ref, err := s.refundStore()
	if err != nil {
		return nil, err
	}
	items, err := ref.Refunds(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		payment, found, err := ref.RefundPayment(ctx, items[i].UserID, items[i].OrderID)
		if err != nil {
			return nil, err
		}
		if found {
			items[i].Payment = payment
			items[i].PriorRefundedKRW, err = ref.ConfirmedRefundTotal(ctx, items[i].OrderID)
			if err != nil {
				return nil, err
			}
			items[i].Evidence, err = s.refundEvidence(ctx, ref, items[i], payment)
			if err != nil {
				return nil, err
			}
		}
	}
	return items, nil
}

func (s *Service) RefundRequest(ctx context.Context, requestID string) (RefundRequest, error) {
	ref, err := s.refundStore()
	if err != nil {
		return RefundRequest{}, err
	}
	item, found, err := ref.RefundRequest(ctx, requestID)
	if err != nil {
		return RefundRequest{}, err
	}
	if !found {
		return RefundRequest{}, ErrRefundNotFound
	}
	payment, found, err := ref.RefundPayment(ctx, item.UserID, item.OrderID)
	if err != nil {
		return RefundRequest{}, err
	}
	if found {
		item.Payment = payment
		item.PriorRefundedKRW, err = ref.ConfirmedRefundTotal(ctx, item.OrderID)
		if err != nil {
			return RefundRequest{}, err
		}
		item.Evidence, err = s.refundEvidence(ctx, ref, item, payment)
	}
	return item, err
}

// ReconcilePendingRefunds is called by the billing worker. A lost response
// stays visible as processing and is retried with its original idempotency key.
func (s *Service) ReconcilePendingRefunds(ctx context.Context) error {
	ref, err := s.refundStore()
	if err != nil {
		return err
	}
	ids, err := ref.ProcessingRefundIDs(ctx, s.now().Add(-7*24*time.Hour), 100)
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err := s.ReconcileRefund(ctx, id); err != nil && !errors.Is(err, ErrRefundProviderPending) {
			failures = append(failures, fmt.Errorf("refund %s: %w", id, err))
		}
	}
	return errors.Join(failures...)
}

func (s *Service) refundEvidence(ctx context.Context, ref RefundStore, request RefundRequest, payment RefundPayment) (RefundEvidence, error) {
	if request.Status != "requested" {
		if snapshot, found, err := ref.ReviewedEvidence(ctx, request.ID); err != nil {
			return RefundEvidence{}, err
		} else if found {
			return snapshot, nil
		}
	}
	if benefits := ref.RefundBenefits(); benefits != nil {
		return benefits.Inspect(ctx, payment, s.now())
	}
	return RefundEvidence{}, ErrUnavailable
}

// ReviewRefund freezes affected funding in the same writer transaction as its
// decision. Provider I/O follows the commit; a timeout leaves the guard and
// processing intent in place for ReconcileRefund.
func (s *Service) ReviewRefund(ctx context.Context, reviewerID, requestID, outcome string, amountKRW int) (RefundRequest, error) {
	if reviewerID == "" || requestID == "" || (outcome != "approve" && outcome != "reject") {
		return RefundRequest{}, ErrInvalidRefundRequest
	}
	ref, err := s.refundStore()
	if err != nil {
		return RefundRequest{}, err
	}
	stored, found, err := ref.RefundRequest(ctx, requestID)
	if err != nil {
		return RefundRequest{}, err
	}
	if !found {
		return RefundRequest{}, ErrRefundNotFound
	}
	if stored.Status != "requested" {
		return RefundRequest{}, ErrRefundConflict
	}
	var providerPayment Payment
	if outcome == "approve" {
		if s.provider == nil {
			return RefundRequest{}, ErrUnavailable
		}
		providerPayment, found, err = s.provider.PaymentByOrder(ctx, stored.OrderID)
		if err != nil || !found {
			return RefundRequest{}, errors.Join(ErrRefundProviderPending, err)
		}
	}
	now := s.now().UTC()
	var reviewed RefundRequest
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
		ref := tx.(RefundStore)
		request, found, err := ref.RefundRequest(ctx, requestID)
		if err != nil {
			return err
		}
		if !found || request.Status != "requested" {
			return ErrRefundConflict
		}
		payment, found, err := ref.RefundPayment(ctx, request.UserID, request.OrderID)
		if err != nil {
			return err
		}
		if !found {
			return ErrRefundNotFound
		}
		benefits := ref.RefundBenefits()
		if benefits == nil {
			return ErrUnavailable
		}
		evidence, err := benefits.Inspect(ctx, payment, now)
		if err != nil {
			return err
		}
		request.Payment, request.Evidence = payment, evidence
		request.PriorRefundedKRW, err = ref.ConfirmedRefundTotal(ctx, request.OrderID)
		if err != nil {
			return err
		}
		decision := RefundDecision{ID: s.newID(), RequestID: request.ID, ReviewerID: reviewerID,
			Outcome: outcome, AmountKRW: amountKRW, CreatedAt: now}
		decision.EvidenceJSON, err = encodeRefundJSON(evidence)
		if err != nil {
			return err
		}
		if outcome == "reject" {
			if amountKRW != 0 {
				return ErrRefundAmount
			}
			decision.DispositionJSON = `{"effect":"none"}`
			request.IdempotencyKey = ""
		} else {
			if evidence.Active() {
				return ErrRefundActiveUse
			}
			if dependent, err := ref.HasUnresolvedDependentUpgrade(ctx, payment); err != nil {
				return err
			} else if dependent {
				return ErrRefundDependentPayment
			}
			if providerPayment.OrderID != payment.OrderID || providerPayment.PaymentKey != payment.PaymentKey ||
				providerPayment.AmountKRW != payment.KRW || providerPayment.Currency != "KRW" ||
				amountKRW <= 0 || amountKRW > providerPayment.BalanceKRW {
				return ErrRefundAmount
			}
			if now.Before(payment.ChargedAt.Add(7*24*time.Hour)) && evidence.Unused() &&
				amountKRW != providerPayment.BalanceKRW {
				return ErrRefundAmount
			}
			request.IdempotencyKey = "refund:" + request.ID
			request.ProviderBalanceBeforeKRW = providerPayment.BalanceKRW
			decision.DispositionJSON = `{"effect":"void_remaining_payment_funding"}`
			if err := benefits.Guard(ctx, request, payment, now); err != nil {
				return err
			}
		}
		if err := ref.RecordRefundDecision(ctx, request, decision); err != nil {
			return err
		}
		request.Status, request.ReviewedBy = "rejected", reviewerID
		if outcome == "approve" {
			request.Status = "processing"
		}
		request.ReviewedAt, request.ReviewedAmountKRW = &now, amountKRW
		request.DispositionJSON = decision.DispositionJSON
		reviewed = request
		return nil
	})
	if err != nil {
		return RefundRequest{}, err
	}
	if outcome == "reject" {
		if err := s.sendMail(ctx, reviewed.UserID, MailMessage{Subject: "Postpilot 환불 요청 심사 결과 / Refund review decision",
			Text: "환불 요청이 심사 후 반려되었습니다. 확인된 환불액은 0원입니다.\n\nYour refund request was rejected after review. Confirmed refund: KRW 0."}); err != nil {
			slog.Error("refund decision mail failed", "refund_id", reviewed.ID, "err", err)
		}
		return reviewed, nil
	}
	if err := s.ReconcileRefund(ctx, requestID); err != nil && !errors.Is(err, ErrRefundProviderPending) {
		return reviewed, err
	}
	latest, _, err := ref.RefundRequest(ctx, requestID)
	if err == nil {
		latest.Payment, latest.Evidence = reviewed.Payment, reviewed.Evidence
	}
	return latest, err
}

func encodeRefundJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

// ReconcileRefund can run after a timeout or restart. The same provider
// idempotency key is used every time; no second local entitlement reversal is
// possible once the request becomes completed.
func (s *Service) ReconcileRefund(ctx context.Context, requestID string) error {
	ref, err := s.refundStore()
	if err != nil {
		return err
	}
	request, found, err := ref.RefundRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if !found {
		return ErrRefundNotFound
	}
	if request.Status == "completed" {
		return nil
	}
	if request.Status != "processing" {
		return ErrRefundConflict
	}
	payment, found, err := ref.RefundPayment(ctx, request.UserID, request.OrderID)
	if err != nil || !found {
		return errors.Join(ErrRefundNotFound, err)
	}
	provider, ok := s.provider.(RefundProvider)
	if !ok {
		return ErrUnavailable
	}
	transaction := request.ProviderTransactionKey
	if transaction == "" {
		// A payment outcome with no persisted transaction may have succeeded even
		// when its response was lost. Never send a new cancel after the provider's
		// idempotency retention may have expired; leave it for manual review.
		if request.ReviewedAt != nil && s.now().Sub(*request.ReviewedAt) >= 7*24*time.Hour {
			return ErrRefundProviderPending
		}
		canceled, cancelErr := provider.CancelPayment(ctx, payment.PaymentKey, request.ReviewedAmountKRW,
			"Postpilot reviewed refund", request.IdempotencyKey)
		if cancelErr != nil {
			var typed *ProviderError
			if errors.As(cancelErr, &typed) && typed.Code == "INVALID_REQUEST" {
				observed, seen, readErr := s.provider.PaymentByOrder(ctx, payment.OrderID)
				if readErr == nil && seen && observed.PaymentKey == payment.PaymentKey &&
					observed.BalanceKRW == request.ProviderBalanceBeforeKRW {
					failure := s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
						ref := tx.(RefundStore)
						if benefits := ref.RefundBenefits(); benefits != nil {
							if err := benefits.Release(ctx, request, payment); err != nil {
								return err
							}
						}
						return ref.FailRefund(ctx, request.ID, typed.Code, s.now())
					})
					return errors.Join(ErrRefundFailed, cancelErr, failure)
				}
			}
			return errors.Join(ErrRefundProviderPending, cancelErr)
		}
		if canceled.PaymentKey != payment.PaymentKey || canceled.OrderID != payment.OrderID ||
			canceled.AmountKRW != payment.KRW || canceled.Currency != "KRW" ||
			canceled.BalanceKRW > request.ProviderBalanceBeforeKRW-request.ReviewedAmountKRW {
			return ErrRefundProviderPending
		}
		transaction = matchingCancelTransaction(canceled, request.ReviewedAmountKRW)
		if transaction == "" {
			return ErrRefundProviderPending
		}
		if err := s.store.InWriteTx(ctx, func(tx Store, _ Credits, _ Plans) error {
			return tx.(RefundStore).RecordRefundProviderAttempt(ctx, request.ID, transaction)
		}); err != nil {
			return err
		}
	}
	confirmed, found, err := s.provider.PaymentByOrder(ctx, payment.OrderID)
	if err != nil || !found {
		return errors.Join(ErrRefundProviderPending, err)
	}
	if !confirmedCancelHasKey(confirmed, transaction, request.ReviewedAmountKRW) ||
		confirmed.PaymentKey != payment.PaymentKey || confirmed.OrderID != payment.OrderID ||
		confirmed.AmountKRW != payment.KRW || confirmed.Currency != "KRW" ||
		confirmed.BalanceKRW > request.ProviderBalanceBeforeKRW-request.ReviewedAmountKRW {
		return ErrRefundProviderPending
	}
	now := s.now().UTC()
	request.ProviderTransactionKey = transaction
	request.ConfirmedAmountKRW = request.ReviewedAmountKRW
	var applied bool
	err = s.store.InWriteTx(ctx, func(tx Store, _ Credits, plans Plans) error {
		ref := tx.(RefundStore)
		current, found, err := ref.RefundRequest(ctx, request.ID)
		if err != nil {
			return err
		}
		if !found {
			return ErrRefundNotFound
		}
		if current.Status == "completed" {
			return nil
		}
		if current.Status != "processing" {
			return ErrRefundConflict
		}
		benefits := ref.RefundBenefits()
		if benefits == nil {
			return ErrUnavailable
		}
		if err := benefits.Confirm(ctx, request, payment, now); err != nil {
			return err
		}
		if err := s.reverseRefundedEntitlements(ctx, tx, plans, request, payment, now); err != nil {
			return err
		}
		if err := ref.RecordRefundOutcome(ctx, request, confirmed, now); err != nil {
			return err
		}
		amount := request.ConfirmedAmountKRW
		note, key := request.ID, payment.PaymentKey
		if err := tx.InsertEvent(ctx, Event{UserID: request.UserID, Kind: "refund", KRW: &amount,
			ProviderPaymentKey: &key, Note: &note, CreatedAt: now}); err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil {
		return err
	}
	if applied {
		if err := s.sendMail(ctx, request.UserID, reviewedRefundMail(request.ConfirmedAmountKRW)); err != nil {
			slog.Error("refund mail failed", "refund_id", request.ID, "err", err)
		}
	}
	return nil
}

func matchingCancelTransaction(payment Payment, amount int) string {
	for i := len(payment.Cancels) - 1; i >= 0; i-- {
		candidate := payment.Cancels[i]
		if candidate.AmountKRW == amount && candidate.Status == "DONE" && candidate.TransactionKey != "" {
			return candidate.TransactionKey
		}
	}
	return ""
}

func confirmedCancelHasKey(payment Payment, key string, amount int) bool {
	for _, cancel := range payment.Cancels {
		if cancel.TransactionKey == key && cancel.AmountKRW == amount && cancel.Status == "DONE" {
			return true
		}
	}
	return false
}

func (s *Service) reverseRefundedEntitlements(ctx context.Context, tx Store, plans Plans,
	request RefundRequest, payment RefundPayment, at time.Time) error {
	if payment.Kind == "pack" {
		marked, err := tx.MarkPurchaseRefunded(ctx, payment.UserID, payment.OrderID, at)
		if err != nil {
			return err
		}
		if !marked {
			return ErrRefundConflict
		}
		return nil
	}
	sub, found, err := tx.Subscription(ctx, payment.UserID)
	if err != nil {
		return err
	}
	if !found || sub.CoverageID != payment.CoverageID || !at.Before(payment.FundingEnd) || sub.Status != "active" {
		return nil
	}
	if payment.Kind == "upgrade" {
		if sub.Tier != payment.Tier || payment.PriorTier == "" {
			return nil
		}
		sub.Tier = payment.PriorTier
		sub.UpdatedAt = at
		if err := tx.UpsertSubscription(ctx, sub); err != nil {
			return err
		}
		if err := tx.InsertTierTransition(ctx, payment.UserID, payment.CoverageID,
			at, payment.PriorTier, "refund:"+request.ID); err != nil {
			return err
		}
		return plans.AssignTier(ctx, payment.UserID, payment.PriorTier)
	}
	// A refunded base charge no longer funds this term. A newer renewal has a
	// later funding interval and therefore never enters this branch.
	sub.Status, sub.AutoRenew = "lapsed", false
	sub.TermEnd, sub.NextGrantAt, sub.UpdatedAt = at, at, at
	if err := tx.UpsertSubscription(ctx, sub); err != nil {
		return err
	}
	return plans.AssignTier(ctx, payment.UserID, "free")
}

func reviewedRefundMail(amount int) MailMessage {
	return MailMessage{Subject: "Postpilot 환불 완료 / Refund completed",
		Text: fmt.Sprintf("검토된 환불 %d원이 결제사에서 확인되었습니다.\n\nYour reviewed refund of KRW %d was confirmed by the payment provider.", amount, amount)}
}
