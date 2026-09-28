-- +goose Up
ALTER TABLE clip_projects ADD COLUMN regions_json TEXT CHECK (regions_json IS NULL OR json_valid(regions_json));

-- +goose StatementBegin
CREATE TRIGGER clip_finalized_regions BEFORE UPDATE OF regions_json ON clip_projects
WHEN OLD.finalized_at IS NOT NULL
BEGIN SELECT RAISE(ABORT,'clip finalized'); END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_finalized_regions;
ALTER TABLE clip_projects DROP COLUMN regions_json;
