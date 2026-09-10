-- +goose Up
ALTER TABLE usage_admissions ADD COLUMN approved_max_credits INTEGER CHECK (approved_max_credits IS NULL OR approved_max_credits >= 0);
CREATE TABLE clip_generation_quotes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
    batch_id TEXT NOT NULL UNIQUE,
    input_digest TEXT NOT NULL,
    pricing_json TEXT NOT NULL,
    max_credits INTEGER NOT NULL CHECK (max_credits >= 0),
    expires_at TEXT NOT NULL,
    consumed_job_id TEXT UNIQUE,
    FOREIGN KEY(batch_id,user_id) REFERENCES clip_source_batches(id,user_id) ON DELETE CASCADE
);
-- +goose StatementBegin
CREATE TRIGGER clip_quote_owner BEFORE INSERT ON clip_generation_quotes
WHEN NOT EXISTS (SELECT 1 FROM clip_projects WHERE id=NEW.project_id AND user_id=NEW.user_id AND deleting=0)
  OR NOT EXISTS (SELECT 1 FROM clip_source_batches WHERE id=NEW.batch_id AND user_id=NEW.user_id AND project_id=NEW.project_id)
BEGIN SELECT RAISE(ABORT,'clip quote target unavailable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER clip_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_generation_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
      AND clip_project_id=NEW.project_id AND kind='generate_clip' AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip quote job unavailable'); END;
-- +goose StatementEnd
-- +goose Down
DROP TRIGGER clip_quote_job_owner;
DROP TRIGGER clip_quote_owner;
DROP TABLE clip_generation_quotes;
ALTER TABLE usage_admissions DROP COLUMN approved_max_credits;
