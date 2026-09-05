-- +goose NO TRANSACTION
-- +goose Up
-- Post templates (change 25). A template replaces the purpose: instead of prose about the
-- shape of a post, it IS the shape — literal text, positions reserved for content the app
-- cannot invent, per-photo repetition, and the places prose gets written. Voice keeps
-- deciding how sentences sound and a guideline keeps deciding what to avoid.
--
-- NO TRANSACTION plus an explicit PRAGMA for the same reason 0009/0011 needed it: `posts`
-- carries a COMPOSITE foreign key, which SQLite can only change by rebuilding the table, and
-- `posts` is the FK parent of images, uploads, generation_jobs, model_experiments and several
-- voice tables with ON DELETE CASCADE. Dropping the old table with foreign keys enabled would
-- cascade every one of those children away. `PRAGMA foreign_keys` is a no-op inside a
-- transaction, so goose must not open one for us; the work still runs in one explicit
-- transaction and `foreign_key_check` proves the graph before it commits.
--
-- THIS MIGRATION IS DESTRUCTIVE BY DECISION. Every `purposes` row and every purpose-scoped
-- guideline link is dropped with no conversion, and every post's assignment becomes NULL.
-- The deployed database was inspected first (2 purposes, 2 finalized posts, 0 guideline
-- links) and the owner chose to drop rather than convert. `model_experiments.purpose_name`
-- is the one exception: it is RENAMED, not cleared, because a frozen comparison record must
-- keep saying what both candidates were given (change 25 AC12).

PRAGMA foreign_keys=OFF;

BEGIN;

-- Dropped before the rebuilds below: renaming a table back into place reparses every trigger,
-- and these name tables that do not exist yet at that point.
DROP TRIGGER purposes_detach_posts_on_delete;
DROP TRIGGER posts_require_active_voice_on_reassign;
DROP TRIGGER posts_require_active_voice_on_insert;

CREATE TABLE templates (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- The template body in the grammar of spec/legacy/tech/post-template-grammar.md. Stored as the
    -- author wrote it: the parser keeps every literal's raw slice so the builder can
    -- re-serialize this exact string back.
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- The composite target posts.template_id points at: it carries the account through the FK,
-- so a post can never name another account's template even if a service check is bypassed.
CREATE UNIQUE INDEX templates_id_user ON templates(id, user_id);
-- One display name per account. Like a purpose and unlike a voice there is no tombstone, so
-- this needs no partial predicate: a deleted template frees its name immediately.
CREATE UNIQUE INDEX templates_user_name ON templates(user_id, name);

----------------------------------------------------------------------------------
-- guidelines: the scope target moves from purposes to templates.
----------------------------------------------------------------------------------
-- The link rows go first. Their FK parent is `purposes`, which this migration drops, and
-- there is nothing to convert them into: a purpose id is not a template id.
DROP INDEX idx_guideline_purposes_purpose;
DROP TABLE guideline_purposes;

-- `scope`'s CHECK names the old kind, and SQLite cannot alter a CHECK constraint, so the
-- table is rebuilt. A guideline that was purpose-scoped becomes template-scoped with an empty
-- link set, which the product already has a state for: 적용 대상 없음 — it reaches no prompt
-- until it is rescoped, rather than silently widening to every post.
CREATE TABLE guidelines_new (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('global','templates')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, user_id),
    UNIQUE (user_id, text)
);

INSERT INTO guidelines_new (id, user_id, text, scope, created_at, updated_at)
SELECT id, user_id, text,
       CASE scope WHEN 'purposes' THEN 'templates' ELSE scope END,
       created_at, updated_at
FROM guidelines;

DROP INDEX idx_guidelines_user_created;
DROP TABLE guidelines;
ALTER TABLE guidelines_new RENAME TO guidelines;
CREATE INDEX idx_guidelines_user_created ON guidelines(user_id, created_at, id);

