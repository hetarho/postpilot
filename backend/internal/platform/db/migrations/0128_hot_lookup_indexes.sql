-- +goose Up
-- The lookups every clip poll and every provider call make, served by an index instead of a
-- table scan: a clip project's latest job (newest first), the FK child lookups a deleted clip
-- project or post makes into generation_jobs, and a job's admission and usage events.
--
-- The generation_jobs indexes are partial: a lookup by `col = ?` implies `col IS NOT NULL`,
-- so SQLite still uses them for the read and the FK action, while a job without a project or
-- a post costs nothing. The admission index is not UNIQUE: one hold per job is enforced by
-- the hold's re-check inside the single writer's transaction, and a UNIQUE index would abort
-- boot on any legacy duplicate.
CREATE INDEX generation_jobs_project_latest_idx ON generation_jobs(clip_project_id, created_at DESC, id DESC)
  WHERE clip_project_id IS NOT NULL;
CREATE INDEX generation_jobs_post_idx ON generation_jobs(post_slug, user_id)
  WHERE post_slug IS NOT NULL;
CREATE INDEX usage_admissions_job_idx ON usage_admissions(job_id);
CREATE INDEX usage_events_job_idx ON usage_events(job_id);

-- A browser render's sampling is not one of the project's attempts (CLIP-192): it must not
-- drop the checkpoint a failed generation's retry resumes from.
DROP TRIGGER clip_checkpoint_next_attempt;
-- +goose StatementBegin
CREATE TRIGGER clip_checkpoint_next_attempt AFTER INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL AND NEW.kind != 'sample_browser_render'
BEGIN DELETE FROM clip_attempt_checkpoints WHERE project_id=NEW.clip_project_id; END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_checkpoint_next_attempt;
-- +goose StatementBegin
CREATE TRIGGER clip_checkpoint_next_attempt AFTER INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL
BEGIN DELETE FROM clip_attempt_checkpoints WHERE project_id=NEW.clip_project_id; END;
-- +goose StatementEnd
DROP INDEX usage_events_job_idx;
DROP INDEX usage_admissions_job_idx;
DROP INDEX generation_jobs_post_idx;
DROP INDEX generation_jobs_project_latest_idx;
