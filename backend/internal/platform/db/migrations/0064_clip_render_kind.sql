-- +goose Up
-- The latest result lives on its project. A staged completion keeps the same
-- kind across a process restart before it replaces that result.
ALTER TABLE clip_projects ADD COLUMN render_kind TEXT NOT NULL DEFAULT 'server' CHECK (render_kind IN ('server','browser'));
ALTER TABLE clip_attempt_results ADD COLUMN render_kind TEXT NOT NULL DEFAULT 'server' CHECK (render_kind IN ('server','browser'));

-- +goose Down
ALTER TABLE clip_attempt_results DROP COLUMN render_kind;
ALTER TABLE clip_projects DROP COLUMN render_kind;
