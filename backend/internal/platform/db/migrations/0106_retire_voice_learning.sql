-- +goose Up
-- VOICE r5 (T465): a voice no longer learns from finalized posts. Learning events,
-- the authored sources they kept, contrast rules with their evidence and
-- confirmations, sentence feedback, rule comparisons and profile validations
-- leave the schema, and so do the three job kinds that wrote them.
--
-- A queued or running job of a retired kind can never be handled again, so it is
-- failed as interrupted; the open-hold sweep then settles its credits. Finished
-- rows stay as history.
UPDATE generation_jobs
SET status = 'failed', error = NULL, error_reason = 'JOB_INTERRUPTED',
    error_params = NULL, technical_detail = NULL,
    finished_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE kind IN ('learn_voice', 'compare_voice_rule', 'validate_voice_profile')
  AND status IN ('queued', 'running');

-- The delete guard reads two of the dropped tables, so it goes first.
DROP TRIGGER voices_refuse_publishable_work_on_delete;

-- Children before parents: every table below is referenced only by the ones
-- dropped before it.
DROP TABLE voice_rule_evidence;
DROP TABLE voice_rule_confirmations;
DROP TABLE voice_rule_comparison_candidates;
DROP TABLE voice_rule_comparisons;
DROP TABLE voice_profile_validation_items;
DROP TABLE voice_profile_validations;
DROP TABLE voice_sentence_feedback;
DROP TABLE voice_authored_sources;
DROP TABLE voice_contrast_rules;
DROP TABLE voice_learning_events;

-- One non-terminal voice-owned job per (voice_id, kind), for the kinds that remain.
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP INDEX generation_jobs_active_voice_kind_idx;

CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'seed_voice')
      AND status IN ('queued', 'running');

-- +goose StatementBegin
CREATE TRIGGER generation_jobs_refuse_duplicate_voice_work
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL
 AND NEW.kind IN ('analyze_voice', 'seed_voice')
 AND NEW.status IN ('queued', 'running')
 AND EXISTS (
     SELECT 1 FROM generation_jobs active
     WHERE active.voice_id = NEW.voice_id
       AND active.kind = NEW.kind
       AND active.status IN ('queued', 'running')
 )
BEGIN
    SELECT RAISE(ABORT, 'active voice job already exists');
END;
-- +goose StatementEnd

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
    SELECT CASE WHEN
        EXISTS (
            SELECT 1 FROM generation_jobs
            WHERE voice_id = OLD.id AND status IN ('queued', 'running')
        ) OR EXISTS (
            SELECT 1 FROM model_experiments
            WHERE voice_id = OLD.id
              AND (status IN ('queued', 'running', 'review', 'partial')
                   OR (status = 'decided' AND applied_at IS NULL))
        )
        THEN RAISE(ABORT, 'voice has publishable work') END;
END;
-- +goose StatementEnd

-- +goose Down
-- Retirement is irreversible. Lowering the recorded version must not recreate the
-- learning schema or let the retired kinds run again.
SELECT 1;
