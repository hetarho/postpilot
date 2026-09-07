-- +goose NO TRANSACTION
-- +goose Up
-- A third kind of lot: credits an account bought (QUOTA-34), which never expire and are
-- the last kind consumed (QUOTA-12).
--
-- Widening a CHECK means rebuilding the table, and NO TRANSACTION plus an explicit PRAGMA
-- follows 0019's precedent for the same reason: `credit_hold_lots.lot_id` references this
-- table with ON DELETE CASCADE, so dropping the old table with foreign keys on would take
-- every open hold's debit rows with it. `PRAGMA foreign_keys` is a no-op inside a
-- transaction, so goose must not open one for us; the work still runs in one explicit
-- transaction and `foreign_key_check` proves the graph before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE credit_lots_new (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('monthly','bonus','purchased')),
    granted    INTEGER NOT NULL CHECK (granted >= 0),
    remaining  INTEGER NOT NULL CHECK (remaining >= 0 AND remaining <= granted),
    expires_at TEXT,
    created_at TEXT NOT NULL
);

INSERT INTO credit_lots_new (id, user_id, kind, granted, remaining, expires_at, created_at)
SELECT id, user_id, kind, granted, remaining, expires_at, created_at FROM credit_lots;

DROP TABLE credit_lots;
ALTER TABLE credit_lots_new RENAME TO credit_lots;

-- Consumption now orders by KIND first and only then by expiry (QUOTA-12), so the index
-- carries kind beside the two it already had. It narrows to one account's unexpired rows;
-- ranking the three kinds is a sort over the handful that survive, which is why the rank
-- itself is not stored as a column.
CREATE INDEX idx_credit_lots_consumption ON credit_lots(user_id, kind, expires_at, created_at);

DROP TABLE IF EXISTS migration_0027_up_integrity_guard;
CREATE TABLE migration_0027_up_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0027_up_integrity_guard (problem)
SELECT 'migration left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0027_up_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE credit_lots_new (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('monthly','bonus')),
    granted    INTEGER NOT NULL CHECK (granted >= 0),
    remaining  INTEGER NOT NULL CHECK (remaining >= 0 AND remaining <= granted),
    expires_at TEXT,
    created_at TEXT NOT NULL
);

-- A purchased lot becomes a bonus rather than being dropped: the credits were paid for, and
-- the two-kind world has no other place to keep them.
INSERT INTO credit_lots_new (id, user_id, kind, granted, remaining, expires_at, created_at)
SELECT id, user_id,
       CASE kind WHEN 'purchased' THEN 'bonus' ELSE kind END,
       granted, remaining, expires_at, created_at
FROM credit_lots;

DROP TABLE credit_lots;
ALTER TABLE credit_lots_new RENAME TO credit_lots;

CREATE INDEX idx_credit_lots_user_expiry ON credit_lots(user_id, expires_at, created_at);

COMMIT;

PRAGMA foreign_keys=ON;
