-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys=OFF;
BEGIN;
-- The creation screen settles the ratio alone; the length is chosen in ①
-- beside the sources it measures (CLIP-130), so a project exists before it has
-- one. 0 is that unset state and generation is the gate that refuses it
-- (CLIP-7). SQLite cannot relax a CHECK in place, so the table is rebuilt with
-- its indexes and its own triggers; rows, ownership and every other constraint
-- are carried over unchanged.
DROP TRIGGER clip_finalized_answers_insert;
DROP TRIGGER clip_finalized_answers_update;
DROP TRIGGER clip_finalized_job_activate;
DROP TRIGGER clip_finalized_job_insert;
DROP TRIGGER clip_quote_owner;
DROP TRIGGER generation_jobs_clip_owner;
DROP TRIGGER video_templates_detach;
CREATE TABLE clip_projects_unset_duration (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    video_template_id TEXT,
    ratio TEXT NOT NULL CHECK(ratio IN ('vertical', 'horizontal', 'square')),
    target_duration_ms INTEGER NOT NULL CHECK(target_duration_ms=0 OR target_duration_ms BETWEEN 15000 AND 90000),
    analysis_json TEXT,
    edit_plan_json TEXT,
    result_key TEXT,
    result_content_type TEXT,
    result_bytes INTEGER,
    result_duration_ms INTEGER,
    result_created_at TEXT,
    edit_plan_revision INTEGER NOT NULL DEFAULT 0,
    rendered_plan_revision INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleting INTEGER NOT NULL DEFAULT 0 CHECK (deleting IN (0,1)),
    disclosure TEXT NOT NULL DEFAULT '',
    cta TEXT NOT NULL DEFAULT '',
    hide_disclosure INTEGER NOT NULL DEFAULT 0 CHECK (hide_disclosure IN (0, 1)),
    composition_inputs_json TEXT,
    composition_snapshot_json TEXT,
    source_retention_expires_at TEXT,
    source_access_revoked_at TEXT,
    source_batch_id TEXT,
    result_id TEXT CHECK(result_id IS NULL OR length(result_id)>0),
    finalized_at TEXT,
    finalized_plan_revision INTEGER,
    finalized_result_key TEXT
     CHECK ((finalized_at IS NULL AND finalized_plan_revision IS NULL AND finalized_result_key IS NULL)
     OR (finalized_at IS NOT NULL AND finalized_plan_revision IS NOT NULL AND finalized_plan_revision>0
     AND finalized_result_key IS NOT NULL AND result_key IS NOT NULL AND finalized_result_key=result_key AND result_id IS NOT NULL
     AND finalized_plan_revision=edit_plan_revision AND finalized_plan_revision=rendered_plan_revision
     AND source_access_revoked_at IS NOT NULL)),
    language TEXT NOT NULL DEFAULT 'ko' CHECK (language IN ('ko','en')),
    instruction TEXT NOT NULL DEFAULT '',
    UNIQUE(id, user_id),
    FOREIGN KEY(video_template_id, user_id) REFERENCES video_templates(id, user_id)
);
INSERT INTO clip_projects_unset_duration (id,user_id,title,video_template_id,ratio,target_duration_ms,analysis_json,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,edit_plan_revision,rendered_plan_revision,created_at,updated_at,deleting,disclosure,cta,hide_disclosure,composition_inputs_json,composition_snapshot_json,source_retention_expires_at,source_access_revoked_at,source_batch_id,result_id,finalized_at,finalized_plan_revision,finalized_result_key,language,instruction)
SELECT id,user_id,title,video_template_id,ratio,target_duration_ms,analysis_json,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,edit_plan_revision,rendered_plan_revision,created_at,updated_at,deleting,disclosure,cta,hide_disclosure,composition_inputs_json,composition_snapshot_json,source_retention_expires_at,source_access_revoked_at,source_batch_id,result_id,finalized_at,finalized_plan_revision,finalized_result_key,language,instruction FROM clip_projects;
DROP TABLE clip_projects;
ALTER TABLE clip_projects_unset_duration RENAME TO clip_projects;
CREATE INDEX clip_projects_owner_updated ON clip_projects(user_id, updated_at DESC, id);
CREATE INDEX clip_projects_template ON clip_projects(video_template_id, user_id);
CREATE UNIQUE INDEX clip_result_identity ON clip_projects(result_id) WHERE result_id IS NOT NULL;
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_content BEFORE UPDATE OF title,ratio,target_duration_ms,disclosure,cta,hide_disclosure,analysis_json,edit_plan_json,edit_plan_revision,rendered_plan_revision,result_key,result_id,result_content_type,result_bytes,result_duration_ms,result_created_at,composition_snapshot_json,composition_inputs_json ON clip_projects
WHEN OLD.finalized_at IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_identity BEFORE UPDATE OF finalized_at,finalized_plan_revision,finalized_result_key,source_access_revoked_at ON clip_projects
WHEN OLD.finalized_at IS NOT NULL AND (NEW.finalized_at IS NOT OLD.finalized_at OR NEW.finalized_plan_revision IS NOT OLD.finalized_plan_revision OR NEW.finalized_result_key IS NOT OLD.finalized_result_key OR NEW.source_access_revoked_at IS NOT OLD.source_access_revoked_at)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_template BEFORE UPDATE OF video_template_id ON clip_projects
WHEN OLD.finalized_at IS NOT NULL AND NEW.video_template_id IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
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
CREATE TRIGGER clip_recovery_finalized AFTER UPDATE OF finalized_at,deleting ON clip_projects
WHEN NEW.finalized_at IS NOT NULL OR NEW.deleting<>0
BEGIN DELETE FROM clip_recovery_states WHERE project_id=NEW.id; END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_answers_insert BEFORE INSERT ON clip_project_answers
WHEN EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_answers_update BEFORE UPDATE ON clip_project_answers
WHEN EXISTS(SELECT 1 FROM clip_projects WHERE id=OLD.project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
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
CREATE TRIGGER clip_quote_owner BEFORE INSERT ON clip_generation_quotes
WHEN NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND user_id=NEW.user_id AND deleting=0 AND source_access_revoked_at IS NULL)
  OR NOT EXISTS (SELECT 1 FROM clip_source_batches WHERE id=NEW.batch_id AND user_id=NEW.user_id AND project_id=NEW.project_id)
