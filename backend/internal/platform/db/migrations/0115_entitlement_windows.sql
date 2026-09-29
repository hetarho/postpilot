-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE credit_lots_new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('daily','monthly','bonus','purchased','voucher')),
    granted INTEGER NOT NULL CHECK (granted >= 0),
    remaining INTEGER NOT NULL CHECK (remaining >= 0 AND remaining <= granted),
    expires_at TEXT,
    created_at TEXT NOT NULL,
    coverage_id TEXT,
    window_start TEXT,
    issuance_cause TEXT,
    correlation_id TEXT
);
INSERT INTO credit_lots_new (id,user_id,kind,granted,remaining,expires_at,created_at)
SELECT id,user_id,kind,granted,remaining,expires_at,created_at FROM credit_lots;
DROP TABLE credit_lots;
ALTER TABLE credit_lots_new RENAME TO credit_lots;
CREATE INDEX idx_credit_lots_consumption ON credit_lots(user_id,expires_at,created_at,id);
CREATE UNIQUE INDEX idx_credit_lots_grant_window
ON credit_lots(user_id,coverage_id,kind,window_start)
WHERE coverage_id IS NOT NULL AND window_start IS NOT NULL
  AND issuance_cause IN ('coverage','lazy');
-- Upgrade deltas are separate correlated lots in the same benefit window.

CREATE TABLE usage_admission_eligible_lots (
    job_id TEXT NOT NULL,
    lot_id TEXT NOT NULL REFERENCES credit_lots(id) ON DELETE CASCADE,
    PRIMARY KEY(job_id,lot_id)
);
CREATE INDEX idx_usage_admission_eligible_lot ON usage_admission_eligible_lots(lot_id);
ALTER TABLE usage_admissions ADD COLUMN coverage_id TEXT;
ALTER TABLE usage_admissions ADD COLUMN daily_window_start TEXT;
ALTER TABLE usage_admissions ADD COLUMN benefit_window_start TEXT;
ALTER TABLE credit_hold_lots ADD COLUMN origin_coverage_id TEXT;
ALTER TABLE credit_hold_lots ADD COLUMN origin_window_start TEXT;

ALTER TABLE subscriptions ADD COLUMN coverage_id TEXT;
UPDATE subscriptions SET coverage_id = 'paid:' || user_id || ':' || anchor_at;

CREATE TABLE migration_0115_subscription_tiers (
    user_id TEXT PRIMARY KEY, tier TEXT NOT NULL, scheduled_tier TEXT
);
INSERT INTO migration_0115_subscription_tiers
SELECT user_id,tier,scheduled_tier FROM subscriptions;
ALTER TABLE subscriptions DROP COLUMN tier;
ALTER TABLE subscriptions ADD COLUMN tier TEXT NOT NULL DEFAULT 'basic'
    CHECK (tier IN ('light','basic','pro','max'));
ALTER TABLE subscriptions DROP COLUMN scheduled_tier;
ALTER TABLE subscriptions ADD COLUMN scheduled_tier TEXT
    CHECK (scheduled_tier IS NULL OR scheduled_tier IN ('light','basic','pro','max'));
UPDATE subscriptions SET
    tier = (SELECT saved.tier FROM migration_0115_subscription_tiers saved WHERE saved.user_id = subscriptions.user_id),
    scheduled_tier = (SELECT saved.scheduled_tier FROM migration_0115_subscription_tiers saved WHERE saved.user_id = subscriptions.user_id);
DROP TABLE migration_0115_subscription_tiers;

CREATE TABLE migration_0115_event_tiers (id INTEGER PRIMARY KEY, tier TEXT);
INSERT INTO migration_0115_event_tiers SELECT id,tier FROM billing_events;
ALTER TABLE billing_events DROP COLUMN tier;
ALTER TABLE billing_events ADD COLUMN tier TEXT CHECK (tier IS NULL OR tier IN ('light','basic','pro','max'));
UPDATE billing_events SET tier = (SELECT saved.tier FROM migration_0115_event_tiers saved WHERE saved.id = billing_events.id);
DROP TABLE migration_0115_event_tiers;

CREATE TABLE entitlement_tier_transitions (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    coverage_id TEXT NOT NULL,
    effective_at TEXT NOT NULL,
    tier TEXT NOT NULL CHECK (tier IN ('light','basic','pro','max')),
    correlation_id TEXT NOT NULL,
    PRIMARY KEY (user_id,coverage_id,effective_at),
    UNIQUE (correlation_id)
);
CREATE INDEX idx_entitlement_tier_at
ON entitlement_tier_transitions(user_id,coverage_id,effective_at DESC);
INSERT INTO entitlement_tier_transitions(user_id,coverage_id,effective_at,tier,correlation_id)
SELECT user_id,coverage_id,anchor_at,tier,coverage_id || ':legacy'
FROM subscriptions WHERE status = 'active';

