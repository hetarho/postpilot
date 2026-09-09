-- +goose NO TRANSACTION
-- +goose Up
-- PUB-13 r4 puts `filling_settings` between `uploading_photos` and `committing`: tags,
-- category and visibility live behind a layer that covers the editor, so the body and every
-- photo are complete before it opens (PUB-37). The server enforces single-step progress, so
-- a stage the column cannot hold would make `uploading_photos → committing` an illegal
-- two-step jump and no job could ever reach the fence.
--
-- Widening a CHECK constraint in SQLite means rebuilding the table, and `publish_jobs` is an
-- FK parent: `publish_assets` references (id, user_id) ON DELETE CASCADE, so dropping the old
-- table with foreign keys on would cascade-delete every asset row. `PRAGMA foreign_keys` is a
-- no-op inside a transaction, so goose must not open one — this follows 0019's precedent,
-- doing the work in one explicit transaction with `foreign_key_check` proving the graph
-- before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE publish_jobs_new (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug           TEXT NOT NULL,
    post_created_at     TEXT NOT NULL,
    agent_id            TEXT NOT NULL,
    platform            TEXT NOT NULL CHECK (platform = 'naver_blog'),
    status              TEXT NOT NULL CHECK (status IN (
                            'queued','running','published','failed','needs_attention',
                            'outcome_unknown','canceled')),
    stage               TEXT NOT NULL CHECK (stage IN (
                            'queued','claimed','preparing','opening_editor','filling_content',
                            'uploading_photos','filling_settings','committing','verifying',
                            'published')),
    progress_seq        INTEGER NOT NULL DEFAULT 0 CHECK (progress_seq >= 0),
    attempt             INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    content_revision    INTEGER NOT NULL CHECK (content_revision > 0),
    manifest_json       TEXT,
    settings_json       TEXT NOT NULL,
    lease_token_hash    TEXT,
    lease_expires_at    TEXT,
    error_code          TEXT,
    error_message       TEXT,
    platform_post_url   TEXT,
    created_at          TEXT NOT NULL,
    claimed_at          TEXT,
    committed_at        TEXT,
    published_at        TEXT,
    updated_at          TEXT NOT NULL,
    -- Added by 0012 and reproduced here verbatim, constraints included.
    target_language       TEXT CHECK (target_language IS NULL OR target_language IN ('ko','en')),
    content_language      TEXT CHECK (content_language IS NULL OR content_language IN ('ko','en')),
    voice_source_language TEXT CHECK (voice_source_language IS NULL OR voice_source_language IN ('ko','en')),
    error_reason          TEXT,
    error_params          TEXT CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object')),
    technical_detail      TEXT,
    -- Publication history deliberately outlives the source post. StartPublish is
    -- the application-level integrity boundary: after it freezes the manifest,
    -- deleting the post must neither delete nor block its publication record.
    FOREIGN KEY (id, user_id) REFERENCES publish_job_ids(id, user_id),
    FOREIGN KEY (agent_id, user_id) REFERENCES publishing_agents(id, user_id),
    UNIQUE (id, user_id)
);

INSERT INTO publish_jobs_new (
    id, user_id, post_slug, post_created_at, agent_id, platform, status, stage,
    progress_seq, attempt, content_revision, manifest_json, settings_json,
    lease_token_hash, lease_expires_at, error_code, error_message, platform_post_url,
    created_at, claimed_at, committed_at, published_at, updated_at,
    target_language, content_language, voice_source_language,
    error_reason, error_params, technical_detail)
SELECT
    id, user_id, post_slug, post_created_at, agent_id, platform, status, stage,
    progress_seq, attempt, content_revision, manifest_json, settings_json,
    lease_token_hash, lease_expires_at, error_code, error_message, platform_post_url,
    created_at, claimed_at, committed_at, published_at, updated_at,
    target_language, content_language, voice_source_language,
    error_reason, error_params, technical_detail
FROM publish_jobs;

DROP TABLE publish_jobs;
ALTER TABLE publish_jobs_new RENAME TO publish_jobs;

-- Every index from 0010, recreated verbatim: a rebuild drops them with the old table.
CREATE UNIQUE INDEX publish_jobs_one_live_or_success_idx
ON publish_jobs(user_id, post_slug, post_created_at, platform)
WHERE status IN ('queued','running','published','needs_attention','outcome_unknown');
CREATE INDEX publish_jobs_agent_queue_idx
ON publish_jobs(agent_id, status, created_at, id);
CREATE INDEX publish_jobs_post_history_idx
ON publish_jobs(user_id, post_slug, post_created_at, created_at DESC, id DESC);
CREATE INDEX publish_jobs_deleted_post_history_idx
ON publish_jobs(user_id, post_slug, created_at DESC, id DESC);
CREATE INDEX publish_jobs_retryable_idx
ON publish_jobs(user_id, updated_at DESC, id DESC)
WHERE status='needs_attention' AND committed_at IS NULL AND manifest_json IS NOT NULL;
CREATE INDEX publish_jobs_expired_running_idx
ON publish_jobs(lease_expires_at)
WHERE status='running';
CREATE INDEX publish_jobs_terminal_cleanup_idx
ON publish_jobs(status, id)
WHERE status IN ('published','failed','outcome_unknown','canceled');

