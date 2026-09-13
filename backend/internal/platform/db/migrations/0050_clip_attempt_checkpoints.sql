-- +goose Up
CREATE TABLE clip_attempt_checkpoints (
    project_id TEXT PRIMARY KEY REFERENCES clip_projects(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id TEXT NOT NULL REFERENCES generation_jobs(id) ON DELETE CASCADE,
    checkpoint_json TEXT NOT NULL CHECK(json_valid(checkpoint_json) AND length(CAST(checkpoint_json AS BLOB)) <= 2097152)
);
-- A new attempt invalidates old intermediate work even before its worker starts.
-- +goose StatementBegin
CREATE TRIGGER clip_checkpoint_next_attempt AFTER INSERT ON generation_jobs
WHEN NEW.clip_project_id IS NOT NULL
BEGIN DELETE FROM clip_attempt_checkpoints WHERE project_id=NEW.clip_project_id; END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_checkpoint_next_attempt;
DROP TABLE clip_attempt_checkpoints;
