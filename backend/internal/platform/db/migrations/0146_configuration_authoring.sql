-- +goose Up
CREATE TABLE configuration_authoring_sessions (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('post_template','video_template','post_guideline','video_guideline','writing_voice')),
 target_id TEXT NOT NULL DEFAULT '',
 request_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision >= 0),
 phase TEXT NOT NULL,
 snapshot TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 UNIQUE(user_id,request_id),
 UNIQUE(user_id,id)
);
CREATE INDEX configuration_authoring_latest ON configuration_authoring_sessions(user_id,kind,target_id,updated_at DESC,id DESC);
CREATE TABLE configuration_authoring_operations (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 session_id TEXT NOT NULL,
 request_id TEXT NOT NULL,
 fingerprint TEXT NOT NULL,
 base_revision INTEGER NOT NULL,
 mode TEXT NOT NULL CHECK(mode IN ('recommend','refine')),
 payload BLOB NOT NULL,
 job_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('pending','admitted','done','failed','cancelled')),
 failure_reason TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 UNIQUE(user_id,session_id,request_id),
 FOREIGN KEY(user_id,session_id) REFERENCES configuration_authoring_sessions(user_id,id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX configuration_authoring_active_account ON configuration_authoring_operations(user_id) WHERE status IN ('pending','admitted');
CREATE UNIQUE INDEX configuration_authoring_job_receipt ON configuration_authoring_operations(job_id) WHERE job_id <> '';
-- +goose Down
DROP TABLE configuration_authoring_operations;
DROP TABLE configuration_authoring_sessions;
