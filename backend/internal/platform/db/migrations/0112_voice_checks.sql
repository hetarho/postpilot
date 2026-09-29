-- +goose Up
-- VOICE r5 (T473): 검증 writes one answered prompt in the voice (VOICE-43). Each check freezes
-- the projection it was written with, the prompt, the answer it withheld and the analysis it
-- read, and keeps its piece or its failure; results stay listed newest first (VOICE-44).
CREATE TABLE voice_checks (
    id                  TEXT PRIMARY KEY,
    user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id            TEXT NOT NULL,
    prompt_key          TEXT NOT NULL,
    material_id         TEXT NOT NULL,
    analysis_created_at TEXT NOT NULL,
    projection          TEXT NOT NULL,
    write_model         TEXT NOT NULL,
    status              TEXT NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed')),
    piece               TEXT,
    error_reason        TEXT,
    error_params        TEXT CHECK (error_params IS NULL OR json_valid(error_params)),
    technical_detail    TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);

CREATE INDEX voice_checks_by_voice_idx ON voice_checks(voice_id, created_at DESC, id DESC);

-- One queued or running job per (voice, kind), now for 검증 as well as the analysis: a voice
-- runs at most one 검증 at a time, beside at most one analysis.
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP INDEX generation_jobs_active_voice_kind_idx;

CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'check_voice')
      AND status IN ('queued', 'running');

-- +goose StatementBegin
CREATE TRIGGER generation_jobs_refuse_duplicate_voice_work
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL
 AND NEW.kind IN ('analyze_voice', 'check_voice')
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

DROP TABLE voice_checks;
