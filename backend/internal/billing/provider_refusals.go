package billing

import "errors"

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

// definitiveChargeRefusal reports a typed 4xx answer whose code is in definitiveChargeRefusals.
func definitiveChargeRefusal(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.HTTPStatus >= 400 && providerErr.HTTPStatus < 500 &&
		definitiveChargeRefusals[providerErr.Code]
}