-- Operator assignments carry no payment or renewal state. Their coverage is explicit so
-- a support tier cannot be mistaken for a card-funded subscription.
CREATE TABLE support_coverages (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    coverage_id TEXT NOT NULL,
    tier TEXT NOT NULL CHECK (tier IN ('light','basic','pro','max')),
    anchor_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Clip owns these counters. T482 attaches reservation and completion to this window.
CREATE TABLE server_export_windows (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    coverage_id TEXT NOT NULL,
    window_start TEXT NOT NULL,
    window_end TEXT NOT NULL,
    allowance INTEGER NOT NULL CHECK (allowance >= 0),
    used INTEGER NOT NULL DEFAULT 0 CHECK (used >= 0),
    reserved INTEGER NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    correlation_id TEXT,
    PRIMARY KEY(user_id,coverage_id,window_start),
    CHECK (used + reserved <= allowance)
);
CREATE INDEX idx_server_export_window_current
ON server_export_windows(user_id,window_end);
CREATE TABLE server_export_adjustments (
    correlation_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    coverage_id TEXT NOT NULL,
    window_start TEXT NOT NULL,
    allowance_delta INTEGER NOT NULL CHECK (allowance_delta >= 0),
    FOREIGN KEY(user_id,coverage_id,window_start)
        REFERENCES server_export_windows(user_id,coverage_id,window_start) ON DELETE CASCADE
);

CREATE TABLE migration_0115_up_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0115_up_integrity_guard(problem)
SELECT 'foreign-key violation' WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0115_up_integrity_guard;
COMMIT;
PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;
BEGIN;
DROP TABLE server_export_adjustments;
DROP TABLE server_export_windows;
DROP TABLE support_coverages;
DROP TABLE entitlement_tier_transitions;
DROP TABLE usage_admission_eligible_lots;
ALTER TABLE credit_hold_lots DROP COLUMN origin_window_start;
ALTER TABLE credit_hold_lots DROP COLUMN origin_coverage_id;
ALTER TABLE usage_admissions DROP COLUMN benefit_window_start;
ALTER TABLE usage_admissions DROP COLUMN daily_window_start;
ALTER TABLE usage_admissions DROP COLUMN coverage_id;
CREATE TABLE migration_0115_event_tiers (id INTEGER PRIMARY KEY, tier TEXT);
INSERT INTO migration_0115_event_tiers SELECT id,CASE tier WHEN 'light' THEN 'basic' ELSE tier END FROM billing_events;
ALTER TABLE billing_events DROP COLUMN tier;
ALTER TABLE billing_events ADD COLUMN tier TEXT CHECK (tier IS NULL OR tier IN ('basic','pro','max'));
UPDATE billing_events SET tier = (SELECT saved.tier FROM migration_0115_event_tiers saved WHERE saved.id = billing_events.id);
DROP TABLE migration_0115_event_tiers;
CREATE TABLE migration_0115_subscription_tiers (user_id TEXT PRIMARY KEY, tier TEXT NOT NULL, scheduled_tier TEXT);
INSERT INTO migration_0115_subscription_tiers SELECT user_id,
    CASE tier WHEN 'light' THEN 'basic' ELSE tier END,
    CASE scheduled_tier WHEN 'light' THEN 'basic' ELSE scheduled_tier END FROM subscriptions;
ALTER TABLE subscriptions DROP COLUMN tier;
ALTER TABLE subscriptions ADD COLUMN tier TEXT NOT NULL DEFAULT 'basic' CHECK (tier IN ('basic','pro','max'));
ALTER TABLE subscriptions DROP COLUMN scheduled_tier;
ALTER TABLE subscriptions ADD COLUMN scheduled_tier TEXT CHECK (scheduled_tier IS NULL OR scheduled_tier IN ('basic','pro','max'));
UPDATE subscriptions SET
    tier = (SELECT saved.tier FROM migration_0115_subscription_tiers saved WHERE saved.user_id = subscriptions.user_id),
    scheduled_tier = (SELECT saved.scheduled_tier FROM migration_0115_subscription_tiers saved WHERE saved.user_id = subscriptions.user_id);
DROP TABLE migration_0115_subscription_tiers;
ALTER TABLE subscriptions DROP COLUMN coverage_id;
CREATE TABLE credit_lots_old (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('monthly','bonus','purchased','voucher')),
    granted INTEGER NOT NULL CHECK (granted >= 0),
    remaining INTEGER NOT NULL CHECK (remaining >= 0 AND remaining <= granted),
    expires_at TEXT,
    created_at TEXT NOT NULL
);
INSERT INTO credit_lots_old(id,user_id,kind,granted,remaining,expires_at,created_at)
SELECT id,user_id,CASE kind WHEN 'daily' THEN 'bonus' ELSE kind END,
       granted,remaining,expires_at,created_at FROM credit_lots;
DROP TABLE credit_lots;
ALTER TABLE credit_lots_old RENAME TO credit_lots;
CREATE INDEX idx_credit_lots_consumption ON credit_lots(user_id,kind,expires_at,created_at);
COMMIT;
PRAGMA foreign_keys=ON;
