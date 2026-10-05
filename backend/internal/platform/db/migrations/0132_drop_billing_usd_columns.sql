-- +goose Up
-- Billing charges fixed KRW (BILL-2) and its history shows whole KRW only (BILL-15), so the
-- dollar amount and exchange-rate columns of the regime before it are dropped. No index,
-- trigger, view or foreign key names them, and the CHECK on credit_purchases.usd_cents is the
-- column's own, so each goes in place without a rebuild.
ALTER TABLE billing_events DROP COLUMN usd_cents;
ALTER TABLE billing_events DROP COLUMN krw_per_usd_e4;
ALTER TABLE billing_events DROP COLUMN rate_date;
ALTER TABLE credit_purchases DROP COLUMN usd_cents;

-- +goose Down
-- The dropped values are not restored: a rolled-back binary reads a KRW-only row as one
-- without a dollar amount, which every fixed-KRW row already was.
ALTER TABLE credit_purchases ADD COLUMN usd_cents INTEGER NOT NULL DEFAULT 0 CHECK (usd_cents >= 0);
ALTER TABLE billing_events ADD COLUMN rate_date TEXT;
ALTER TABLE billing_events ADD COLUMN krw_per_usd_e4 INTEGER;
ALTER TABLE billing_events ADD COLUMN usd_cents INTEGER;
