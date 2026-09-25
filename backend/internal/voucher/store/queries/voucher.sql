-- Queries for the voucher context. sqlc compiles these into internal/voucher/store/sqlc;
-- internal/voucher/store maps the generated rows to domain types.
--
-- Keep every comment in this file ASCII: sqlc slices the emitted query text by byte offset,
-- so one multi-byte character shifts it and generates SQL that will not parse.

-- name: InsertVoucher :exec
INSERT INTO vouchers (
    id, token, credits, validity_days, sale_krw, payer_name, message,
    issued_by, issued_at, link_expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: VoucherByToken :one
SELECT id, token, credits, validity_days, sale_krw, payer_name, message, issued_by, issued_at,
       link_expires_at, redeemed_by, redeemed_at, lot_id, revoked_at
FROM vouchers
WHERE token = ?;

-- name: VoucherByID :one
SELECT id, token, credits, validity_days, sale_krw, payer_name, message, issued_by, issued_at,
       link_expires_at, redeemed_by, redeemed_at, lot_id, revoked_at
FROM vouchers
WHERE id = ?;

-- name: Vouchers :many
SELECT id, token, credits, validity_days, sale_krw, payer_name, message, issued_by, issued_at,
       link_expires_at, redeemed_by, redeemed_at, lot_id, revoked_at
FROM vouchers
ORDER BY issued_at DESC, id DESC;

-- name: MarkVoucherRedeemed :execrows
-- The guard is in the statement: of two redemptions that both read the voucher as
-- redeemable, only the first write matches, and the second rolls its lot back.
UPDATE vouchers SET redeemed_by = ?, redeemed_at = ?, lot_id = ?
WHERE id = ? AND redeemed_at IS NULL AND revoked_at IS NULL;

-- name: MarkVoucherRevoked :execrows
UPDATE vouchers SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL;