-- Created after the rename so its foreign key names the table that will still be there.
-- Both keys are composite for the reason 0014 gives: a link can then never cross accounts
-- even if a service check is bypassed. ON DELETE CASCADE deletes only link rows, so deleting
-- a template unlinks it from every guideline while the guideline rows survive.
CREATE TABLE guideline_templates (
    guideline_id TEXT NOT NULL,
    template_id  TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    PRIMARY KEY (guideline_id, template_id),
    FOREIGN KEY (guideline_id, user_id) REFERENCES guidelines(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (template_id, user_id) REFERENCES templates(id, user_id) ON DELETE CASCADE
);

CREATE INDEX idx_guideline_templates_template ON guideline_templates(user_id, template_id);

----------------------------------------------------------------------------------
-- posts: purpose_id becomes template_id.
----------------------------------------------------------------------------------
CREATE TABLE posts_new (
    slug         TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id     TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    memo         TEXT NOT NULL DEFAULT '',
    observations TEXT,
    content      TEXT,
    status       TEXT NOT NULL DEFAULT 'draft',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    content_revision INTEGER NOT NULL DEFAULT 0,
    machine_baseline TEXT,
    machine_baseline_revision INTEGER NOT NULL DEFAULT 0,
    machine_baseline_voice_id TEXT,
    target_length INTEGER CHECK (target_length IS NULL OR target_length > 0),
    finalized_revision INTEGER,
    finalized_at TEXT,
    -- NULL is the default and a real answer: 없음 is a choice, not a missing value, and the
    -- server never substitutes one.
    template_id  TEXT,
    target_language TEXT NOT NULL DEFAULT 'ko' CHECK (target_language IN ('ko','en')),
    content_language TEXT CHECK (content_language IS NULL OR content_language IN ('ko','en')),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    -- Deliberately NOT `ON DELETE SET NULL`: SQLite sets EVERY column of a composite child
    -- key to NULL, which would try to null user_id too and fail its NOT NULL constraint. The
    -- detach is the trigger below instead; this clause is here for the account guarantee —
    -- a post cannot name another account's template even if a service check is bypassed.
    FOREIGN KEY (template_id, user_id) REFERENCES templates(id, user_id)
);

-- template_id starts NULL for every post: purposes are dropped, so there is nothing an
-- assignment could point at. Title, memo, content, photos, revisions and finalization are
-- carried across untouched — no post loses anything but its brief.
INSERT INTO posts_new (slug, user_id, voice_id, title, memo, observations, content, status,
                       created_at, updated_at, content_revision, machine_baseline,
                       machine_baseline_revision, machine_baseline_voice_id, target_length,
                       finalized_revision, finalized_at, template_id, target_language,
                       content_language)
SELECT slug, user_id, voice_id, title, memo, observations, content, status,
       created_at, updated_at, content_revision, machine_baseline,
       machine_baseline_revision, machine_baseline_voice_id, target_length,
       finalized_revision, finalized_at, NULL, target_language, content_language
FROM posts;

DROP TABLE posts;
ALTER TABLE posts_new RENAME TO posts;
CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);
CREATE INDEX posts_voice_idx ON posts(voice_id);
-- The delete path counts and detaches by template, and it is the only query that reads posts
-- this way; without the index it would scan every post the account owns.
CREATE INDEX posts_template_idx ON posts(template_id);

-- Recreated because SQLite drops a table's triggers with the table. These two are 0009's,
-- byte for byte: a post may only be created in, or moved to, an active voice.
-- +goose StatementBegin
CREATE TRIGGER posts_require_active_voice_on_insert
BEFORE INSERT ON posts
WHEN NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'post voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posts_require_active_voice_on_reassign
BEFORE UPDATE OF voice_id ON posts
WHEN NEW.voice_id <> OLD.voice_id AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'post voice must be active');
END;
-- +goose StatementEnd

-- Deleting a template detaches the posts that named it, in the delete's own transaction.
-- Deleting a template must never delete a post, and the frozen job payloads and experiment
-- snapshots keep the template's text regardless.
-- +goose StatementBegin
CREATE TRIGGER templates_detach_posts_on_delete
BEFORE DELETE ON templates
BEGIN
    UPDATE posts SET template_id = NULL
    WHERE template_id = OLD.id AND user_id = OLD.user_id;
END;
-- +goose StatementEnd

----------------------------------------------------------------------------------
-- The frozen comparison record is renamed, not cleared.
----------------------------------------------------------------------------------
ALTER TABLE model_experiments RENAME COLUMN purpose_name TO template_name;

DROP TABLE purposes;

DROP TABLE IF EXISTS migration_0022_integrity_guard;
CREATE TABLE migration_0022_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0022_integrity_guard (problem)
SELECT 'rebuilding posts or guidelines left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0022_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- The rollback restores the SHAPE, never the data: purposes were dropped going up, so coming
-- back down produces an empty `purposes` table, `posts.purpose_id` all NULL, and no guideline
-- scope links. It exists so a bad deploy can be reverted, not so the briefs come back.
PRAGMA foreign_keys=OFF;

