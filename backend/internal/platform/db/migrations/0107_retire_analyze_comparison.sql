-- +goose Up
-- VOICE r5, MODEL r20 (T466): the model lab compares observe and write only. The
-- 문체 분석 comparison goes with its experiments, candidates and badges, and the
-- analyze stage keeps its active selection alone, so its A/B pair rows go too.
-- The CHECKs that still name 'analyze' stay: the service refuses an analyze
-- comparison and an analyze pair, and a table rebuild for a CHECK is not worth it.
--
-- A queued or running job of a deleted comparison can never be handled again, so
-- it is failed as interrupted first and the open-hold sweep settles its credits.
UPDATE generation_jobs
SET status = 'failed', error = NULL, error_reason = 'JOB_INTERRUPTED',
    error_params = NULL, technical_detail = NULL,
    finished_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE kind = 'model_experiment'
  AND status IN ('queued', 'running')
  AND experiment_id IN (SELECT id FROM model_experiments WHERE stage = 'analyze');

-- Candidates and badges cascade from their experiment; they are named anyway so the
-- statement does not depend on foreign keys being on.
DELETE FROM model_experiment_badges
WHERE experiment_id IN (SELECT id FROM model_experiments WHERE stage = 'analyze');
DELETE FROM model_experiment_candidates
WHERE experiment_id IN (SELECT id FROM model_experiments WHERE stage = 'analyze');
DELETE FROM model_experiments WHERE stage = 'analyze';

DELETE FROM model_selections
WHERE stage = 'analyze' AND slot IN ('candidate_a', 'candidate_b');

-- No experiment holds a voice any more (VOICE-13): a comparison keeps the projection it
-- froze, so the delete guard waits for queued or running jobs alone.
DROP TRIGGER voices_refuse_publishable_work_on_delete;

-- +goose StatementBegin
CREATE TRIGGER voices_refuse_publishable_work_on_delete
BEFORE UPDATE OF deleted_at ON voices
WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL
BEGIN
    SELECT CASE WHEN OLD.is_default = 1
        THEN RAISE(ABORT, 'default voice cannot be deleted') END;
    SELECT CASE WHEN (
        SELECT count(*) FROM voices
        WHERE user_id = OLD.user_id AND deleted_at IS NULL
    ) <= 1 THEN RAISE(ABORT, 'last active voice cannot be deleted') END;
    SELECT CASE WHEN EXISTS (
            SELECT 1 FROM generation_jobs
            WHERE voice_id = OLD.id AND status IN ('queued', 'running')
        )
        THEN RAISE(ABORT, 'voice has publishable work') END;
END;
-- +goose StatementEnd

-- +goose Down
-- Retirement is irreversible: the deleted comparisons and pair rows cannot come back, and
-- lowering the recorded version must not let a lab start one again.
SELECT 1;
