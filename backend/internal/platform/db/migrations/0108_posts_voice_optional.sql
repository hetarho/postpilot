-- +goose NO TRANSACTION
-- +goose Up
-- POST r25 (T467): a post holds zero or one voice. `voice_id` becomes nullable — NULL is
-- 말투 없음, a real answer the server never overrides (POST-23) — and
-- `machine_baseline_voice_id`, read only by the finalization learning retired in T465,
-- leaves the table.
--
-- NO TRANSACTION plus an explicit PRAGMA for the same reason 0022 needed it: making a
-- column of a composite foreign key nullable means rebuilding `posts`, and `posts` is the
-- FK parent of images, uploads, videos, generation_jobs, model_experiments, post_measurements
-- and post_template_answers. Dropping the old table with foreign keys enabled would cascade
-- every one of those children away. The work still runs in one explicit transaction and
-- `foreign_key_check` proves the graph before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

-- Dropped before the rebuild: renaming a table back into place reparses every trigger that
-- names it.
DROP TRIGGER posts_require_active_voice_on_insert;
DROP TRIGGER posts_require_active_voice_on_reassign;
DROP TRIGGER templates_detach_posts_on_delete;

CREATE TABLE posts_new (
    slug         TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- NULL is 말투 없음 (POST-23). A composite FK with a NULL member is not checked, so the
    -- account guarantee below holds for every post that names a voice.
    voice_id     TEXT,
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
    target_length INTEGER CHECK (target_length IS NULL OR target_length > 0),
    finalized_revision INTEGER,
    finalized_at TEXT,
    -- NULL is the default and a real answer: 없음 is a choice, not a missing value, and the
    -- server never substitutes one.
    template_id  TEXT,
    target_language TEXT NOT NULL DEFAULT 'ko' CHECK (target_language IN ('ko','en')),
    content_language TEXT CHECK (content_language IS NULL OR content_language IN ('ko','en')),
    tag_count INTEGER,
    use_memory INTEGER NOT NULL DEFAULT 0,
    published_url TEXT,
    published_at TEXT CHECK ((published_url IS NULL) = (published_at IS NULL)),
    field TEXT,
    content_nouns TEXT CHECK (content_nouns IS NULL OR json_valid(content_nouns)),
    quality_rules TEXT CHECK (quality_rules IS NULL OR json_valid(quality_rules)),
    storyline TEXT CHECK (storyline IS NULL OR json_valid(storyline)),
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id),
    -- Deliberately NOT `ON DELETE SET NULL`: SQLite sets EVERY column of a composite child
    -- key to NULL, which would try to null user_id too and fail its NOT NULL constraint. The
    -- detach is the trigger below instead; this clause is here for the account guarantee —
    -- a post cannot name another account's template even if a service check is bypassed.
    FOREIGN KEY (template_id, user_id) REFERENCES templates(id, user_id)
);

INSERT INTO posts_new (slug, user_id, voice_id, title, memo, observations, content, status,
    created_at, updated_at, content_revision, machine_baseline, machine_baseline_revision,
    target_length, finalized_revision, finalized_at, template_id, target_language,
    content_language, tag_count, use_memory, published_url, published_at, field,
    content_nouns, quality_rules, storyline)
SELECT slug, user_id, voice_id, title, memo, observations, content, status,
    created_at, updated_at, content_revision, machine_baseline, machine_baseline_revision,
    target_length, finalized_revision, finalized_at, template_id, target_language,
    content_language, tag_count, use_memory, published_url, published_at, field,
    content_nouns, quality_rules, storyline
FROM posts;

DROP TABLE posts;
ALTER TABLE posts_new RENAME TO posts;

CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);
CREATE INDEX posts_template_idx ON posts(template_id);
CREATE INDEX posts_user_published_idx ON posts(user_id, published_at) WHERE status = 'published';
CREATE INDEX posts_voice_idx ON posts(voice_id);

-- A post may be created in, or moved to, an active voice or none: a named voice must be
-- live, and 말투 없음 names nothing to check.
-- +goose StatementBegin
CREATE TRIGGER posts_require_active_voice_on_insert
BEFORE INSERT ON posts
WHEN NEW.voice_id IS NOT NULL AND NOT EXISTS (
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
WHEN NEW.voice_id IS NOT NULL AND NEW.voice_id IS NOT OLD.voice_id AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'post voice must be active');
END;
-- +goose StatementEnd

-- Deleting a template detaches the posts that named it, in the delete's own transaction.
-- +goose StatementBegin
CREATE TRIGGER templates_detach_posts_on_delete
BEFORE DELETE ON templates
BEGIN
    UPDATE posts SET template_id = NULL
    WHERE template_id = OLD.id AND user_id = OLD.user_id;
END;
-- +goose StatementEnd

DROP TABLE IF EXISTS migration_0108_integrity_guard;
CREATE TABLE migration_0108_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0108_integrity_guard (problem)
SELECT 'rebuilding posts left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0108_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- Irreversible: a 말투 없음 post has no voice to put back, and the dropped baseline voice was
-- read by nothing.
SELECT 1;
