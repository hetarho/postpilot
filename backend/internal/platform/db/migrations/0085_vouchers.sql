-- +goose Up
-- The operator's vouchers (GIFT): credits that expire a set number of days after redemption,
-- sold by bank transfer or given, each delivered as a one-time gift link.
--
-- The token is stored as issued rather than hashed: the operator must be able to copy an
-- unredeemed link again (GIFT-14), and whoever can read this table can already grant
-- credits. State is not a column; it is derived from the three instants below.
--
-- lot_id names the credit lot a redemption opened. It is a reference by id into the usage
-- context's table with no foreign key: a context reads another's data only through its
-- behaviour (ARCH-7).
CREATE TABLE vouchers (
    id              TEXT PRIMARY KEY,
    token           TEXT NOT NULL UNIQUE,
    credits         INTEGER NOT NULL CHECK (credits > 0),
    validity_days   INTEGER NOT NULL CHECK (validity_days > 0),
    sale_krw        INTEGER CHECK (sale_krw > 0),
    payer_name      TEXT,
    message         TEXT NOT NULL DEFAULT '',
    issued_by       TEXT NOT NULL,
    issued_at       TEXT NOT NULL,
    link_expires_at TEXT NOT NULL,
    redeemed_by     TEXT REFERENCES users(id) ON DELETE SET NULL,
    redeemed_at     TEXT,
    lot_id          TEXT,
    revoked_at      TEXT,
    -- A sale carries both its amount and its payer; a given voucher carries neither.
    CHECK ((sale_krw IS NULL) = (payer_name IS NULL))
);

CREATE INDEX idx_vouchers_issued ON vouchers(issued_at);

-- +goose Down
DROP INDEX idx_vouchers_issued;
DROP TABLE vouchers;
