-- +goose Up
-- GEN-31: one non-terminal voice-owned job per (voice_id, kind), and seeding a voice is
-- voice-owned work too. The trigger and its index named the four other kinds only, so two
-- concurrent seeds of one voice were refused by the Go guard alone. Both are rebuilt with
-- seed_voice beside them; nothing else about either changes.
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP INDEX generation_jobs_active_voice_kind_idx;

CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile', 'seed_voice')
      AND status IN ('queued', 'running');

-- +goose StatementBegin
CREATE TRIGGER generation_jobs_refuse_duplicate_voice_work
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL
 AND NEW.kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile', 'seed_voice')
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

-- +goose Down
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP INDEX generation_jobs_active_voice_kind_idx;

CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile')
      AND status IN ('queued', 'running');

-- +goose StatementBegin
CREATE TRIGGER generation_jobs_refuse_duplicate_voice_work
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL
 AND NEW.kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile')
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
