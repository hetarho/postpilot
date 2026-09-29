-- +goose Up
CREATE TABLE billing_quotes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier TEXT NOT NULL CHECK (tier IN ('light','basic','pro','max')),
    term TEXT NOT NULL CHECK (term IN ('monthly','annual')),
    krw INTEGER NOT NULL CHECK (krw >= 0),
    applied_now INTEGER NOT NULL CHECK (applied_now IN (0,1)),
    effective_at TEXT NOT NULL,
    subscription_updated_at TEXT NOT NULL,
    quoted_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE billing_intents (
    order_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('subscribe','renew','upgrade','pack')),
    tier TEXT CHECK (tier IS NULL OR tier IN ('light','basic','pro','max')),
    term TEXT CHECK (term IS NULL OR term IN ('monthly','annual')),
    pack_id TEXT,
    billing_key TEXT NOT NULL,
    customer_key TEXT NOT NULL,
    krw INTEGER NOT NULL CHECK (krw > 0),
    quote_id TEXT,
    quoted_at TEXT NOT NULL,
    subscription_updated_at TEXT,
    effective_at TEXT,
    status TEXT NOT NULL DEFAULT 'pending'
      CHECK (status IN ('pending','applied','failed','review')),
    provider_status TEXT,
    provider_payment_key TEXT,
    applied_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX billing_one_pending_intent ON billing_intents(user_id) WHERE status IN ('pending','review');
CREATE INDEX billing_intents_status ON billing_intents(status, created_at);

-- New fixed-KRW purchases have no historical USD amount. Keep the deprecated
-- column for old rows until T487 retires its contract, but allow a zero sentinel.
CREATE TABLE credit_purchases_fixed (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lot_id TEXT NOT NULL,
    pack_id TEXT,
    credits INTEGER NOT NULL CHECK (credits > 0),
    usd_cents INTEGER NOT NULL CHECK (usd_cents >= 0),
    krw INTEGER NOT NULL CHECK (krw > 0),
    provider_payment_key TEXT NOT NULL,
    order_id TEXT NOT NULL UNIQUE,
    charged_at TEXT NOT NULL,
    refunded_at TEXT
);
INSERT INTO credit_purchases_fixed(id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at,refunded_at)
SELECT id,user_id,lot_id,credits,usd_cents,krw,provider_payment_key,order_id,charged_at,refunded_at
FROM credit_purchases;
DROP TABLE credit_purchases;
ALTER TABLE credit_purchases_fixed RENAME TO credit_purchases;

-- +goose Down
CREATE TABLE credit_purchases_legacy (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lot_id TEXT NOT NULL,
    credits INTEGER NOT NULL CHECK (credits > 0),
    usd_cents INTEGER NOT NULL CHECK (usd_cents > 0),
    krw INTEGER NOT NULL CHECK (krw > 0),
    provider_payment_key TEXT NOT NULL,
    order_id TEXT NOT NULL UNIQUE,
    charged_at TEXT NOT NULL,
    refunded_at TEXT
);
INSERT INTO credit_purchases_legacy
SELECT id,user_id,lot_id,credits,max(usd_cents,1),krw,provider_payment_key,order_id,charged_at,refunded_at
FROM credit_purchases;
DROP TABLE credit_purchases;
ALTER TABLE credit_purchases_legacy RENAME TO credit_purchases;
DROP INDEX billing_intents_status;
DROP INDEX billing_one_pending_intent;
DROP TABLE billing_intents;
DROP TABLE billing_quotes;
