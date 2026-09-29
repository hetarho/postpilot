-- +goose Up
ALTER TABLE billing_intents ADD COLUMN coverage_id TEXT;
ALTER TABLE billing_intents ADD COLUMN funding_end_at TEXT;
UPDATE billing_intents SET coverage_id=COALESCE(
    (SELECT l.coverage_id FROM credit_lots l WHERE l.correlation_id=billing_intents.order_id
     AND l.coverage_id IS NOT NULL LIMIT 1),
    (SELECT t.coverage_id FROM entitlement_tier_transitions t
     WHERE t.correlation_id=billing_intents.order_id LIMIT 1))
WHERE kind<>'pack' AND status='applied';

CREATE TABLE billing_refund_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id TEXT NOT NULL,
    reason TEXT NOT NULL CHECK (length(reason) BETWEEN 1 AND 500),
    status TEXT NOT NULL CHECK (status IN ('requested','rejected','processing','completed','failed')),
    requested_at TEXT NOT NULL,
    reviewed_by TEXT REFERENCES users(id),
    reviewed_at TEXT,
    reviewed_amount_krw INTEGER CHECK (reviewed_amount_krw >= 0),
    disposition_json TEXT,
    idempotency_key TEXT UNIQUE,
    provider_balance_before_krw INTEGER CHECK (provider_balance_before_krw >= 0),
    provider_status TEXT,
    provider_transaction_key TEXT UNIQUE,
    confirmed_amount_krw INTEGER CHECK (confirmed_amount_krw >= 0),
    confirmed_at TEXT
);
CREATE UNIQUE INDEX billing_refund_one_open_per_payment
ON billing_refund_requests(order_id) WHERE status IN ('requested','processing');
CREATE INDEX billing_refund_owner_history
ON billing_refund_requests(user_id,requested_at DESC,id DESC);

CREATE TABLE billing_refund_decisions (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL REFERENCES billing_refund_requests(id) ON DELETE CASCADE,
    reviewer_id TEXT NOT NULL REFERENCES users(id),
    outcome TEXT NOT NULL CHECK (outcome IN ('approve','reject')),
    reviewed_amount_krw INTEGER NOT NULL CHECK (reviewed_amount_krw >= 0),
    evidence_json TEXT NOT NULL,
    disposition_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX billing_refund_decision_history
ON billing_refund_decisions(request_id,created_at,id);

CREATE TABLE billing_refund_provider_outcomes (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL REFERENCES billing_refund_requests(id) ON DELETE CASCADE,
    provider_status TEXT NOT NULL,
    transaction_key TEXT,
    confirmed_amount_krw INTEGER NOT NULL CHECK (confirmed_amount_krw >= 0),
    observed_at TEXT NOT NULL,
    UNIQUE(request_id,transaction_key)
);

-- A pending provider outcome freezes only lots funded by that payment. The
-- usage owner excludes these from new admission while retaining existing holds.
ALTER TABLE credit_lots ADD COLUMN refund_request_id TEXT;
CREATE INDEX credit_lots_refund_guard ON credit_lots(refund_request_id)
WHERE refund_request_id IS NOT NULL;
CREATE TABLE credit_refund_funding_guards (
    request_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    coverage_id TEXT NOT NULL,
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL
);
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

-- Server admission likewise excludes a window while its funding is under
-- review. Final disposition can restore unrelated upgrade capacity.
ALTER TABLE server_export_windows ADD COLUMN refund_request_id TEXT;
ALTER TABLE server_export_adjustments ADD COLUMN refunded_at TEXT;
CREATE TABLE server_export_refund_guards (
    request_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    order_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    coverage_id TEXT NOT NULL,
    starts_at TEXT NOT NULL,
    ends_at TEXT NOT NULL
);
CREATE TRIGGER server_export_refund_block_future_window BEFORE INSERT ON server_export_windows
WHEN EXISTS (SELECT 1 FROM server_export_refund_guards g
    WHERE g.user_id=NEW.user_id AND g.coverage_id=NEW.coverage_id
      AND g.kind IN ('subscribe','renew','upgrade')
      AND NEW.window_start>=g.starts_at AND NEW.window_start<g.ends_at)
BEGIN SELECT RAISE(IGNORE); END;

-- +goose Down
DROP TRIGGER server_export_refund_block_future_window;
DROP TABLE server_export_refund_guards;
ALTER TABLE server_export_adjustments DROP COLUMN refunded_at;
ALTER TABLE server_export_windows DROP COLUMN refund_request_id;
DROP TRIGGER credit_refund_block_future_grant;
DROP TABLE credit_refund_funding_guards;
DROP INDEX credit_lots_refund_guard;
ALTER TABLE credit_lots DROP COLUMN refund_request_id;
DROP TABLE billing_refund_provider_outcomes;
DROP TABLE billing_refund_decisions;
DROP INDEX billing_refund_owner_history;
DROP INDEX billing_refund_one_open_per_payment;
DROP TABLE billing_refund_requests;
ALTER TABLE billing_intents DROP COLUMN coverage_id;
ALTER TABLE billing_intents DROP COLUMN funding_end_at;
