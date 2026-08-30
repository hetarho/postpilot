-- +goose Up
-- Naver publication is a separate durable aggregate: browser execution is leased to
-- an account-paired Mac and the irreversible external click has an explicit fence.

CREATE TABLE publishing_pairings (
    code_hash   TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    consumed_at TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX publishing_pairings_pending_idx
ON publishing_pairings(expires_at) WHERE consumed_at IS NULL;

CREATE TABLE publishing_agents (
    id                      TEXT PRIMARY KEY,
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash              TEXT NOT NULL UNIQUE,
    label                   TEXT NOT NULL,
    platform                TEXT NOT NULL CHECK (platform = 'naver_blog'),
    platform_account_id     TEXT NOT NULL DEFAULT '',
    platform_account_label  TEXT NOT NULL DEFAULT '',
    browser_label           TEXT NOT NULL DEFAULT '',
    categories_json         TEXT NOT NULL DEFAULT '[]',
    default_category_id     TEXT NOT NULL DEFAULT '',
    default_visibility      TEXT NOT NULL DEFAULT 'public'
                                CHECK (default_visibility IN ('public','private')),
    compatibility_ready     INTEGER NOT NULL DEFAULT 0
                                CHECK (compatibility_ready IN (0,1)),
    hermes_version          TEXT NOT NULL DEFAULT '',
    last_seen_at            TEXT,
    revoked_at              TEXT,
    created_at              TEXT NOT NULL,
    updated_at              TEXT NOT NULL,
    UNIQUE (id, user_id)
);
CREATE INDEX publishing_agents_user_idx ON publishing_agents(user_id, created_at DESC);

-- Reserve immutable object-key ownership before any R2 copy. Reservations are
-- deliberately durable for successful jobs so an RNG collision can never reuse
-- a historical job prefix and overwrite or delete another publication's JPEGs.
CREATE TABLE publish_job_ids (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    UNIQUE (id, user_id)
);

CREATE TABLE publish_jobs (
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
    -- Publication history deliberately outlives the source post. StartPublish is
    -- the application-level integrity boundary: after it freezes the manifest,
    -- deleting the post must neither delete nor block its publication record.
    FOREIGN KEY (id, user_id) REFERENCES publish_job_ids(id, user_id),
    FOREIGN KEY (agent_id, user_id) REFERENCES publishing_agents(id, user_id),
    UNIQUE (id, user_id)
);

-- Failed/canceled jobs may be retried. Every state that might already have published,
-- plus the one successful history row, keeps the post/platform slot occupied.
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

CREATE TABLE publish_assets (
    job_id      TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    ordinal     INTEGER NOT NULL CHECK (ordinal >= 0),
	filename    TEXT NOT NULL,
	source_filename TEXT NOT NULL,
    staged_key  TEXT NOT NULL UNIQUE,
    bytes       INTEGER NOT NULL CHECK (bytes > 0),
    created_at  TEXT NOT NULL,
    PRIMARY KEY (job_id, ordinal),
    UNIQUE (job_id, filename),
    FOREIGN KEY (job_id, user_id) REFERENCES publish_jobs(id, user_id) ON DELETE CASCADE
);
CREATE INDEX publish_assets_user_idx ON publish_assets(user_id, job_id);

-- +goose Down
DROP TABLE publish_assets;
DROP TABLE publish_jobs;
DROP TABLE publish_job_ids;
DROP TABLE publishing_agents;
DROP TABLE publishing_pairings;
