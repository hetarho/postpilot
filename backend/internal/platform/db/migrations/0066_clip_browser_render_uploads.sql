-- +goose Up
ALTER TABLE clip_browser_renders ADD COLUMN upload_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE clip_browser_renders ADD COLUMN stored_at TEXT;

-- +goose Down
ALTER TABLE clip_browser_renders DROP COLUMN stored_at;
ALTER TABLE clip_browser_renders DROP COLUMN upload_bytes;
