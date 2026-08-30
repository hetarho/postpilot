-- +goose NO TRANSACTION
-- +goose Up
-- Post purposes (plan 11). A purpose is a reusable account-owned brief — what a kind of
-- post is for and how to write it — and a post optionally names one. Voice keeps deciding
-- how sentences sound; the purpose decides genre, structure and required content.
--
-- NO TRANSACTION plus an explicit PRAGMA is the same reason 0009 needed it: `posts` gains a
-- COMPOSITE foreign key, which SQLite can only add by rebuilding the table, and `posts` is
-- the FK parent of images, uploads, generation_jobs, model_experiments and several voice
-- tables with ON DELETE CASCADE. Dropping the old table with foreign keys enabled would
-- cascade every one of those children away. `PRAGMA foreign_keys` is a no-op inside a
-- transaction, so goose must not open one for us; the work still runs in one explicit
-- transaction and `foreign_key_check` proves the graph before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE purposes (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- The composite target posts.purpose_id points at: it carries the account through the FK,
-- so a post can never name another account's purpose even if a service check is bypassed.
CREATE UNIQUE INDEX purposes_id_user ON purposes(id, user_id);
-- One display name per account. Unlike a voice there is no tombstone, so this needs no
-- partial predicate: a deleted purpose is gone and its name is free again immediately.
CREATE UNIQUE INDEX purposes_user_name ON purposes(user_id, name);

-- The purpose a write comparison was frozen for, kept by NAME beside the frozen voice id.
-- By name, and stored rather than derived from input_snapshot, because the detail must keep
-- saying which brief both candidates were given after the purpose is renamed or deleted and
-- after the retention sweep has cleared the snapshot itself.
ALTER TABLE model_experiments ADD COLUMN purpose_name TEXT NOT NULL DEFAULT '';

----------------------------------------------------------------------------------
-- posts: the optional purpose assignment.
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
    purpose_id   TEXT,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    -- Deliberately NOT `ON DELETE SET NULL`: SQLite sets EVERY column of a composite child
    -- key to NULL, which would try to null user_id too and fail its NOT NULL constraint. The
    -- detach is the trigger below instead; this clause is here for the account guarantee —
    -- a post cannot name another account's purpose even if a service check is bypassed.
    FOREIGN KEY (purpose_id, user_id) REFERENCES purposes(id, user_id)
);

INSERT INTO posts_new (slug, user_id, voice_id, title, memo, observations, content, status,
                       created_at, updated_at, content_revision, machine_baseline,
                       machine_baseline_revision, machine_baseline_voice_id, target_length,
                       finalized_revision, finalized_at, purpose_id)
SELECT slug, user_id, voice_id, title, memo, observations, content, status,
       created_at, updated_at, content_revision, machine_baseline,
       machine_baseline_revision, machine_baseline_voice_id, target_length,
       finalized_revision, finalized_at, NULL
FROM posts;

DROP TABLE posts;
ALTER TABLE posts_new RENAME TO posts;
CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);
CREATE INDEX posts_voice_idx ON posts(voice_id);
-- The delete path counts and detaches by purpose, and it is the only query that reads posts
-- this way; without the index it would scan every post the account owns.
CREATE INDEX posts_purpose_idx ON posts(purpose_id);

-- Recreated because SQLite drops a table's triggers with the table. They are 0009's, byte
-- for byte: a post may only be created in, or moved to, an active voice.
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

-- Deleting a purpose detaches the posts that named it, in the delete's own transaction.
-- Deleting a purpose must never delete a post, and the frozen job payloads and experiment
-- snapshots keep the brief's text regardless.
-- +goose StatementBegin
CREATE TRIGGER purposes_detach_posts_on_delete
BEFORE DELETE ON purposes
BEGIN
    UPDATE posts SET purpose_id = NULL
    WHERE purpose_id = OLD.id AND user_id = OLD.user_id;
END;
-- +goose StatementEnd

DROP TABLE IF EXISTS migration_0011_integrity_guard;
CREATE TABLE migration_0011_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0011_integrity_guard (problem)
SELECT 'rebuilding posts left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0011_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- Dropping purposes is safe in either direction: no post owns one, so the rollback loses
-- the briefs and the assignments and nothing else.
PRAGMA foreign_keys=OFF;

BEGIN;

-- Dropped before the posts rebuild, not with the purposes table: renaming posts_old back to
-- posts reparses every trigger, and this one names a `posts` that does not exist yet at that
-- point in the rollback.
DROP TRIGGER purposes_detach_posts_on_delete;
DROP TRIGGER posts_require_active_voice_on_reassign;
DROP TRIGGER posts_require_active_voice_on_insert;

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
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);

INSERT INTO posts_old (slug, user_id, voice_id, title, memo, observations, content, status,
                       created_at, updated_at, content_revision, machine_baseline,
                       machine_baseline_revision, machine_baseline_voice_id, target_length,
                       finalized_revision, finalized_at)
SELECT slug, user_id, voice_id, title, memo, observations, content, status,
       created_at, updated_at, content_revision, machine_baseline,
       machine_baseline_revision, machine_baseline_voice_id, target_length,
       finalized_revision, finalized_at
FROM posts;

DROP TABLE posts;
ALTER TABLE posts_old RENAME TO posts;
CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);
CREATE INDEX posts_voice_idx ON posts(voice_id);

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

ALTER TABLE model_experiments DROP COLUMN purpose_name;

DROP TABLE purposes;

DROP TABLE IF EXISTS migration_0011_down_integrity_guard;
CREATE TABLE migration_0011_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0011_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0011_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
