-- +goose Up
-- Keep plan-04 selections compatible while adding explicit A/B slots. SQLite cannot
-- alter a primary key, so the table is rebuilt and every existing choice becomes the
-- active slot.
CREATE TABLE model_selections_v2 (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL CHECK (stage IN ('observe', 'write', 'analyze')),
    slot        TEXT NOT NULL CHECK (slot IN ('active', 'candidate_a', 'candidate_b')),
    provider_id TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, stage, slot)
);

INSERT INTO model_selections_v2 (user_id, stage, slot, provider_id, model_id, updated_at)
SELECT user_id, stage, 'active', provider_id, model_id, updated_at
FROM model_selections;

DROP TABLE model_selections;
ALTER TABLE model_selections_v2 RENAME TO model_selections;

CREATE TABLE model_experiments (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug           TEXT REFERENCES posts(slug) ON DELETE SET NULL,
    stage               TEXT NOT NULL CHECK (stage IN ('observe', 'write', 'analyze')),
    status              TEXT NOT NULL CHECK (status IN ('queued', 'running', 'review', 'partial', 'decided', 'dismissed', 'failed')),
    job_id              TEXT,
    input_snapshot      TEXT,
    input_hash          TEXT NOT NULL,
    prompt_version      TEXT NOT NULL,
    winner_candidate_id TEXT,
    outcome             TEXT CHECK (outcome IS NULL OR outcome IN ('winner', 'skipped', 'unpaired')),
    apply_error         TEXT,
    applied_at          TEXT,
    created_at          TEXT NOT NULL,
    finished_at         TEXT,
    decided_at          TEXT,
    content_expires_at  TEXT
);

CREATE TABLE model_experiment_candidates (
    id                  TEXT PRIMARY KEY,
    experiment_id       TEXT NOT NULL REFERENCES model_experiments(id) ON DELETE CASCADE,
    model_provider_id   TEXT NOT NULL,
    model_id            TEXT NOT NULL,
    model_label         TEXT NOT NULL,
    display_side        TEXT NOT NULL CHECK (display_side IN ('left', 'right')),
    status              TEXT NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    output              TEXT,
    error               TEXT,
    prompt_tokens       INTEGER,
    completion_tokens   INTEGER,
    cost_microusd       INTEGER,
    cost_source         TEXT CHECK (cost_source IS NULL OR cost_source IN ('reported', 'estimated', 'unavailable')),
    latency_ms          INTEGER,
    started_at          TEXT,
    finished_at         TEXT,
    UNIQUE (experiment_id, display_side)
);

CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
    OR (status = 'decided' AND applied_at IS NULL)
  );

CREATE INDEX model_experiments_user_stage_created
ON model_experiments(user_id, stage, created_at DESC, id DESC);

CREATE INDEX model_experiments_terminal_expiry
ON model_experiments(content_expires_at)
WHERE input_snapshot IS NOT NULL AND status IN ('decided', 'dismissed');

CREATE INDEX model_experiment_candidates_experiment
ON model_experiment_candidates(experiment_id, display_side);

-- +goose Down
DROP INDEX model_experiment_candidates_experiment;
DROP INDEX model_experiments_terminal_expiry;
DROP INDEX model_experiments_user_stage_created;
DROP INDEX one_unresolved_write_experiment_per_post;
DROP TABLE model_experiment_candidates;
DROP TABLE model_experiments;

CREATE TABLE model_selections_v1 (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stage       TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    model_id    TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, stage)
);

INSERT INTO model_selections_v1 (user_id, stage, provider_id, model_id, updated_at)
SELECT user_id, stage, provider_id, model_id, updated_at
FROM model_selections
WHERE slot = 'active';

DROP TABLE model_selections;
ALTER TABLE model_selections_v1 RENAME TO model_selections;
