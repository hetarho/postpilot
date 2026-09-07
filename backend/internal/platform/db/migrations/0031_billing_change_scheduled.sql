-- +goose NO TRANSACTION
-- +goose Up
-- SQLite cannot widen a CHECK in place. Rebuild the append-only event table so scheduled
-- tier and term changes have their own durable history kind.
PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE billing_events_new (
    id                   INTEGER PRIMARY KEY,
    user_id              TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind                 TEXT NOT NULL CHECK (kind IN (
                             'charge','charge_failed','refund','grant','tier_change',
                             'renewal_failed','cancel_scheduled','change_scheduled',
                             'cancelled','method_registered'
                         )),
    tier                 TEXT CHECK (tier IS NULL OR tier IN ('basic','pro','max')),
    term                 TEXT CHECK (term IS NULL OR term IN ('monthly','annual')),
    credits              INTEGER,
    usd_cents            INTEGER,
    krw_per_usd_e4       INTEGER,
    rate_date            TEXT,
    krw                  INTEGER,
    provider_payment_key TEXT,
    order_id             TEXT UNIQUE,
    note                 TEXT,
    created_at           TEXT NOT NULL
);

INSERT INTO billing_events_new (
    id, user_id, kind, tier, term, credits, usd_cents, krw_per_usd_e4, rate_date,
    krw, provider_payment_key, order_id, note, created_at
)
SELECT id, user_id, kind, tier, term, credits, usd_cents, krw_per_usd_e4, rate_date,
       krw, provider_payment_key, order_id, note, created_at
FROM billing_events;

DROP TABLE billing_events;
ALTER TABLE billing_events_new RENAME TO billing_events;
CREATE INDEX idx_billing_events_user_created ON billing_events(user_id, created_at);

DROP TABLE IF EXISTS migration_0031_up_integrity_guard;
CREATE TABLE migration_0031_up_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0031_up_integrity_guard (problem)
SELECT 'migration left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0031_up_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE billing_events_new (
    id                   INTEGER PRIMARY KEY,
    user_id              TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind                 TEXT NOT NULL CHECK (kind IN (
                             'charge','charge_failed','refund','grant','tier_change',
                             'renewal_failed','cancel_scheduled','cancelled','method_registered'
                         )),
    tier                 TEXT CHECK (tier IS NULL OR tier IN ('basic','pro','max')),
    term                 TEXT CHECK (term IS NULL OR term IN ('monthly','annual')),
    credits              INTEGER,
    usd_cents            INTEGER,
    krw_per_usd_e4       INTEGER,
    rate_date            TEXT,
    krw                  INTEGER,
    provider_payment_key TEXT,
    order_id             TEXT UNIQUE,
    note                 TEXT,
    created_at           TEXT NOT NULL
);

INSERT INTO billing_events_new (
    id, user_id, kind, tier, term, credits, usd_cents, krw_per_usd_e4, rate_date,
    krw, provider_payment_key, order_id, note, created_at
)
SELECT id, user_id,
       CASE kind WHEN 'change_scheduled' THEN 'cancel_scheduled' ELSE kind END,
       tier, term, credits, usd_cents, krw_per_usd_e4, rate_date,
       krw, provider_payment_key, order_id, note, created_at
FROM billing_events;

DROP TABLE billing_events;
ALTER TABLE billing_events_new RENAME TO billing_events;
CREATE INDEX idx_billing_events_user_created ON billing_events(user_id, created_at);

COMMIT;

PRAGMA foreign_keys=ON;