DROP TABLE IF EXISTS migration_0037_up_integrity_guard;
CREATE TABLE migration_0037_up_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0037_up_integrity_guard (problem)
SELECT 'migration left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0037_up_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- Rolling back narrows the CHECK again, so any job parked in `filling_settings` would become
-- unrepresentable. Such a job is mid-run and pre-commit, which PUB-14 already requeues
-- safely, so it is moved back to `uploading_photos` — the stage it legally came from — rather
-- than being dropped or left to violate the constraint.

PRAGMA foreign_keys=OFF;

BEGIN;

UPDATE publish_jobs
SET stage='uploading_photos',
    progress_seq=progress_seq+1,
    updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE stage='filling_settings';

CREATE TABLE publish_jobs_old (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug           TEXT NOT NULL,
    post_created_at     TEXT NOT NULL,
    agent_id            TEXT NOT NULL,
    platform            TEXT NOT NULL CHECK (platform = 'naver_blog'),
    status              TEXT NOT NULL CHECK (status IN (
                            'queued','running','published','failed','needs_attention',
                            'outcome_unknown','canceled')),
    stage               TEXT NOT NULL CHECK (stage IN (
                            'queued','claimed','preparing','opening_editor','filling_content',
                            'uploading_photos','committing','verifying','published')),
    progress_seq        INTEGER NOT NULL DEFAULT 0 CHECK (progress_seq >= 0),
    attempt             INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    content_revision    INTEGER NOT NULL CHECK (content_revision > 0),
    manifest_json       TEXT,
    settings_json       TEXT NOT NULL,
    lease_token_hash    TEXT,
    lease_expires_at    TEXT,
    error_code          TEXT,
    error_message       TEXT,
    platform_post_url   TEXT,
    created_at          TEXT NOT NULL,
    claimed_at          TEXT,
    committed_at        TEXT,
    published_at        TEXT,
    updated_at          TEXT NOT NULL,
    target_language       TEXT CHECK (target_language IS NULL OR target_language IN ('ko','en')),
    content_language      TEXT CHECK (content_language IS NULL OR content_language IN ('ko','en')),
    voice_source_language TEXT CHECK (voice_source_language IS NULL OR voice_source_language IN ('ko','en')),
    error_reason          TEXT,
    error_params          TEXT CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object')),
    technical_detail      TEXT,
    FOREIGN KEY (id, user_id) REFERENCES publish_job_ids(id, user_id),
    FOREIGN KEY (agent_id, user_id) REFERENCES publishing_agents(id, user_id),
    UNIQUE (id, user_id)
);

INSERT INTO publish_jobs_old (
    id, user_id, post_slug, post_created_at, agent_id, platform, status, stage,
    progress_seq, attempt, content_revision, manifest_json, settings_json,
    lease_token_hash, lease_expires_at, error_code, error_message, platform_post_url,
    created_at, claimed_at, committed_at, published_at, updated_at,
    target_language, content_language, voice_source_language,
    error_reason, error_params, technical_detail)
SELECT
    id, user_id, post_slug, post_created_at, agent_id, platform, status, stage,
    progress_seq, attempt, content_revision, manifest_json, settings_json,
    lease_token_hash, lease_expires_at, error_code, error_message, platform_post_url,
    created_at, claimed_at, committed_at, published_at, updated_at,
    target_language, content_language, voice_source_language,
    error_reason, error_params, technical_detail
FROM publish_jobs;

DROP TABLE publish_jobs;
ALTER TABLE publish_jobs_old RENAME TO publish_jobs;

CREATE UNIQUE INDEX publish_jobs_one_live_or_success_idx
ON publish_jobs(user_id, post_slug, post_created_at, platform)
WHERE status IN ('queued','running','published','needs_attention','outcome_unknown');
CREATE INDEX publish_jobs_agent_queue_idx
ON publish_jobs(agent_id, status, created_at, id);
CREATE INDEX publish_jobs_post_history_idx
ON publish_jobs(user_id, post_slug, post_created_at, created_at DESC, id DESC);
CREATE INDEX publish_jobs_deleted_post_history_idx
ON publish_jobs(user_id, post_slug, created_at DESC, id DESC);
CREATE INDEX publish_jobs_retryable_idx
ON publish_jobs(user_id, updated_at DESC, id DESC)
WHERE status='needs_attention' AND committed_at IS NULL AND manifest_json IS NOT NULL;
CREATE INDEX publish_jobs_expired_running_idx
ON publish_jobs(lease_expires_at)
WHERE status='running';
CREATE INDEX publish_jobs_terminal_cleanup_idx
ON publish_jobs(status, id)
WHERE status IN ('published','failed','outcome_unknown','canceled');

DROP TABLE IF EXISTS migration_0037_down_integrity_guard;
CREATE TABLE migration_0037_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0037_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0037_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
