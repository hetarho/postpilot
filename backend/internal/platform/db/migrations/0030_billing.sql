-- +goose Up
CREATE TABLE subscriptions (
    user_id        TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tier           TEXT NOT NULL CHECK (tier IN ('basic','pro','max')),
    term           TEXT NOT NULL CHECK (term IN ('monthly','annual')),
    anchor_at      TEXT NOT NULL,
    term_start     TEXT NOT NULL,
    term_end       TEXT NOT NULL,
    next_grant_at  TEXT NOT NULL,
    auto_renew     INTEGER NOT NULL DEFAULT 1 CHECK (auto_renew IN (0,1)),
    scheduled_tier TEXT CHECK (scheduled_tier IS NULL OR scheduled_tier IN ('basic','pro','max')),
    scheduled_term TEXT CHECK (scheduled_term IS NULL OR scheduled_term IN ('monthly','annual')),
    status          TEXT NOT NULL CHECK (status IN ('active','lapsed')),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE payment_methods (
    user_id       TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    provider      TEXT NOT NULL,
    billing_key   TEXT NOT NULL,
    customer_key  TEXT NOT NULL,
    card_label    TEXT NOT NULL,
    registered_at TEXT NOT NULL
);

-- This table is append-only at the application boundary: billing's sqlc query set contains
-- INSERT and SELECT statements only, never UPDATE or DELETE.
CREATE TABLE billing_events (
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
CREATE INDEX idx_billing_events_user_created ON billing_events(user_id, created_at);

CREATE TABLE credit_purchases (
    id                   TEXT PRIMARY KEY,
    user_id              TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lot_id               TEXT NOT NULL,
    credits              INTEGER NOT NULL CHECK (credits > 0),
    usd_cents            INTEGER NOT NULL CHECK (usd_cents > 0),
    krw                  INTEGER NOT NULL CHECK (krw > 0),
    provider_payment_key TEXT NOT NULL,
    order_id             TEXT NOT NULL UNIQUE,
    charged_at           TEXT NOT NULL,
    refunded_at          TEXT
);

CREATE TABLE provider_notifications (
    id          INTEGER PRIMARY KEY,
    provider    TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    payment_key TEXT,
    order_id    TEXT,
    status      TEXT,
    payload     TEXT NOT NULL,
    received_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE provider_notifications;
DROP TABLE credit_purchases;
DROP INDEX idx_billing_events_user_created;
DROP TABLE billing_events;
DROP TABLE payment_methods;
DROP TABLE subscriptions;
