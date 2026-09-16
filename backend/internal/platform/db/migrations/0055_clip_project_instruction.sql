-- +goose Up
-- The instruction belongs to the project (CLIP-1), not to the answers the
-- template declared, so it is a column rather than a member of
-- composition_inputs_json. Every existing project reads as having none.
ALTER TABLE clip_projects ADD COLUMN instruction TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN instruction;
