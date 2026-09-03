-- +goose NO TRANSACTION
-- +goose Up
-- Credit metering (change 19). Usage stops being three USD windows and becomes one
-- balance: lots of credits granted per month or as a bonus, held when work starts and
-- settled against the ledger when it ends.
--
-- NO TRANSACTION plus an explicit PRAGMA follows 0009's precedent: `users` is an FK parent
-- with ON DELETE CASCADE children across nearly every context, and SQLite's supported way
-- to widen a CHECK constraint is the rebuild below — which would cascade-delete those
-- children if foreign keys stayed on while the old table is dropped. `PRAGMA foreign_keys`
-- is a no-op inside a transaction, so goose must not open one for us; the work still runs
-- in one explicit transaction, and `foreign_key_check` proves the graph before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

----------------------------------------------------------------------------------
-- users: admit `pro` into the ladder. Nothing about credits is stored here — the balance
-- and its renewal instant belong to the usage context, which owns credit_lots.
CREATE TABLE users_new (
    id            TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    plan          TEXT NOT NULL DEFAULT 'free'
                  CHECK (plan IN ('free','basic','pro','max','master'))
);

INSERT INTO users_new (id, password_hash, created_at, plan)
SELECT id, password_hash, created_at, plan FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

----------------------------------------------------------------------------------
-- One row per grant. A balance is the sum of `remaining` over the rows that have not
-- expired, and consumption walks them by `expires_at` ascending with NULLs last — which
-- is why "spend the monthly grant before a non-expiring bonus" needs no second rule: a
-- monthly lot always carries the earlier expiry.
--
-- The `remaining` CHECK is the database's own half of the no-negative-balance guarantee.
-- The Go path already refuses a hold it cannot cover, but that guarantee is the reason
-- this change exists, so it does not rest on one layer.
CREATE TABLE credit_lots (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('monthly','bonus')),
    granted    INTEGER NOT NULL CHECK (granted >= 0),
    remaining  INTEGER NOT NULL CHECK (remaining >= 0 AND remaining <= granted),
    expires_at TEXT,
    created_at TEXT NOT NULL
);

-- The consumption order is the only way this table is ever read.
CREATE INDEX idx_credit_lots_user_expiry ON credit_lots(user_id, expires_at, created_at);

----------------------------------------------------------------------------------
-- The hold rides the admission row: there is already exactly one per job start.
--
-- settled_at IS NULL is the open-hold predicate the settle path and the boot sweep both
-- read, so a crash between "job finished" and "hold settled" strands no credits.
ALTER TABLE usage_admissions ADD COLUMN hold_credits INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_admissions ADD COLUMN settled_credits INTEGER;
ALTER TABLE usage_admissions ADD COLUMN settled_at TEXT;

-- Every admission that predates this migration is already finished work with no hold behind
-- it. Left open, the first boot sweep would treat the entire job history as unsettled and
-- charge each row its per-request base against a balance that never held it.
UPDATE usage_admissions SET settled_credits = 0, settled_at = created_at WHERE settled_at IS NULL;

-- Which lots a hold was taken from, and how much from each. Settlement returns the
-- difference to the same lots rather than re-deriving them from the consumption order:
-- by the time a job ends, a lot may have expired or a new one may have opened, and
-- refunding into the wrong lot would move credits between expiry dates.
CREATE TABLE credit_hold_lots (
    job_id  TEXT NOT NULL,
    lot_id  TEXT NOT NULL REFERENCES credit_lots(id) ON DELETE CASCADE,
    credits INTEGER NOT NULL CHECK (credits > 0),
    PRIMARY KEY (job_id, lot_id)
);

----------------------------------------------------------------------------------
-- No opening lot is minted here. Every pre-existing account is `master` (0013 backfilled
-- them), master is never refused for balance, and a master lot would have to encode
-- "unlimited" as an integer the balance rule has no room for. An account demoted off
-- master opens its first lot on the next request, the same way a renewal does.

----------------------------------------------------------------------------------
-- The curated catalog no longer carries a tier per model. Access is decided by what the
-- account can afford, so a column saying which tier may run a model has nothing left to
-- answer.
ALTER TABLE catalog_models DROP COLUMN min_plan;

DROP TABLE IF EXISTS migration_0019_up_integrity_guard;
CREATE TABLE migration_0019_up_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0019_up_integrity_guard (problem)
SELECT 'migration left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0019_up_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;

BEGIN;

ALTER TABLE catalog_models ADD COLUMN min_plan TEXT NOT NULL DEFAULT 'free';

DROP TABLE credit_hold_lots;
DROP INDEX idx_credit_lots_user_expiry;
DROP TABLE credit_lots;

ALTER TABLE usage_admissions DROP COLUMN settled_at;
ALTER TABLE usage_admissions DROP COLUMN settled_credits;
ALTER TABLE usage_admissions DROP COLUMN hold_credits;

-- Back to the four-rung CHECK. Any account left on `pro` is dropped to `basic`, the
-- nearest rung the old ladder can store, rather than failing the rollback.
CREATE TABLE users_old (
    id            TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    plan          TEXT NOT NULL DEFAULT 'free'
                  CHECK (plan IN ('free','basic','max','master'))
);

INSERT INTO users_old (id, password_hash, created_at, plan)
SELECT id, password_hash, created_at,
       CASE WHEN plan = 'pro' THEN 'basic' ELSE plan END
FROM users;

DROP TABLE users;
ALTER TABLE users_old RENAME TO users;

DROP TABLE IF EXISTS migration_0019_down_integrity_guard;
CREATE TABLE migration_0019_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0019_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0019_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
