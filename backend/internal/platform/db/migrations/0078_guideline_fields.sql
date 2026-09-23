-- +goose NO TRANSACTION
-- +goose Up
-- Guidelines gain the fields scope (GUIDE-5): a guideline can be linked to blog fields as it
-- can be linked to templates, and the account's preset keeps its state beside them. This is the
-- delta's one rebuild, landed on its own; 0077 carries everything additive.
--
-- NO TRANSACTION plus an explicit PRAGMA for the reason 0022 gives: SQLite cannot alter the
-- CHECK on guidelines.scope, so the table is rebuilt, and `guidelines` is the FK parent of
-- guideline_templates with ON DELETE CASCADE. Dropping the old table with foreign keys enabled
-- would cascade every template link away. `PRAGMA foreign_keys` is a no-op inside a
-- transaction, so goose must not open one for us; the work still runs in one explicit
-- transaction and `foreign_key_check` proves the graph before it commits.
--
-- Every row keeps its id, text, scope and timestamps. guideline_templates itself is not
-- touched: its REFERENCES guidelines(id, user_id) resolves to the renamed table, so every link
-- survives and still cascades.

PRAGMA foreign_keys=OFF;

BEGIN;

-- The current shape with only the CHECK widened.
CREATE TABLE guidelines_new (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('global','templates','fields')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, user_id),
    UNIQUE (user_id, text)
);

INSERT INTO guidelines_new (id, user_id, text, scope, created_at, updated_at)
SELECT id, user_id, text, scope, created_at, updated_at FROM guidelines;

DROP INDEX idx_guidelines_user_created;
DROP TABLE guidelines;
ALTER TABLE guidelines_new RENAME TO guidelines;
CREATE INDEX idx_guidelines_user_created ON guidelines(user_id, created_at, id);

-- A fields guideline's links, in guideline_templates' shape: one row per field, and a composite
-- key that keeps a link inside its guideline's account even if a service check is bypassed.
-- `field` is the ASCII id and carries no CHECK: the product's list lives in code, and its
-- parser owns validity.
CREATE TABLE guideline_fields (
    guideline_id TEXT NOT NULL,
    field        TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    PRIMARY KEY (guideline_id, field),
    FOREIGN KEY (guideline_id, user_id) REFERENCES guidelines(id, user_id) ON DELETE CASCADE
);

-- Resolution reads one field's links for one account, as idx_guideline_templates_template
-- serves the template group.
CREATE INDEX idx_guideline_fields_field ON guideline_fields(user_id, field);

-- The preset's state. The preset is not a guideline row: its text is a product constant that
-- is never stored, so it spends neither the account cap nor text uniqueness (GUIDE-39).
-- Nothing is seeded (GUIDE-34), and a missing row reads as the preset off with no field.
CREATE TABLE guideline_presets (
    user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled    INTEGER NOT NULL CHECK (enabled IN (0,1)),
    updated_at TEXT NOT NULL
);

-- The preset's fields hang off its row, so a field can never exist without the row saying
-- whether the preset is on.
CREATE TABLE guideline_preset_fields (
    user_id TEXT NOT NULL REFERENCES guideline_presets(user_id) ON DELETE CASCADE,
    field   TEXT NOT NULL,
    PRIMARY KEY (user_id, field)
);

DROP TABLE IF EXISTS migration_0078_integrity_guard;
CREATE TABLE migration_0078_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0078_integrity_guard (problem)
SELECT 'rebuilding guidelines left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0078_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- The fields scope and the preset go away. A fields guideline becomes a templates guideline
-- with no links, the state the product already has for a scope that reaches nothing: it
-- reaches no prompt until it is rescoped, rather than silently widening to every post, as 0022
-- did for purposes. Every template link is kept.
PRAGMA foreign_keys=OFF;

BEGIN;

DROP TABLE guideline_preset_fields;
DROP TABLE guideline_presets;
DROP INDEX idx_guideline_fields_field;
DROP TABLE guideline_fields;

CREATE TABLE guidelines_old (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('global','templates')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, user_id),
    UNIQUE (user_id, text)
);
INSERT INTO guidelines_old (id, user_id, text, scope, created_at, updated_at)
SELECT id, user_id, text,
       CASE scope WHEN 'fields' THEN 'templates' ELSE scope END,
       created_at, updated_at
FROM guidelines;
DROP INDEX idx_guidelines_user_created;
DROP TABLE guidelines;
ALTER TABLE guidelines_old RENAME TO guidelines;
CREATE INDEX idx_guidelines_user_created ON guidelines(user_id, created_at, id);

DROP TABLE IF EXISTS migration_0078_down_integrity_guard;
CREATE TABLE migration_0078_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0078_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0078_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