BEGIN;

DROP TRIGGER templates_detach_posts_on_delete;
DROP TRIGGER posts_require_active_voice_on_reassign;
DROP TRIGGER posts_require_active_voice_on_insert;

CREATE TABLE purposes (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX purposes_id_user ON purposes(id, user_id);
CREATE UNIQUE INDEX purposes_user_name ON purposes(user_id, name);

DROP INDEX idx_guideline_templates_template;
DROP TABLE guideline_templates;

CREATE TABLE guidelines_old (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text       TEXT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('global','purposes')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, user_id),
    UNIQUE (user_id, text)
);
INSERT INTO guidelines_old (id, user_id, text, scope, created_at, updated_at)
SELECT id, user_id, text,
       CASE scope WHEN 'templates' THEN 'purposes' ELSE scope END,
       created_at, updated_at
FROM guidelines;
DROP INDEX idx_guidelines_user_created;
DROP TABLE guidelines;
ALTER TABLE guidelines_old RENAME TO guidelines;
CREATE INDEX idx_guidelines_user_created ON guidelines(user_id, created_at, id);

CREATE TABLE guideline_purposes (
    guideline_id TEXT NOT NULL,
    purpose_id   TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    PRIMARY KEY (guideline_id, purpose_id),
    FOREIGN KEY (guideline_id, user_id) REFERENCES guidelines(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (purpose_id, user_id) REFERENCES purposes(id, user_id) ON DELETE CASCADE
);
CREATE INDEX idx_guideline_purposes_purpose ON guideline_purposes(user_id, purpose_id);

CREATE TABLE posts_old (
    slug         TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id     TEXT NOT NULL,
    title        TEXT NOT NULL DEFAULT '',
    memo         TEXT NOT NULL DEFAULT '',
    observations TEXT,
    content      TEXT,
    status       TEXT NOT NULL DEFAULT 'draft',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    content_revision INTEGER NOT NULL DEFAULT 0,
    machine_baseline TEXT,
    machine_baseline_revision INTEGER NOT NULL DEFAULT 0,
    machine_baseline_voice_id TEXT,
    target_length INTEGER CHECK (target_length IS NULL OR target_length > 0),
    finalized_revision INTEGER,
    finalized_at TEXT,
    purpose_id   TEXT,
    target_language TEXT NOT NULL DEFAULT 'ko' CHECK (target_language IN ('ko','en')),
    content_language TEXT CHECK (content_language IS NULL OR content_language IN ('ko','en')),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    FOREIGN KEY (purpose_id, user_id) REFERENCES purposes(id, user_id)
);

INSERT INTO posts_old (slug, user_id, voice_id, title, memo, observations, content, status,
                       created_at, updated_at, content_revision, machine_baseline,
                       machine_baseline_revision, machine_baseline_voice_id, target_length,
                       finalized_revision, finalized_at, purpose_id, target_language,
                       content_language)
SELECT slug, user_id, voice_id, title, memo, observations, content, status,
       created_at, updated_at, content_revision, machine_baseline,
       machine_baseline_revision, machine_baseline_voice_id, target_length,
       finalized_revision, finalized_at, NULL, target_language, content_language
FROM posts;

DROP TABLE posts;
ALTER TABLE posts_old RENAME TO posts;
CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);
CREATE INDEX posts_voice_idx ON posts(voice_id);
CREATE INDEX posts_purpose_idx ON posts(purpose_id);

-- +goose StatementBegin
CREATE TRIGGER posts_require_active_voice_on_insert
BEFORE INSERT ON posts
WHEN NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'post voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posts_require_active_voice_on_reassign
BEFORE UPDATE OF voice_id ON posts
WHEN NEW.voice_id <> OLD.voice_id AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'post voice must be active');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER purposes_detach_posts_on_delete
BEFORE DELETE ON purposes
BEGIN
    UPDATE posts SET purpose_id = NULL
    WHERE purpose_id = OLD.id AND user_id = OLD.user_id;
END;
-- +goose StatementEnd

ALTER TABLE model_experiments RENAME COLUMN template_name TO purpose_name;

DROP TABLE templates;

DROP TABLE IF EXISTS migration_0022_down_integrity_guard;
CREATE TABLE migration_0022_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0022_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0022_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
