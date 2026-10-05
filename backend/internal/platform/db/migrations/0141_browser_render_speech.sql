-- +goose Up
ALTER TABLE clip_browser_renders ADD COLUMN speech_json TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE clip_browser_renders DROP COLUMN speech_json;
