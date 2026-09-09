-- +goose Up
ALTER TABLE generation_jobs ADD COLUMN clip_project_id TEXT REFERENCES clip_projects(id) ON DELETE SET NULL;
ALTER TABLE generation_jobs ADD COLUMN dispatch_ready INTEGER NOT NULL DEFAULT 1 CHECK (dispatch_ready IN (0,1));
CREATE UNIQUE INDEX generation_jobs_active_clip_idx ON generation_jobs(clip_project_id) WHERE clip_project_id IS NOT NULL AND status IN ('queued','running');
DROP INDEX generation_jobs_active_user_kind_idx;
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx ON generation_jobs(user_id,kind) WHERE post_slug IS NULL AND voice_id IS NULL AND clip_project_id IS NULL AND status IN ('queued','running');
ALTER TABLE clip_source_batches ADD COLUMN job_id TEXT;
CREATE UNIQUE INDEX clip_source_job_idx ON clip_source_batches(job_id) WHERE job_id IS NOT NULL;
CREATE TABLE clip_object_deletions (object_key TEXT PRIMARY KEY, created_at TEXT NOT NULL);
CREATE TABLE clip_proxy_leases (object_key TEXT PRIMARY KEY, batch_id TEXT NOT NULL REFERENCES clip_source_batches(id) ON DELETE CASCADE);
-- +goose StatementBegin
CREATE TRIGGER generation_jobs_clip_owner BEFORE INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.clip_project_id AND user_id=NEW.user_id AND deleting=0)
BEGIN SELECT RAISE(ABORT,'clip job target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_delete BEFORE DELETE ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_edit BEFORE UPDATE OF title,target_duration_ms,deleting ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
-- Template deletion may detach its recipe without changing a frozen running payload.
-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_template BEFORE UPDATE OF video_template_id ON clip_projects
WHEN NEW.video_template_id IS NOT NULL AND EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd
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
-- +goose Down
DROP TRIGGER clip_answers_busy_update;
DROP TRIGGER clip_answers_busy_insert;
DROP TRIGGER clip_projects_busy_template;
DROP TRIGGER clip_projects_busy_edit;
DROP TRIGGER clip_projects_busy_delete;
DROP TRIGGER generation_jobs_clip_owner;
DROP TABLE clip_proxy_leases;
DROP TABLE clip_object_deletions;
DROP INDEX clip_source_job_idx;
ALTER TABLE clip_source_batches DROP COLUMN job_id;
DROP INDEX generation_jobs_active_clip_idx;
DROP INDEX generation_jobs_active_user_kind_idx;
ALTER TABLE generation_jobs DROP COLUMN dispatch_ready;
ALTER TABLE generation_jobs DROP COLUMN clip_project_id;
CREATE UNIQUE INDEX generation_jobs_active_user_kind_idx ON generation_jobs(user_id,kind) WHERE post_slug IS NULL AND voice_id IS NULL AND status IN ('queued','running');
