-- +goose Up
ALTER TABLE clip_projects ADD COLUMN hide_disclosure INTEGER NOT NULL DEFAULT 0 CHECK (hide_disclosure IN (0, 1));

-- +goose StatementBegin
CREATE TRIGGER clip_projects_busy_disclosure_visibility BEFORE UPDATE OF hide_disclosure ON clip_projects
WHEN EXISTS (SELECT 1 FROM generation_jobs WHERE clip_project_id=OLD.id AND status IN ('queued','running'))
BEGIN SELECT RAISE(ABORT,'clip busy'); END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_projects_busy_disclosure_visibility;
ALTER TABLE clip_projects DROP COLUMN hide_disclosure;
