-- +goose Up
ALTER TABLE generation_jobs ADD COLUMN wait_expires_at TEXT;

-- +goose Down
ALTER TABLE generation_jobs DROP COLUMN wait_expires_at;
