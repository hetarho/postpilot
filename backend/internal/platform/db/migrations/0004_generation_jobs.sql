-- +goose Up
-- The composite key lets generation_jobs prove that a post target and its recorded
-- owner belong together. `slug` is already globally unique; this second key exists
-- specifically for the composite foreign key below.
CREATE UNIQUE INDEX posts_slug_user_idx ON posts(slug, user_id);

CREATE TABLE generation_jobs (
    id             TEXT PRIMARY KEY,
    post_slug      TEXT,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed')),
    stage          TEXT,
    progress_done  INTEGER NOT NULL DEFAULT 0,
    progress_total INTEGER NOT NULL DEFAULT 0,
    error          TEXT,
    observe_model  TEXT,
    write_model    TEXT,
    payload        TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    started_at     TEXT,
    finished_at    TEXT,
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE
);

-- The partial unique indexes close the race between the guard query and insert. A
-- terminal row leaves the target free for a retry while retaining its history.
CREATE UNIQUE INDEX generation_jobs_active_post_idx
    ON generation_jobs(post_slug)
    WHERE post_slug IS NOT NULL AND status IN ('queued', 'running');
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx
    ON generation_jobs(user_id, kind)
    WHERE post_slug IS NULL AND status IN ('queued', 'running');
CREATE INDEX generation_jobs_queue_idx ON generation_jobs(status, created_at);

-- +goose Down
DROP TABLE generation_jobs;
DROP INDEX posts_slug_user_idx;
