-- +goose Up
-- VOICE r5 (T468): no voice is created for an account, a voice is created by name alone as
-- a Korean voice, the 기본 is optional and any voice deletes.
--
-- The description seed leaves the product. A queued or running `seed_voice` job can never
-- be handled again, so it is failed as interrupted; the open-hold sweep then settles its
-- credits. Finished rows stay as history.
UPDATE generation_jobs
SET status = 'failed', error = NULL, error_reason = 'JOB_INTERRUPTED',
    error_params = NULL, technical_detail = NULL,
    finished_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
WHERE kind = 'seed_voice'
  AND status IN ('queued', 'running');

-- A voice carries no language (VOICE-10): its CHECK belongs to the column, so the column
-- drops on its own.
ALTER TABLE voices DROP COLUMN source_language;

-- One non-terminal voice-owned job per (voice_id, kind), for the one kind that remains.
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP INDEX generation_jobs_active_voice_kind_idx;

CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice')
      AND status IN ('queued', 'running');

-- +goose StatementBegin
CREATE TRIGGER generation_jobs_refuse_duplicate_voice_work
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL
 AND NEW.kind IN ('analyze_voice')
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

-- The 기본 and the last voice delete like any other (VOICE-13); only a queued or running job
-- frozen to the voice still holds it. `voices_one_default` keeps at most one 기본.
DROP TRIGGER voices_refuse_publishable_work_on_delete;

-- +goose StatementBegin
CREATE TRIGGER voices_refuse_publishable_work_on_delete
BEFORE UPDATE OF deleted_at ON voices
WHEN OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL
BEGIN
    SELECT CASE WHEN EXISTS (
            SELECT 1 FROM generation_jobs
            WHERE voice_id = OLD.id AND status IN ('queued', 'running')
        )
        THEN RAISE(ABORT, 'voice has publishable work') END;
END;
-- +goose StatementEnd

-- +goose Down
-- Retirement is irreversible: the failed seeds cannot run again, and lowering the recorded
-- version must not bring back a column no code writes.
SELECT 1;
