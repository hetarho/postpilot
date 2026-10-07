-- +goose Up
-- Private post-owned payload, never a usage/accounting record. At most one run
-- per stage is retained; all data cascades with the post or account.
CREATE TABLE post_request_captures (
    post_slug TEXT NOT NULL REFERENCES posts(slug) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id TEXT NOT NULL,
    call_id TEXT NOT NULL,
    call_sequence INTEGER NOT NULL CHECK(call_sequence >= 0),
    attachment_id TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL,
    input_revision INTEGER NOT NULL,
    content_revision INTEGER NOT NULL,
    source_fingerprint TEXT NOT NULL,
    source_plan_fingerprint TEXT NOT NULL,
    result_revision INTEGER,
    result_hash TEXT,
    plan_fingerprint TEXT,
    payload TEXT NOT NULL,
    PRIMARY KEY(post_slug, job_id, call_id)
);
CREATE INDEX post_request_captures_stage ON post_request_captures(post_slug, stage, call_sequence);

-- Payload erasure is irreversible for the admitted run, including a late writer.
CREATE TABLE post_request_capture_purges (
    post_slug TEXT NOT NULL REFERENCES posts(slug) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id TEXT NOT NULL,
    PRIMARY KEY(post_slug, job_id)
);

-- +goose Down
DROP TABLE post_request_captures;
DROP TABLE post_request_capture_purges;
