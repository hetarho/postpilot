-- +goose Up
ALTER TABLE clip_projects ADD COLUMN language TEXT NOT NULL DEFAULT 'ko' CHECK (language IN ('ko','en'));

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN language;
