-- +goose Up
ALTER TABLE clip_browser_renders ADD COLUMN cancelled_at TEXT;

-- +goose Down
ALTER TABLE clip_browser_renders DROP COLUMN cancelled_at;
