-- +goose Up
ALTER TABLE clip_projects ADD COLUMN result_id TEXT CHECK(result_id IS NULL OR length(result_id)>0);
-- Existing result identifiers are stable and contain no object-storage key.
UPDATE clip_projects SET result_id='legacy-' || id WHERE result_key IS NOT NULL;
CREATE UNIQUE INDEX clip_result_identity ON clip_projects(result_id) WHERE result_id IS NOT NULL;
ALTER TABLE clip_projects ADD COLUMN finalized_at TEXT;
ALTER TABLE clip_projects ADD COLUMN finalized_plan_revision INTEGER;
ALTER TABLE clip_projects ADD COLUMN finalized_result_key TEXT
 CHECK ((finalized_at IS NULL AND finalized_plan_revision IS NULL AND finalized_result_key IS NULL)
 OR (finalized_at IS NOT NULL AND finalized_plan_revision IS NOT NULL AND finalized_plan_revision>0
 AND finalized_result_key IS NOT NULL AND result_key IS NOT NULL AND finalized_result_key=result_key AND result_id IS NOT NULL
 AND finalized_plan_revision=edit_plan_revision AND finalized_plan_revision=rendered_plan_revision
 AND source_access_revoked_at IS NOT NULL));

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
-- Template deletion can clear the reusable association; frozen content cannot change.
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_template BEFORE UPDATE OF video_template_id ON clip_projects
WHEN OLD.finalized_at IS NOT NULL AND NEW.video_template_id IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
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
CREATE TRIGGER clip_finalized_job_insert BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_finalized_job_activate BEFORE UPDATE OF dispatch_ready,status ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.status IN ('queued','running') AND EXISTS(SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND finalized_at IS NOT NULL)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd

-- +goose Down
CREATE TABLE clip_finalization_rollback_guard (allowed INTEGER CHECK(allowed=1));
INSERT INTO clip_finalization_rollback_guard SELECT 0 WHERE EXISTS(SELECT 1 FROM clip_projects WHERE finalized_at IS NOT NULL);
DROP TABLE clip_finalization_rollback_guard;
DROP TRIGGER clip_finalized_job_activate;
DROP TRIGGER clip_finalized_job_insert;
DROP TRIGGER clip_finalized_answers_update;
DROP TRIGGER clip_finalized_answers_insert;
DROP TRIGGER clip_finalized_template;
DROP TRIGGER clip_finalized_identity;
DROP TRIGGER clip_finalized_content;
ALTER TABLE clip_projects DROP COLUMN finalized_result_key;
ALTER TABLE clip_projects DROP COLUMN finalized_plan_revision;
ALTER TABLE clip_projects DROP COLUMN finalized_at;
DROP INDEX clip_result_identity;
ALTER TABLE clip_projects DROP COLUMN result_id;
