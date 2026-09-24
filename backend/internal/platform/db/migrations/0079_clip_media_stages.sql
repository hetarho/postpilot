-- +goose Up
CREATE TABLE clip_media_stages (
    id TEXT PRIMARY KEY,
    parent_job_id TEXT NOT NULL REFERENCES generation_jobs(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
    expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
    stage_key TEXT NOT NULL CHECK(stage_key IN ('prepare','render')),
    operation TEXT NOT NULL CHECK(operation = stage_key),
    contract_version INTEGER NOT NULL CHECK(contract_version > 0),
    input_digest TEXT NOT NULL,
    input_payload TEXT NOT NULL CHECK(json_valid(input_payload)),
    renderer_version TEXT NOT NULL,
    asset_version TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','succeeded','failed','cancelled')),
    current_attempt_id TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    created_at TEXT NOT NULL,
    queue_deadline_at TEXT NOT NULL,
    deadline_at TEXT NOT NULL,
    lease_ttl_ns INTEGER NOT NULL CHECK(lease_ttl_ns > 0),
    attempt_limit INTEGER NOT NULL CHECK(attempt_limit BETWEEN 1 AND 5),
    accepted_result TEXT,
    failure TEXT CHECK(failure IN ('worker_lost','wait_expired','deadline_exceeded','attempts_exhausted','invalid_input','invalid_output','internal')),
    UNIQUE(parent_job_id, stage_key),
    CHECK(created_at < queue_deadline_at AND queue_deadline_at <= deadline_at),
    CHECK(attempt_count <= attempt_limit),
    CHECK((state = 'succeeded') = (accepted_result IS NOT NULL))
);
CREATE INDEX clip_media_claim ON clip_media_stages(state, operation, contract_version, created_at);
CREATE TABLE clip_media_attempts (
    id TEXT PRIMARY KEY,
    stage_id TEXT NOT NULL REFERENCES clip_media_stages(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK(ordinal > 0),
    worker_id TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    lease_expires_at TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    selected_profile TEXT NOT NULL,
    runtime_manifest TEXT NOT NULL CHECK(json_valid(runtime_manifest)),
    progress INTEGER NOT NULL DEFAULT 0 CHECK(progress BETWEEN 0 AND 1000),
    outcome TEXT CHECK(outcome IN ('succeeded','expired','failed','cancelled')),
    UNIQUE(stage_id, ordinal)
);
CREATE INDEX clip_media_attempt_expiry ON clip_media_attempts(lease_expires_at) WHERE finished_at IS NULL;
CREATE TABLE clip_media_artifacts (
    attempt_id TEXT NOT NULL REFERENCES clip_media_attempts(id) ON DELETE CASCADE,
    slot TEXT NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL,
    max_bytes INTEGER NOT NULL CHECK(max_bytes > 0),
    state TEXT NOT NULL DEFAULT 'reserved' CHECK(state IN ('reserved','uploaded','accepted','cleanup')),
    created_at TEXT NOT NULL,
    PRIMARY KEY(attempt_id, slot)
);

-- +goose Down
DROP TABLE clip_media_artifacts;
DROP TABLE clip_media_attempts;
DROP TABLE clip_media_stages;
