-- +goose NO TRANSACTION
-- +goose Up
-- A revision request is a cancellable clip job like a generation (CLIP-131,
-- CLIP-132), but the two cancellation CHECKs written for T084 named the
-- generation and the render alone, so the owner's cancel was refused by the
-- table itself. Widening a CHECK means rebuilding the table, which is why every
-- trigger that names generation_jobs is dropped and written back unchanged.
--
-- NO TRANSACTION plus an explicit PRAGMA follows 0027's precedent: both
-- clip_attempt_checkpoints.job_id and clip_recovery_states.job_id reference this
-- table with ON DELETE CASCADE, so dropping the old table with foreign keys on
-- would take every saved checkpoint and recovery state with it. `PRAGMA
-- foreign_keys` is a no-op inside a transaction, so goose must not open one for
-- us; the work still runs in one explicit transaction and `foreign_key_check`
-- proves the graph before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

DROP TRIGGER clip_answers_busy_insert;
DROP TRIGGER clip_answers_busy_update;
DROP TRIGGER clip_checkpoint_next_attempt;
DROP TRIGGER clip_finalized_job_activate;
DROP TRIGGER clip_finalized_job_insert;
DROP TRIGGER clip_projects_busy_delete;
DROP TRIGGER clip_projects_busy_disclosure_visibility;
DROP TRIGGER clip_projects_busy_edit;
DROP TRIGGER clip_projects_busy_template;
DROP TRIGGER clip_quote_job_owner;
DROP TRIGGER generation_jobs_clip_owner;
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP TRIGGER generation_jobs_require_active_voice;
DROP TRIGGER voices_refuse_publishable_work_on_delete;
CREATE TABLE generation_jobs_revisable (
    id             TEXT PRIMARY KEY,
    post_slug      TEXT,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id       TEXT,
    kind           TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed', 'cancelled')),
    stage          TEXT,
    progress_done  INTEGER NOT NULL DEFAULT 0,
    progress_total INTEGER NOT NULL DEFAULT 0,
    error          TEXT,
    observe_model  TEXT,
    write_model    TEXT,
    payload        TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    started_at     TEXT,
    finished_at    TEXT, target_language TEXT
  CHECK (target_language IS NULL OR target_language IN ('ko','en')), error_reason TEXT, error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object')), technical_detail TEXT, clip_project_id TEXT REFERENCES clip_projects(id) ON DELETE SET NULL, dispatch_ready INTEGER NOT NULL DEFAULT 1 CHECK (dispatch_ready IN (0,1)),
    cancel_requested_at TEXT,
    cancellation_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (cancellation_policy_version IN (0,1)),
    CHECK (cancel_requested_at IS NULL OR kind='render_clip' OR (kind IN ('generate_clip','revise_clip') AND cancellation_policy_version=1)),
    CHECK (status!='cancelled' OR (kind IN ('generate_clip','render_clip','revise_clip') AND cancel_requested_at IS NOT NULL)),
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO generation_jobs_revisable (id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version) SELECT id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version FROM generation_jobs;
DROP TABLE generation_jobs;
ALTER TABLE generation_jobs_revisable RENAME TO generation_jobs;
CREATE UNIQUE INDEX generation_jobs_active_clip_idx ON generation_jobs(clip_project_id) WHERE clip_project_id IS NOT NULL AND status IN ('queued','running');
CREATE UNIQUE INDEX generation_jobs_active_post_idx
    ON generation_jobs(post_slug)
    WHERE post_slug IS NOT NULL AND status IN ('queued', 'running');
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx ON generation_jobs(user_id,kind) WHERE post_slug IS NULL AND voice_id IS NULL AND clip_project_id IS NULL AND status IN ('queued','running');
CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile')
      AND status IN ('queued', 'running');
CREATE INDEX generation_jobs_queue_idx ON generation_jobs(status, created_at);
-- +goose StatementBegin
CREATE TRIGGER clip_answers_busy_insert BEFORE INSERT ON clip_project_answers
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=NEW.project_id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_answers_busy_update BEFORE UPDATE ON clip_project_answers
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.project_id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_checkpoint_next_attempt AFTER INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL
BEGIN DELETE FROM clip_attempt_checkpoints WHERE project_id=NEW.clip_project_id; END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_activate BEFORE UPDATE OF dispatch_ready,status ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.status IN ('queued','running') AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_insert BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_delete BEFORE DELETE ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_disclosure_visibility BEFORE UPDATE OF hide_disclosure ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_edit BEFORE UPDATE OF title,target_duration_ms,deleting ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_template BEFORE UPDATE OF video_template_id ON clip_projects
WHEN NEW.video_template_id IS NOT NULL AND EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_generation_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
      AND clip_project_id=NEW.project_id AND kind IN ('generate_clip','revise_clip') AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip quote job unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_clip_owner BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND user_id=NEW.user_id AND deleting=0)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
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
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_require_active_voice
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'job voice must be active');
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
            SELECT 1 FROM voice_rule_comparisons
            WHERE voice_id = OLD.id AND status IN ('queued', 'running', 'review', 'partial')
        ) OR EXISTS (
            SELECT 1 FROM voice_profile_validations
            WHERE voice_id = OLD.id AND status IN ('queued', 'running', 'review', 'partial')
        ) OR EXISTS (
            SELECT 1 FROM model_experiments
            WHERE voice_id = OLD.id
              AND (status IN ('queued', 'running', 'review', 'partial')
                   OR (status = 'decided' AND applied_at IS NULL))
        )
        THEN RAISE(ABORT, 'voice has publishable work') END;
