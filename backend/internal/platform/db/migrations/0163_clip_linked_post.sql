-- +goose Up
ALTER TABLE clip_projects ADD COLUMN linked_post_slug TEXT;
ALTER TABLE clip_projects ADD COLUMN linked_post_created_at TEXT;

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN linked_post_created_at;
ALTER TABLE clip_projects DROP COLUMN linked_post_slug;
