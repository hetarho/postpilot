-- +goose NO TRANSACTION
-- +goose Up
-- A browser render's grounds are sampled on a media worker (CLIP-192): a
-- `sample_browser_render` job owns one `sample` media stage, and the render row
-- keeps what it measured. The job is cancellable like a render, so both
-- cancellation CHECKs on generation_jobs name it, and the stage CHECK admits the
-- new operation. Widening a CHECK means rebuilding the table, so every trigger
-- that names generation_jobs is dropped and written back unchanged.
--
-- NO TRANSACTION plus an explicit PRAGMA follows 0063: checkpoints, recovery
-- states, continuations and media stages reference generation_jobs, and media
-- attempts reference clip_media_stages, all ON DELETE CASCADE, so dropping an
-- old table with foreign keys on would take their rows with it. The work still
-- runs in one explicit transaction and `foreign_key_check` proves the graph
-- before it commits.

PRAGMA foreign_keys=OFF;

BEGIN;

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
CREATE TABLE generation_jobs_sampling (
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
    cancellation_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (cancellation_policy_version IN (0,1)), experiment_id TEXT
  GENERATED ALWAYS AS (CASE WHEN kind='model_experiment' THEN payload END) VIRTUAL,
    CHECK (cancel_requested_at IS NULL OR kind IN ('render_clip','sample_browser_render') OR (kind IN ('generate_clip','revise_clip','storyline_clip','revise_storyline_clip') AND cancellation_policy_version=1)),
    CHECK (status!='cancelled' OR (kind IN ('generate_clip','render_clip','sample_browser_render','revise_clip','storyline_clip','revise_storyline_clip') AND cancel_requested_at IS NOT NULL)),
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO generation_jobs_sampling (id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version) SELECT id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version FROM generation_jobs;
DROP TABLE generation_jobs;
ALTER TABLE generation_jobs_sampling RENAME TO generation_jobs;
CREATE UNIQUE INDEX generation_jobs_active_clip_idx ON generation_jobs(clip_project_id) WHERE clip_project_id IS NOT NULL AND status IN ('queued','running');
CREATE UNIQUE INDEX generation_jobs_active_post_idx
    ON generation_jobs(post_slug)
    WHERE post_slug IS NOT NULL AND status IN ('queued', 'running');
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx ON generation_jobs(user_id,kind) WHERE post_slug IS NULL AND voice_id IS NULL AND clip_project_id IS NULL AND status IN ('queued','running');
CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile', 'seed_voice')
      AND status IN ('queued', 'running');
CREATE INDEX generation_jobs_experiment_idx ON generation_jobs(experiment_id, status)
  WHERE experiment_id IS NOT NULL;
CREATE INDEX generation_jobs_queue_idx ON generation_jobs(status, created_at);
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
      AND clip_project_id=NEW.project_id AND kind IN ('generate_clip','revise_clip','storyline_clip','revise_storyline_clip') AND status='queued' AND dispatch_ready=0
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
CREATE TABLE clip_media_stages_sampling (
    id TEXT PRIMARY KEY,
    parent_job_id TEXT NOT NULL REFERENCES generation_jobs(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
    expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
    stage_key TEXT NOT NULL CHECK(stage_key IN ('prepare','render','sample')),
    operation TEXT NOT NULL CHECK(operation = stage_key),
    contract_version INTEGER NOT NULL CHECK(contract_version > 0),
    input_digest TEXT NOT NULL,
    input_payload TEXT NOT NULL CHECK(json_valid(input_payload)),
    renderer_version TEXT NOT NULL,
    asset_version TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','succeeded','failed','cancelled')),
    current_attempt_id TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    created_at TEXT NOT NULL,
    queue_deadline_at TEXT NOT NULL,
    deadline_at TEXT NOT NULL,
    lease_ttl_ns INTEGER NOT NULL CHECK(lease_ttl_ns > 0),
    attempt_limit INTEGER NOT NULL CHECK(attempt_limit BETWEEN 1 AND 5),
    accepted_result TEXT,
    failure TEXT CHECK(failure IN ('worker_lost','wait_expired','deadline_exceeded','attempts_exhausted','invalid_input','invalid_output','internal')), retry_not_before TEXT, failure_detail TEXT CHECK(failure_detail IN ('workspace_limit','input_too_large','analysis_too_large')), reconciled_at TEXT,
    UNIQUE(parent_job_id, stage_key),
    CHECK(created_at < queue_deadline_at AND queue_deadline_at <= deadline_at),
    CHECK(attempt_count <= attempt_limit),
    CHECK((state = 'succeeded') = (accepted_result IS NOT NULL))
);
INSERT INTO clip_media_stages_sampling (id,parent_job_id,user_id,project_id,expected_revision,stage_key,operation,contract_version,input_digest,input_payload,renderer_version,asset_version,state,current_attempt_id,attempt_count,created_at,queue_deadline_at,deadline_at,lease_ttl_ns,attempt_limit,accepted_result,failure,retry_not_before,failure_detail,reconciled_at) SELECT id,parent_job_id,user_id,project_id,expected_revision,stage_key,operation,contract_version,input_digest,input_payload,renderer_version,asset_version,state,current_attempt_id,attempt_count,created_at,queue_deadline_at,deadline_at,lease_ttl_ns,attempt_limit,accepted_result,failure,retry_not_before,failure_detail,reconciled_at FROM clip_media_stages;
DROP TABLE clip_media_stages;
ALTER TABLE clip_media_stages_sampling RENAME TO clip_media_stages;
CREATE INDEX clip_media_claim ON clip_media_stages(state, operation, contract_version, created_at);
CREATE INDEX clip_media_recovery ON clip_media_stages(id) WHERE reconciled_at IS NULL;

ALTER TABLE clip_browser_renders ADD COLUMN sample_job_id TEXT;
ALTER TABLE clip_browser_renders ADD COLUMN grounds_json TEXT CHECK (grounds_json IS NULL OR json_valid(grounds_json));
ALTER TABLE clip_browser_renders ADD COLUMN sampled_at TEXT;

DROP TABLE IF EXISTS migration_0105_up_integrity_guard;
CREATE TABLE migration_0105_up_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0105_up_integrity_guard (problem)
SELECT 'migration left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0105_up_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;

-- +goose Down
-- The sampling jobs go first, with foreign keys on, so their stages,
-- attempts and artifacts cascade away before the narrower CHECKs return.
DELETE FROM generation_jobs WHERE kind='sample_browser_render';

PRAGMA foreign_keys=OFF;

BEGIN;

ALTER TABLE clip_browser_renders DROP COLUMN sampled_at;
ALTER TABLE clip_browser_renders DROP COLUMN grounds_json;
ALTER TABLE clip_browser_renders DROP COLUMN sample_job_id;

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
CREATE TABLE generation_jobs_unsampled (
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
    cancellation_policy_version INTEGER NOT NULL DEFAULT 0 CHECK (cancellation_policy_version IN (0,1)), experiment_id TEXT
  GENERATED ALWAYS AS (CASE WHEN kind='model_experiment' THEN payload END) VIRTUAL,
    CHECK (cancel_requested_at IS NULL OR kind='render_clip' OR (kind IN ('generate_clip','revise_clip','storyline_clip','revise_storyline_clip') AND cancellation_policy_version=1)),
    CHECK (status!='cancelled' OR (kind IN ('generate_clip','render_clip','revise_clip','storyline_clip','revise_storyline_clip') AND cancel_requested_at IS NOT NULL)),
    FOREIGN KEY (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE,
    FOREIGN KEY (voice_id, user_id) REFERENCES voices(id, user_id)
);
INSERT INTO generation_jobs_unsampled (id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version) SELECT id,post_slug,user_id,voice_id,kind,status,stage,progress_done,progress_total,error,observe_model,write_model,payload,created_at,updated_at,started_at,finished_at,target_language,error_reason,error_params,technical_detail,clip_project_id,dispatch_ready,cancel_requested_at,cancellation_policy_version FROM generation_jobs;
DROP TABLE generation_jobs;
ALTER TABLE generation_jobs_unsampled RENAME TO generation_jobs;
CREATE UNIQUE INDEX generation_jobs_active_clip_idx ON generation_jobs(clip_project_id) WHERE clip_project_id IS NOT NULL AND status IN ('queued','running');
CREATE UNIQUE INDEX generation_jobs_active_post_idx
    ON generation_jobs(post_slug)
    WHERE post_slug IS NOT NULL AND status IN ('queued', 'running');
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx ON generation_jobs(user_id,kind) WHERE post_slug IS NULL AND voice_id IS NULL AND clip_project_id IS NULL AND status IN ('queued','running');
CREATE INDEX generation_jobs_active_voice_kind_idx
    ON generation_jobs(voice_id, kind)
    WHERE voice_id IS NOT NULL
      AND kind IN ('analyze_voice', 'learn_voice', 'compare_voice_rule', 'validate_voice_profile', 'seed_voice')
      AND status IN ('queued', 'running');
CREATE INDEX generation_jobs_experiment_idx ON generation_jobs(experiment_id, status)
  WHERE experiment_id IS NOT NULL;
CREATE INDEX generation_jobs_queue_idx ON generation_jobs(status, created_at);
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
      AND clip_project_id=NEW.project_id AND kind IN ('generate_clip','revise_clip','storyline_clip','revise_storyline_clip') AND status='queued' AND dispatch_ready=0
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
CREATE TABLE clip_media_stages_unsampled (
    id TEXT PRIMARY KEY,
    parent_job_id TEXT NOT NULL REFERENCES generation_jobs(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
    expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
    stage_key TEXT NOT NULL CHECK(stage_key IN ('prepare','render')),
    operation TEXT NOT NULL CHECK(operation = stage_key),
    contract_version INTEGER NOT NULL CHECK(contract_version > 0),
    input_digest TEXT NOT NULL,
    input_payload TEXT NOT NULL CHECK(json_valid(input_payload)),
    renderer_version TEXT NOT NULL,
    asset_version TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','running','succeeded','failed','cancelled')),
    current_attempt_id TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    created_at TEXT NOT NULL,
    queue_deadline_at TEXT NOT NULL,
    deadline_at TEXT NOT NULL,
    lease_ttl_ns INTEGER NOT NULL CHECK(lease_ttl_ns > 0),
    attempt_limit INTEGER NOT NULL CHECK(attempt_limit BETWEEN 1 AND 5),
    accepted_result TEXT,
    failure TEXT CHECK(failure IN ('worker_lost','wait_expired','deadline_exceeded','attempts_exhausted','invalid_input','invalid_output','internal')), retry_not_before TEXT, failure_detail TEXT CHECK(failure_detail IN ('workspace_limit','input_too_large','analysis_too_large')), reconciled_at TEXT,
    UNIQUE(parent_job_id, stage_key),
    CHECK(created_at < queue_deadline_at AND queue_deadline_at <= deadline_at),
    CHECK(attempt_count <= attempt_limit),
    CHECK((state = 'succeeded') = (accepted_result IS NOT NULL))
);
INSERT INTO clip_media_stages_unsampled (id,parent_job_id,user_id,project_id,expected_revision,stage_key,operation,contract_version,input_digest,input_payload,renderer_version,asset_version,state,current_attempt_id,attempt_count,created_at,queue_deadline_at,deadline_at,lease_ttl_ns,attempt_limit,accepted_result,failure,retry_not_before,failure_detail,reconciled_at) SELECT id,parent_job_id,user_id,project_id,expected_revision,stage_key,operation,contract_version,input_digest,input_payload,renderer_version,asset_version,state,current_attempt_id,attempt_count,created_at,queue_deadline_at,deadline_at,lease_ttl_ns,attempt_limit,accepted_result,failure,retry_not_before,failure_detail,reconciled_at FROM clip_media_stages;
DROP TABLE clip_media_stages;
ALTER TABLE clip_media_stages_unsampled RENAME TO clip_media_stages;
CREATE INDEX clip_media_claim ON clip_media_stages(state, operation, contract_version, created_at);
CREATE INDEX clip_media_recovery ON clip_media_stages(id) WHERE reconciled_at IS NULL;

DROP TABLE IF EXISTS migration_0105_down_integrity_guard;
CREATE TABLE migration_0105_down_integrity_guard (problem TEXT NOT NULL CHECK (problem = ''));
INSERT INTO migration_0105_down_integrity_guard (problem)
SELECT 'migration left a foreign-key violation'
WHERE EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE migration_0105_down_integrity_guard;

COMMIT;

PRAGMA foreign_keys=ON;
