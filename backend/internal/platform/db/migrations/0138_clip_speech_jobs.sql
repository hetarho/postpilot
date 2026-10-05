-- +goose Up
ALTER TABLE clip_speech_assets ADD COLUMN bytes_count INTEGER NOT NULL DEFAULT 0 CHECK(bytes_count >= 0 AND bytes_count <= 8388608);
CREATE TABLE clip_speech_jobs (
 id TEXT PRIMARY KEY, owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
 request_key TEXT NOT NULL, request_digest TEXT NOT NULL, job_id TEXT NOT NULL DEFAULT '',
 operation_json TEXT NOT NULL, created_at TEXT NOT NULL,
 UNIQUE(owner_id, project_id, request_key)
);
-- +goose Down
DROP TABLE clip_speech_jobs;
ALTER TABLE clip_speech_assets DROP COLUMN bytes_count;
