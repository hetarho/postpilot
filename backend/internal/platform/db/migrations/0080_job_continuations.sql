-- +goose Up
CREATE TABLE job_continuations (
    job_id TEXT PRIMARY KEY REFERENCES generation_jobs(id) ON DELETE CASCADE,
    wait_key TEXT NOT NULL CHECK(length(wait_key) BETWEEN 1 AND 256),
    state TEXT NOT NULL CHECK(state IN ('waiting','ready','claimed')),
    resume_policy TEXT NOT NULL DEFAULT 'fail_on_interrupt' CHECK(resume_policy IN ('fail_on_interrupt','replay_safe')),
    ready_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK((state='waiting' AND ready_at IS NULL) OR (state IN ('ready','claimed') AND ready_at IS NOT NULL))
);
CREATE INDEX job_continuations_ready ON job_continuations(state,ready_at,job_id);

-- +goose Down
DROP TABLE job_continuations;