END;
-- +goose StatementEnd

DROP TABLE IF EXISTS migration_0063_up_integrity_guard;
CREATE TABLE migration_0063_up_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0063_up_integrity_guard (problem)
SELECT 'migration left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0063_up_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;

BEGIN;

CREATE TABLE migration_0063_down_guard (problem TEXT CHECK (problem = ''));
INSERT INTO migration_0063_down_guard SELECT 'a cancelled revision cannot be represented by the narrower check'
WHERE EXISTS (SELECT 1 FROM generation_jobs WHERE kind='revise_clip' AND (cancel_requested_at IS NOT NULL OR status='cancelled'));
DROP TABLE migration_0063_down_guard;
DROP TRIGGER clip_answers_busy_insert;
DROP TRIGGER clip_answers_busy_update;
DROP TRIGGER clip_checkpoint_next_attempt;
DROP TRIGGER clip_finalized_job_activate;
DROP TRIGGER clip_finalized_job_insert;
DROP TRIGGER clip_projects_busy_delete;
DROP TRIGGER clip_projects_busy_disclosure_visibility;
DROP TRIGGER clip_projects_busy_edit;
DROP TRIGGER clip_projects_busy_template;
DROP TRIGGER clip_quote_job_owner;
DROP TRIGGER generation_jobs_clip_owner;
DROP TRIGGER generation_jobs_refuse_duplicate_voice_work;
DROP TRIGGER generation_jobs_require_active_voice;
DROP TRIGGER voices_refuse_publishable_work_on_delete;
CREATE TABLE generation_jobs_previous (
    id             TEXT PRIMARY KEY,
    post_slug      TEXT,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    voice_id       TEXT,
    kind           TEXT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('queued', 'running', 'done', 'failed', 'cancelled')),
    stage          TEXT,
    progress_done  INTEGER NOT NULL DEFAULT 0,
    progress_total INTEGER NOT NULL DEFAULT 0,
    error          TEXT,
    observe_model  TEXT,
    write_model    TEXT,
    payload        TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    started_at     TEXT,
    finished_at    TEXT, target_language TEXT
  CHECK (target_language IS NULL OR target_language IN ('ko','en')), error_reason TEXT, error_params TEXT
  CHECK (error_params IS NULL OR (json_valid(error_params) AND json_type(error_params) = 'object')), technical_detail TEXT, clip_project_id TEXT REFERENCES clip_projects(id) ON DELETE SET NULL, dispatch_ready INTEGER NOT NULL DEFAULT 1 CHECK (dispatch_ready IN (0,1)),
    cancel_requested_at TEXT,
    cancellation_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (cancellation_policy_version IN (0,1)),
    CHECK (cancel_requested_at IS NULL OR kind='render_clip' OR (kind='generate_clip' AND cancellation_policy_version=1)),
    CHECK (status!='cancelled' OR (kind IN ('generate_clip','render_clip') AND cancel_requested_at IS NOT NULL)),
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO generation_jobs_previous (id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version) SELECT id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version FROM generation_jobs;
DROP TABLE generation_jobs;
ALTER TABLE generation_jobs_previous RENAME TO generation_jobs;
CREATE UNIQUE INDEX generation_jobs_active_clip_idx ON generation_jobs(clip_project_id) WHERE clip_project_id IS NOT NULL AND status IN ('queued','running');
CREATE UNIQUE INDEX generation_jobs_active_post_idx
    ON generation_jobs(post_slug)
    WHERE post_slug IS NOT NULL AND status IN ('queued', 'running');
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx ON generation_jobs(user_id,kind) WHERE post_slug IS NULL AND voice_id IS NULL AND clip_project_id IS NULL AND status IN ('queued','running');
CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile')
      AND status IN ('queued', 'running');
