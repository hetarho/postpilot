-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE fx_reference_days (
    publication_date TEXT PRIMARY KEY,
    state TEXT NOT NULL CHECK (state IN ('published','absent')),
    source TEXT NOT NULL,
    reference_e4 INTEGER,
    verified_at TEXT NOT NULL,
    CHECK ((state='published' AND reference_e4 > 0) OR (state='absent' AND reference_e4 IS NULL))
);
CREATE INDEX idx_fx_reference_published ON fx_reference_days(state,publication_date DESC);

ALTER TABLE usage_admissions ADD COLUMN fx_source TEXT;
ALTER TABLE usage_admissions ADD COLUMN fx_publication_date TEXT;
ALTER TABLE usage_admissions ADD COLUMN fx_reference_e4 INTEGER;
ALTER TABLE usage_admissions ADD COLUMN fx_applied_e4 INTEGER;
ALTER TABLE usage_admissions ADD COLUMN fx_temporary INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_admissions ADD COLUMN settlement_cause TEXT;
ALTER TABLE usage_admissions ADD COLUMN compensation_credits INTEGER;
ALTER TABLE usage_admissions ADD COLUMN compensation_lot_id TEXT;
ALTER TABLE usage_admissions ADD COLUMN compensation_expires_at TEXT;

CREATE TABLE credit_lots_new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('daily','monthly','bonus','purchased','voucher','compensation')),
    granted INTEGER NOT NULL CHECK (granted >= 0),
    remaining INTEGER NOT NULL CHECK (remaining >= 0 AND remaining <= granted),
    expires_at TEXT,
    created_at TEXT NOT NULL,
    coverage_id TEXT,
    window_start TEXT,
    issuance_cause TEXT,
    correlation_id TEXT
);
INSERT INTO credit_lots_new
SELECT id,user_id,kind,granted,remaining,expires_at,created_at,
       coverage_id,window_start,issuance_cause,correlation_id
FROM credit_lots;
DROP TABLE credit_lots;
ALTER TABLE credit_lots_new RENAME TO credit_lots;
CREATE INDEX idx_credit_lots_consumption ON credit_lots(user_id,expires_at,created_at,id);
CREATE UNIQUE INDEX idx_credit_lots_grant_window
ON credit_lots(user_id,coverage_id,kind,window_start)
WHERE coverage_id IS NOT NULL AND window_start IS NOT NULL
  AND issuance_cause IN ('coverage','lazy');

CREATE TABLE migration_0116_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0116_integrity_guard(problem)
SELECT 'foreign-key violation' WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0116_integrity_guard;
COMMIT;
PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;
BEGIN;
CREATE TABLE credit_lots_old (
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
INSERT INTO credit_lots_old
SELECT id,user_id,CASE kind WHEN 'compensation' THEN 'bonus' ELSE kind END,
       granted,remaining,expires_at,created_at,
       coverage_id,window_start,issuance_cause,correlation_id
FROM credit_lots;
DROP TABLE credit_lots;
ALTER TABLE credit_lots_old RENAME TO credit_lots;
CREATE INDEX idx_credit_lots_consumption ON credit_lots(user_id,expires_at,created_at,id);
CREATE UNIQUE INDEX idx_credit_lots_grant_window
ON credit_lots(user_id,coverage_id,kind,window_start)
WHERE coverage_id IS NOT NULL AND window_start IS NOT NULL
  AND issuance_cause IN ('coverage','lazy');
ALTER TABLE usage_admissions DROP COLUMN compensation_expires_at;
ALTER TABLE usage_admissions DROP COLUMN compensation_lot_id;
ALTER TABLE usage_admissions DROP COLUMN compensation_credits;
ALTER TABLE usage_admissions DROP COLUMN settlement_cause;
ALTER TABLE usage_admissions DROP COLUMN fx_temporary;
ALTER TABLE usage_admissions DROP COLUMN fx_applied_e4;
ALTER TABLE usage_admissions DROP COLUMN fx_reference_e4;
ALTER TABLE usage_admissions DROP COLUMN fx_publication_date;
ALTER TABLE usage_admissions DROP COLUMN fx_source;
DROP TABLE fx_reference_days;
CREATE TABLE migration_0116_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0116_integrity_guard(problem)
SELECT 'foreign-key violation' WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0116_integrity_guard;
COMMIT;
PRAGMA foreign_keys=ON;
