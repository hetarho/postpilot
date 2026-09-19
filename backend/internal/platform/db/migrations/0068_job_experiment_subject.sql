-- +goose Up
-- The queue addresses a job by the subject it belongs to. Every subject but one already
-- has its own column; a model experiment keeps its id in the payload, so the experiment
-- lookup scanned `payload = ?`. Derive it instead of migrating the product columns: the
-- clip/post/voice columns carry 13 triggers, two partial unique indexes and a foreign
-- key that a real subject_kind/subject_id migration would have to rebuild by hand.
--
-- VIRTUAL, not STORED: SQLite's ALTER TABLE ADD COLUMN refuses a STORED generated column,
-- and rebuilding the table is exactly what this avoids. A virtual column is still indexed.
ALTER TABLE generation_jobs ADD COLUMN experiment_id TEXT
  GENERATED ALWAYS AS (CASE WHEN kind='model_experiment' THEN payload END) VIRTUAL;
CREATE INDEX generation_jobs_experiment_idx ON generation_jobs(experiment_id, status)
  WHERE experiment_id IS NOT NULL;

-- +goose Down
DROP INDEX generation_jobs_experiment_idx;
ALTER TABLE generation_jobs DROP COLUMN experiment_id;