CREATE INDEX generation_jobs_queue_idx ON generation_jobs(status, created_at);
-- +goose StatementBegin
CREATE TRIGGER clip_answers_busy_insert BEFORE INSERT ON clip_project_answers
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=NEW.project_id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_answers_busy_update BEFORE UPDATE ON clip_project_answers
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.project_id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_checkpoint_next_attempt AFTER INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL
BEGIN DELETE FROM clip_attempt_checkpoints WHERE project_id=NEW.clip_project_id; END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_activate BEFORE UPDATE OF dispatch_ready,status ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.status IN ('queued','running') AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_insert BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_delete BEFORE DELETE ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_disclosure_visibility BEFORE UPDATE OF hide_disclosure ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_edit BEFORE UPDATE OF title,target_duration_ms,deleting ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_template BEFORE UPDATE OF video_template_id ON clip_projects
WHEN NEW.video_template_id IS NOT NULL AND EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_generation_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
      AND clip_project_id=NEW.project_id AND kind IN ('generate_clip','revise_clip') AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip quote job unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_clip_owner BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND user_id=NEW.user_id AND deleting=0)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
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
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_require_active_voice
BEFORE INSERT ON generation_jobs
WHEN NEW.voice_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM voices v
    WHERE v.id = NEW.voice_id AND v.user_id = NEW.user_id AND v.deleted_at IS NULL
)
BEGIN
    SELECT RAISE(ABORT, 'job voice must be active');
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
            SELECT 1 FROM voice_rule_comparisons
            WHERE voice_id = OLD.id AND status IN ('queued', 'running', 'review', 'partial')
        ) OR EXISTS (
            SELECT 1 FROM voice_profile_validations
            WHERE voice_id = OLD.id AND status IN ('queued', 'running', 'review', 'partial')
        ) OR EXISTS (
            SELECT 1 FROM model_experiments
            WHERE voice_id = OLD.id
              AND (status IN ('queued', 'running', 'review', 'partial')
                   OR (status = 'decided' AND applied_at IS NULL))
        )
        THEN RAISE(ABORT, 'voice has publishable work') END;
END;
-- +goose StatementEnd

DROP TABLE IF EXISTS migration_0063_down_integrity_guard;
CREATE TABLE migration_0063_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0063_down_integrity_guard (problem)
SELECT 'rollback left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0063_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
