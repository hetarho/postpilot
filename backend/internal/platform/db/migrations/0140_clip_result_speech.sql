-- +goose Up
ALTER TABLE clip_projects ADD COLUMN result_speech_json TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_attempt_results ADD COLUMN result_speech_json TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE clip_attempt_results DROP COLUMN result_speech_json;
ALTER TABLE clip_projects DROP COLUMN result_speech_json;
