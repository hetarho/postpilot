-- Billing owns these reads and notification writes. billing_events deliberately has no
-- UPDATE or DELETE query: its history is append-only at the application boundary.

-- name: GetSubscription :one
SELECT user_id, tier, term, anchor_at, term_start, term_end, next_grant_at, auto_renew,
       scheduled_tier, scheduled_term, status, created_at, updated_at
FROM subscriptions WHERE user_id = ?;

-- name: GetPaymentMethod :one
SELECT user_id, provider, billing_key, customer_key, card_label, registered_at
FROM payment_methods WHERE user_id = ?;

-- name: ListBillingEvents :many
SELECT id, user_id, kind, tier, term, credits, usd_cents, krw_per_usd_e4, rate_date,
       krw, provider_payment_key, order_id, note, created_at
FROM billing_events WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?;

-- name: ListCreditPurchases :many
SELECT id, user_id, lot_id, credits, usd_cents, krw, provider_payment_key, order_id,
       charged_at, refunded_at
FROM credit_purchases WHERE user_id = ? ORDER BY charged_at DESC, id DESC;

-- name: InsertProviderNotification :exec
INSERT INTO provider_notifications (
    provider, event_type, payment_key, order_id, status, payload, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?);
