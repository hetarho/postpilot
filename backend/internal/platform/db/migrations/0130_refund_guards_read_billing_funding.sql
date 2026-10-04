-- +goose Up
-- A refund guard row carries the funding value billing computed for the refunded payment, and
-- the guard triggers read only that row: which grants a payment funded is decided once, in
-- billing (RefundPayment.Funding), never again here by the payment's kind. The credit trigger
-- is the usage store's fundingPredicate applied to a lot about to be issued; the export
-- trigger is the window half of the clip store's exportFundingPredicate.
--
-- correlation_id selects the lots issued for the refunded order (an upgrade's bonus lot);
-- window_cause narrows the coverage window's daily and monthly grants to one issuance cause
-- (an upgrade funded the ones issued lazily at the tier it raised), empty meaning every grant
-- but upgrade bonuses. The upgrade rows a guard already holds are given the values billing now
-- computes for an upgrade, so the rule they enforce is unchanged.
ALTER TABLE credit_refund_funding_guards ADD COLUMN correlation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE credit_refund_funding_guards ADD COLUMN window_cause TEXT NOT NULL DEFAULT '';
UPDATE credit_refund_funding_guards SET correlation_id=order_id, window_cause='lazy' WHERE kind='upgrade';

DROP TRIGGER credit_refund_block_future_grant;
-- +goose StatementBegin
CREATE TRIGGER credit_refund_block_future_grant BEFORE INSERT ON credit_lots
WHEN EXISTS (SELECT 1 FROM credit_refund_funding_guards g
    WHERE g.user_id=NEW.user_id AND
    ((g.correlation_id<>'' AND NEW.correlation_id=g.correlation_id) OR
     (g.coverage_id<>'' AND NEW.coverage_id=g.coverage_id AND NEW.kind IN ('daily','monthly')
      AND COALESCE(NEW.issuance_cause,'')<>'upgrade'
      AND (g.window_cause='' OR NEW.issuance_cause=g.window_cause)
      AND NEW.window_start>=g.starts_at AND NEW.window_start<g.ends_at)))
BEGIN SELECT RAISE(IGNORE); END;
-- +goose StatementEnd

DROP TRIGGER server_export_refund_block_future_window;
-- +goose StatementBegin
CREATE TRIGGER server_export_refund_block_future_window BEFORE INSERT ON server_export_windows
WHEN EXISTS (SELECT 1 FROM server_export_refund_guards g
    WHERE g.user_id=NEW.user_id AND g.coverage_id<>'' AND NEW.coverage_id=g.coverage_id
      AND NEW.window_start>=g.starts_at AND NEW.window_start<g.ends_at)
BEGIN SELECT RAISE(IGNORE); END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER server_export_refund_block_future_window;
-- +goose StatementBegin
CREATE TRIGGER server_export_refund_block_future_window BEFORE INSERT ON server_export_windows
WHEN EXISTS (SELECT 1 FROM server_export_refund_guards g
    WHERE g.user_id=NEW.user_id AND g.coverage_id=NEW.coverage_id
      AND g.kind IN ('subscribe','renew','upgrade')
      AND NEW.window_start>=g.starts_at AND NEW.window_start<g.ends_at)
BEGIN SELECT RAISE(IGNORE); END;
-- +goose StatementEnd

DROP TRIGGER credit_refund_block_future_grant;
-- +goose StatementBegin
CREATE TRIGGER credit_refund_block_future_grant BEFORE INSERT ON credit_lots
WHEN EXISTS (SELECT 1 FROM credit_refund_funding_guards g
    WHERE g.user_id=NEW.user_id AND
    ((g.kind='upgrade' AND (NEW.correlation_id=g.order_id OR
      (NEW.coverage_id=g.coverage_id AND NEW.kind IN ('daily','monthly')
       AND NEW.issuance_cause='lazy' AND NEW.window_start>=g.starts_at
       AND NEW.window_start<g.ends_at))) OR
     (g.kind IN ('subscribe','renew') AND NEW.coverage_id=g.coverage_id
      AND NEW.kind IN ('daily','monthly') AND COALESCE(NEW.issuance_cause,'')<>'upgrade'
      AND NEW.window_start>=g.starts_at AND NEW.window_start<g.ends_at)))
BEGIN SELECT RAISE(IGNORE); END;
-- +goose StatementEnd
ALTER TABLE credit_refund_funding_guards DROP COLUMN window_cause;
ALTER TABLE credit_refund_funding_guards DROP COLUMN correlation_id;
