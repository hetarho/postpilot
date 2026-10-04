package billing

import (
	"context"
	"errors"
)

// definitiveChargeRefusals are the codes in Toss Payments' 자동결제 승인 error table
// (POST /v1/billing/{billingKey}) that say the card, its issuer or its billing key refused
// this charge, copied verbatim from the docs. Only these end an order as failed: a timeout,
// a 429, a 5xx, DUPLICATED_ORDER_ID, ALREADY_PROCESSED_PAYMENT, IDEMPOTENT_REQUEST_PROCESSING
// or any other answer leaves the outcome unknown, so the order stays pending and the next
// pass reads it back before charging again under the same idempotency key.
var definitiveChargeRefusals = map[string]bool{
	"INVALID_STOPPED_CARD":        true,
	"INVALID_REJECT_CARD":         true,
	"INVALID_CARD_LOST_OR_STOLEN": true,
	"INVALID_CARD_EXPIRATION":     true,
	"INVALID_CARD_NUMBER":         true,
	"INVALID_BILL_KEY_REQUEST":    true,
	"NOT_SUPPORTED_CARD_TYPE":     true,
	"NOT_REGISTERED_CARD_COMPANY": true,
	"REJECT_CARD_PAYMENT":         true,
	"REJECT_ACCOUNT_PAYMENT":      true,
	"REJECT_CARD_COMPANY":         true,
	"EXCEED_MAX_AUTH_COUNT":       true,
}

// definitiveCancelRefusals are the codes in Toss Payments' 결제 취소 error table
// (POST /v1/payments/{paymentKey}/cancel) that refuse this cancel for good, copied verbatim
// from the docs. A refusal is not proof that no money moved — ALREADY_CANCELED_PAYMENT can
// answer a retry of a cancel that went through — so the payment is read back before the
// request is failed. Transient answers (PROVIDER_ERROR, FORBIDDEN_CONSECUTIVE_REQUEST,
// NOT_AVAILABLE_BANK, a 5xx) and configuration errors keep the request processing.
var definitiveCancelRefusals = map[string]bool{
	"INVALID_REQUEST":                         true,
	"ALREADY_CANCELED_PAYMENT":                true,
	"ALREADY_REFUND_PAYMENT":                  true,
	"EXCEED_CANCEL_AMOUNT_DISCOUNT_AMOUNT":    true,
	"NOT_MATCHES_REFUNDABLE_AMOUNT":           true,
	"REFUND_REJECTED":                         true,
	"NOT_CANCELABLE_AMOUNT":                   true,
	"NOT_CANCELABLE_PAYMENT":                  true,
	"NOT_CANCELABLE_PAYMENT_FOR_DORMANT_USER": true,
	"EXCEED_MAX_REFUND_DUE":                   true,
	"NOT_ALLOWED_PARTIAL_REFUND":              true,
}

// definitiveCancelRefusal returns the typed 4xx answer whose code is in definitiveCancelRefusals.
func definitiveCancelRefusal(err error) (*ProviderError, bool) {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) && providerErr.HTTPStatus >= 400 && providerErr.HTTPStatus < 500 &&
		definitiveCancelRefusals[providerErr.Code] {
		return providerErr, true
	}
	return nil, false
}

// definitiveChargeRefusal reports a typed 4xx answer whose code is in definitiveChargeRefusals.
func definitiveChargeRefusal(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.HTTPStatus >= 400 && providerErr.HTTPStatus < 500 &&
		definitiveChargeRefusals[providerErr.Code]
}

// cancelOrReadBack sends one cancel under its idempotency key and returns what settles it: the
// provider's answer on success, or on a definitive refusal the payment read back by its order —
// a refusal is not proof that no money moved, so the caller decides by what the payment shows.
// Any other answer leaves the outcome unknown (ErrRefundProviderPending).
func (s *Service) cancelOrReadBack(ctx context.Context, paymentKey, orderID string,
	amountKRW int, reason, idempotencyKey string) (Payment, *ProviderError, error) {
	canceled, err := s.provider.CancelPayment(ctx, paymentKey, amountKRW, reason, idempotencyKey)
	if err == nil {
		return canceled, nil, nil
	}
	refusal, definitive := definitiveCancelRefusal(err)
	if !definitive {
		return Payment{}, nil, errors.Join(ErrRefundProviderPending, err)
	}
	observed, seen, readErr := s.provider.PaymentByOrder(ctx, orderID)
	if readErr != nil || !seen || observed.PaymentKey != paymentKey {
		return Payment{}, nil, errors.Join(ErrRefundProviderPending, err, readErr)
	}
	return observed, refusal, nil
}