BEGIN SELECT RAISE(ABORT,'clip quote target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_clip_owner BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND user_id=NEW.user_id AND deleting=0)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER video_templates_detach BEFORE DELETE ON video_templates BEGIN
    UPDATE clip_projects SET video_template_id = NULL
    WHERE video_template_id = OLD.id AND user_id = OLD.user_id;
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;
BEGIN;
UPDATE clip_projects SET target_duration_ms=15000 WHERE target_duration_ms=0;
DROP TRIGGER clip_finalized_answers_insert;
DROP TRIGGER clip_finalized_answers_update;
DROP TRIGGER clip_finalized_job_activate;
DROP TRIGGER clip_finalized_job_insert;
DROP TRIGGER clip_quote_owner;
DROP TRIGGER generation_jobs_clip_owner;
DROP TRIGGER video_templates_detach;
CREATE TABLE clip_projects_bounded_duration (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    video_template_id TEXT,
    ratio TEXT NOT NULL CHECK(ratio IN ('vertical', 'horizontal', 'square')),
    target_duration_ms INTEGER NOT NULL CHECK(target_duration_ms BETWEEN 15000 AND 90000),
    analysis_json TEXT,
    edit_plan_json TEXT,
    result_key TEXT,
    result_content_type TEXT,
    result_bytes INTEGER,
    result_duration_ms INTEGER,
    result_created_at TEXT,
    edit_plan_revision INTEGER NOT NULL DEFAULT 0,
    rendered_plan_revision INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleting INTEGER NOT NULL DEFAULT 0 CHECK (deleting IN (0,1)),
    disclosure TEXT NOT NULL DEFAULT '',
    cta TEXT NOT NULL DEFAULT '',
    hide_disclosure INTEGER NOT NULL DEFAULT 0 CHECK (hide_disclosure IN (0, 1)),
    composition_inputs_json TEXT,
    composition_snapshot_json TEXT,
    source_retention_expires_at TEXT,
    source_access_revoked_at TEXT,
    source_batch_id TEXT,
    result_id TEXT CHECK(result_id IS NULL OR length(result_id)>0),
    finalized_at TEXT,
    finalized_plan_revision INTEGER,
    finalized_result_key TEXT
     CHECK ((finalized_at IS NULL AND finalized_plan_revision IS NULL AND finalized_result_key IS NULL)
     OR (finalized_at IS NOT NULL AND finalized_plan_revision IS NOT NULL AND finalized_plan_revision>0
     AND finalized_result_key IS NOT NULL AND result_key IS NOT NULL AND finalized_result_key=result_key AND result_id IS NOT NULL
     AND finalized_plan_revision=edit_plan_revision AND finalized_plan_revision=rendered_plan_revision
     AND source_access_revoked_at IS NOT NULL)),
    language TEXT NOT NULL DEFAULT 'ko' CHECK (language IN ('ko','en')),
    instruction TEXT NOT NULL DEFAULT '',
    UNIQUE(id, user_id),
    FOREIGN KEY(video_template_id, user_id) REFERENCES video_templates(id, user_id)
);
INSERT INTO clip_projects_bounded_duration (id,user_id,title,video_template_id,ratio,target_duration_ms,analysis_json,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,edit_plan_revision,rendered_plan_revision,created_at,updated_at,deleting,disclosure,cta,hide_disclosure,composition_inputs_json,composition_snapshot_json,source_retention_expires_at,source_access_revoked_at,source_batch_id,result_id,finalized_at,finalized_plan_revision,finalized_result_key,language,instruction)
SELECT id,user_id,title,video_template_id,ratio,target_duration_ms,analysis_json,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,edit_plan_revision,rendered_plan_revision,created_at,updated_at,deleting,disclosure,cta,hide_disclosure,composition_inputs_json,composition_snapshot_json,source_retention_expires_at,source_access_revoked_at,source_batch_id,result_id,finalized_at,finalized_plan_revision,finalized_result_key,language,instruction FROM clip_projects;
DROP TABLE clip_projects;
ALTER TABLE clip_projects_bounded_duration RENAME TO clip_projects;
CREATE INDEX clip_projects_owner_updated ON clip_projects(user_id, updated_at DESC, id);
CREATE INDEX clip_projects_template ON clip_projects(video_template_id, user_id);
CREATE UNIQUE INDEX clip_result_identity ON clip_projects(result_id) WHERE result_id IS NOT NULL;
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_content BEFORE UPDATE OF title,ratio,target_duration_ms,disclosure,cta,hide_disclosure,analysis_json,edit_plan_json,edit_plan_revision,rendered_plan_revision,result_key,result_id,result_content_type,result_bytes,result_duration_ms,result_created_at,composition_snapshot_json,composition_inputs_json ON clip_projects
WHEN OLD.finalized_at IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_identity BEFORE UPDATE OF finalized_at,finalized_plan_revision,finalized_result_key,source_access_revoked_at ON clip_projects
WHEN OLD.finalized_at IS NOT NULL AND (NEW.finalized_at IS NOT OLD.finalized_at OR NEW.finalized_plan_revision IS NOT OLD.finalized_plan_revision OR NEW.finalized_result_key IS NOT OLD.finalized_result_key OR NEW.source_access_revoked_at IS NOT OLD.source_access_revoked_at)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_template BEFORE UPDATE OF video_template_id ON clip_projects
WHEN OLD.finalized_at IS NOT NULL AND NEW.video_template_id IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
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
CREATE TRIGGER clip_recovery_finalized AFTER UPDATE OF finalized_at,deleting ON clip_projects
WHEN NEW.finalized_at IS NOT NULL OR NEW.deleting<>0
BEGIN DELETE FROM clip_recovery_states WHERE project_id=NEW.id; END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_answers_insert BEFORE INSERT ON clip_project_answers
WHEN EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_answers_update BEFORE UPDATE ON clip_project_answers
WHEN EXISTS(SELECT 1 FROM clip_projects WHERE id=OLD.project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
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
CREATE TRIGGER clip_quote_owner BEFORE INSERT ON clip_generation_quotes
WHEN NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND user_id=NEW.user_id AND deleting=0 AND source_access_revoked_at IS NULL)
  OR NOT EXISTS (SELECT 1 FROM clip_source_batches WHERE id=NEW.batch_id AND user_id=NEW.user_id AND project_id=NEW.project_id)
BEGIN SELECT RAISE(ABORT,'clip quote target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_clip_owner BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND user_id=NEW.user_id AND deleting=0)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER video_templates_detach BEFORE DELETE ON video_templates BEGIN
    UPDATE clip_projects SET video_template_id = NULL
    WHERE video_template_id = OLD.id AND user_id = OLD.user_id;
END;
-- +goose StatementEnd
COMMIT;
PRAGMA foreign_keys=ON;
