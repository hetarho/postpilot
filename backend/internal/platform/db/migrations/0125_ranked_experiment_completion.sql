-- +goose NO TRANSACTION
-- +goose Up
-- SQLite cannot extend a CHECK in place. Keep every legacy row and child FK while adding
-- the completed state; the API has not started yet, so no comparison can race the rebuild.
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE model_experiments_next (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_slug TEXT REFERENCES posts(slug) ON DELETE SET NULL,
    voice_id TEXT,
    stage TEXT NOT NULL CHECK (stage IN ('observe', 'write', 'analyze')),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'review', 'partial', 'decided', 'dismissed', 'failed', 'completed')),
    job_id TEXT,
    input_snapshot TEXT,
    input_hash TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    winner_candidate_id TEXT,
    outcome TEXT CHECK (outcome IS NULL OR outcome IN ('winner', 'skipped', 'unpaired')),
    apply_error TEXT,
    applied_at TEXT,
    created_at TEXT NOT NULL,
    finished_at TEXT,
    decided_at TEXT,
    content_expires_at TEXT,
    adoption_error TEXT,
    adopted_at TEXT,
    adoption_requested INTEGER NOT NULL DEFAULT 0 CHECK (adoption_requested IN (0, 1)),
    template_name TEXT NOT NULL DEFAULT '',
    target_language TEXT CHECK (target_language IS NULL OR target_language IN ('ko', 'en')),
    apply_error_reason TEXT,
    apply_error_params TEXT CHECK (apply_error_params IS NULL OR (json_valid(apply_error_params) AND json_type(apply_error_params) = 'object')),
    apply_technical_detail TEXT,
    adoption_error_reason TEXT,
    adoption_error_params TEXT CHECK (adoption_error_params IS NULL OR (json_valid(adoption_error_params) AND json_type(adoption_error_params) = 'object')),
    adoption_technical_detail TEXT,
    origin TEXT NOT NULL DEFAULT 'lab' CHECK (origin IN ('editor', 'lab')),
    apply_requested INTEGER NOT NULL DEFAULT 0 CHECK (apply_requested IN (0, 1)),
    source TEXT NOT NULL DEFAULT 'post' CHECK (source IN ('post', 'voice')),
    voice_prompt_key TEXT,
    voice_material_id TEXT,
    review_mode TEXT NOT NULL DEFAULT 'pairwise' CHECK (review_mode IN ('pairwise', 'candidate_ranking')),
    completed_at TEXT,
    applied_candidate_id TEXT,
    adopted_candidate_id TEXT,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);

INSERT INTO model_experiments_next (
    id,user_id,post_slug,voice_id,stage,status,job_id,input_snapshot,input_hash,prompt_version,
    winner_candidate_id,outcome,apply_error,applied_at,created_at,finished_at,decided_at,
    content_expires_at,adoption_error,adopted_at,adoption_requested,template_name,
    target_language,apply_error_reason,apply_error_params,apply_technical_detail,
    adoption_error_reason,adoption_error_params,adoption_technical_detail,origin,apply_requested,
    source,voice_prompt_key,voice_material_id,review_mode
)
SELECT id,user_id,post_slug,voice_id,stage,status,job_id,input_snapshot,input_hash,prompt_version,
       winner_candidate_id,outcome,apply_error,applied_at,created_at,finished_at,decided_at,
       content_expires_at,adoption_error,adopted_at,adoption_requested,template_name,
       target_language,apply_error_reason,apply_error_params,apply_technical_detail,
       adoption_error_reason,adoption_error_params,adoption_technical_detail,origin,apply_requested,
       source,voice_prompt_key,voice_material_id,review_mode
FROM model_experiments;

DROP TABLE model_experiments;
ALTER TABLE model_experiments_next RENAME TO model_experiments;

CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
    OR (status = 'completed' AND origin = 'editor' AND applied_at IS NULL)
    OR (status IN ('decided', 'completed') AND (
      (apply_requested = 1 AND applied_at IS NULL)
      OR (adoption_requested = 1 AND adopted_at IS NULL)
    ))
  );
CREATE INDEX model_experiments_user_stage_created
ON model_experiments(user_id, stage, created_at DESC, id DESC);
CREATE INDEX model_experiments_terminal_expiry
ON model_experiments(content_expires_at)
WHERE input_snapshot IS NOT NULL AND status IN ('decided', 'dismissed', 'completed');
CREATE INDEX model_experiments_stage_decided
ON model_experiments(stage, decided_at) WHERE decided_at IS NOT NULL;
CREATE INDEX model_experiments_stage_completed
ON model_experiments(stage, completed_at) WHERE completed_at IS NOT NULL;

-- +goose StatementBegin
CREATE TRIGGER model_experiments_require_active_voice
BEFORE INSERT ON model_experiments
WHEN NEW.voice_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'experiment voice must be active');
END;
-- +goose StatementEnd

ALTER TABLE model_experiment_candidates ADD COLUMN rank INTEGER
    CHECK (rank IS NULL OR rank > 0);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;

-- +goose Down
-- Ranked completion is durable product history. Rolling back the binary retains the schema.
SELECT 1;
