-- +goose Up
-- One-time test-data maintenance is explicit. This migration creates only its
-- durable replay guards; it never removes an entitlement during API boot.
CREATE TABLE test_entitlement_resets (
    id TEXT PRIMARY KEY,
    completed_at TEXT NOT NULL,
    backup_sha256 TEXT NOT NULL,
    report_json TEXT NOT NULL CHECK (json_valid(report_json))
);

CREATE TABLE test_entitlement_reset_orders (
    order_id TEXT PRIMARY KEY,
    reset_id TEXT NOT NULL REFERENCES test_entitlement_resets(id)
);

CREATE TRIGGER test_reset_block_retired_intent BEFORE INSERT ON billing_intents
WHEN EXISTS (SELECT 1 FROM test_entitlement_reset_orders WHERE order_id=NEW.order_id)
BEGIN SELECT RAISE(ABORT, 'retired test order'); END;

CREATE TRIGGER test_reset_block_retired_intent_update BEFORE UPDATE OF order_id ON billing_intents
WHEN EXISTS (SELECT 1 FROM test_entitlement_reset_orders WHERE order_id=NEW.order_id)
BEGIN SELECT RAISE(ABORT, 'retired test order'); END;

CREATE TRIGGER test_reset_block_retired_purchase BEFORE INSERT ON credit_purchases
WHEN EXISTS (SELECT 1 FROM test_entitlement_reset_orders WHERE order_id=NEW.order_id)
BEGIN SELECT RAISE(ABORT, 'retired test order'); END;

CREATE TRIGGER test_reset_block_retired_purchase_update BEFORE UPDATE OF order_id ON credit_purchases
WHEN EXISTS (SELECT 1 FROM test_entitlement_reset_orders WHERE order_id=NEW.order_id)
BEGIN SELECT RAISE(ABORT, 'retired test order'); END;

-- +goose Down
-- Never remove a completed reset's replay tombstones through a rollback.
-- Copying one row onto its own primary key aborts Down if a marker exists.
INSERT INTO test_entitlement_resets(id,completed_at,backup_sha256,report_json)
SELECT id,completed_at,backup_sha256,report_json FROM test_entitlement_resets LIMIT 1;
DROP TRIGGER test_reset_block_retired_purchase;
DROP TRIGGER test_reset_block_retired_purchase_update;
DROP TRIGGER test_reset_block_retired_intent;
DROP TRIGGER test_reset_block_retired_intent_update;
DROP TABLE test_entitlement_reset_orders;
DROP TABLE test_entitlement_resets;
