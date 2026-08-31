-- +goose Up
-- Plan-based authorization and the usage ledger (plan 17). The plan column defaults to
-- 'free' because that is what a newly provisioned account gets; the one-time backfill
-- below then lifts every account that existed BEFORE this migration to 'master', since
-- those are the operator's own accounts and nothing about their authority may regress.
ALTER TABLE users ADD COLUMN plan TEXT NOT NULL DEFAULT 'free'
  CHECK (plan IN ('free','basic','max','master'));

UPDATE users SET plan = 'master';

-- One row per ADMITTED job start. The daily-count axis counts these rows rather than
-- generation_jobs, so a refused start consumes nothing, an A/B comparison that fans out
-- to two candidate calls consumes exactly one, and a job that later fails still consumed
-- the start it may already have spent tokens on.
CREATE TABLE usage_admissions (
    id         INTEGER PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    job_id     TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- One row per completed server-side LLM call, including a failed call whose usage the
-- provider still reported. The two budget axes are SUM(cost_microusd) over this table,
-- which is why the ledger — not a counter on the user row — is the quota source: a
-- window can always be re-derived.
CREATE TABLE usage_events (
    id                INTEGER PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind              TEXT NOT NULL,
    job_id            TEXT NOT NULL,
    stage             TEXT NOT NULL,
    model             TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    cost_microusd     INTEGER NOT NULL,
    cost_source       TEXT NOT NULL CHECK (cost_source IN ('reported','estimated','unavailable')),
    created_at        TEXT NOT NULL
);

-- Both windows are scanned as (owner, time range), so one composite index per table
-- serves every quota question asked at admission time.
CREATE INDEX idx_usage_admissions_user_created ON usage_admissions(user_id, created_at);
CREATE INDEX idx_usage_events_user_created ON usage_events(user_id, created_at);

-- +goose Down
DROP INDEX idx_usage_events_user_created;
DROP INDEX idx_usage_admissions_user_created;
DROP TABLE usage_events;
DROP TABLE usage_admissions;
ALTER TABLE users DROP COLUMN plan;
