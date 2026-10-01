-- +goose Up
ALTER TABLE model_experiments ADD COLUMN review_mode TEXT NOT NULL DEFAULT 'pairwise'
    CHECK (review_mode IN ('pairwise', 'candidate_ranking'));

CREATE TABLE model_experiment_candidates_v2 (
    id TEXT PRIMARY KEY,
    experiment_id TEXT NOT NULL REFERENCES model_experiments(id) ON DELETE CASCADE,
    model_provider_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    model_label TEXT NOT NULL,
    display_side TEXT NOT NULL CHECK (display_side IN ('left', 'right', 'c', 'd', 'e')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    output TEXT,
    error TEXT,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    cost_microusd INTEGER,
    cost_source TEXT CHECK (cost_source IS NULL OR cost_source IN ('reported', 'estimated', 'unavailable')),
    latency_ms INTEGER,
    started_at TEXT,
    finished_at TEXT,
    error_reason TEXT,
    error_params TEXT CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object')),
    technical_detail TEXT,
    UNIQUE (experiment_id, display_side)
);
INSERT INTO model_experiment_candidates_v2
SELECT id, experiment_id, model_provider_id, model_id, model_label, display_side, status,
       output, error, prompt_tokens, completion_tokens, cost_microusd, cost_source,
       latency_ms, started_at, finished_at, error_reason, error_params, technical_detail
FROM model_experiment_candidates;
DROP TABLE model_experiment_candidates;
ALTER TABLE model_experiment_candidates_v2 RENAME TO model_experiment_candidates;
CREATE INDEX model_experiment_candidates_experiment
ON model_experiment_candidates(experiment_id, display_side);

-- +goose Down
-- Candidate identities may already occupy C/D/E; keep the schema when rolling back.
SELECT 1;
