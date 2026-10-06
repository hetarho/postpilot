-- +goose Up
-- Cleanup identities deliberately survive project/quote deletion. Access is
-- authorized through the current project/source/parent, never these references.
CREATE TABLE clip_analysis_preparations (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL,
 project_id TEXT NOT NULL,
 batch_id TEXT NOT NULL,
 quote_id TEXT NOT NULL UNIQUE,
 profile_version TEXT NOT NULL,
 expected_revision INTEGER NOT NULL CHECK(expected_revision>=0),
 metadata_json TEXT NOT NULL,
 manifest_digest TEXT NOT NULL,
 state TEXT NOT NULL DEFAULT 'preparing' CHECK(state IN ('preparing','verifying','accepted','consumed','failed','cancelled','expired')),
 parent_job_id TEXT,
 current_attempt_id TEXT,
 attempt_count INTEGER NOT NULL DEFAULT 0,
 progress INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 expires_at TEXT NOT NULL,
 queue_deadline_at TEXT NOT NULL,
 deadline_at TEXT NOT NULL,
 accepted_result TEXT,
 failure TEXT,
 cleanup_at TEXT,
 reconciled_at TEXT
);
CREATE INDEX clip_analysis_preparation_claim ON clip_analysis_preparations(state,created_at,id);
CREATE TABLE clip_analysis_copies (
 preparation_id TEXT NOT NULL REFERENCES clip_analysis_preparations(id) ON DELETE CASCADE,
 slot TEXT NOT NULL,
 source_id TEXT NOT NULL,
 source_fingerprint TEXT NOT NULL,
 ordinal INTEGER NOT NULL CHECK(ordinal>=0),
 offset_ms INTEGER NOT NULL CHECK(offset_ms>=0),
 duration_ms INTEGER NOT NULL CHECK(duration_ms>0 AND duration_ms<=60000),
 width INTEGER NOT NULL CHECK(width>0 AND width<=720),
 height INTEGER NOT NULL CHECK(height>0 AND height<=720),
 has_audio INTEGER NOT NULL CHECK(has_audio IN (0,1)),
 object_key TEXT UNIQUE,
 exact_bytes INTEGER NOT NULL DEFAULT 0 CHECK(exact_bytes>=0 AND exact_bytes<=8388608),
 sha256 TEXT NOT NULL DEFAULT '',
 state TEXT NOT NULL DEFAULT 'expected' CHECK(state IN ('expected','reserved','verified','cleanup','deleted')),
 put_expires_at TEXT,
 verification_json TEXT,
 PRIMARY KEY(preparation_id,slot)
);
CREATE TABLE clip_analysis_verification_attempts (
 id TEXT PRIMARY KEY,
 preparation_id TEXT NOT NULL REFERENCES clip_analysis_preparations(id) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL,
 worker_id TEXT NOT NULL,
 token_hash TEXT NOT NULL,
 lease_expires_at TEXT NOT NULL,
 started_at TEXT NOT NULL,
 outcome TEXT,
 finished_at TEXT,
 runtime_manifest TEXT NOT NULL,
 UNIQUE(preparation_id,ordinal)
);
CREATE INDEX clip_analysis_verification_worker ON clip_analysis_verification_attempts(worker_id,lease_expires_at);
CREATE TABLE clip_analysis_copy_deletions (
 object_key TEXT PRIMARY KEY,
 delete_after TEXT NOT NULL
);

-- +goose Down
DROP TABLE clip_analysis_verification_attempts;
DROP TABLE clip_analysis_copies;
DROP TABLE clip_analysis_preparations;
DROP TABLE clip_analysis_copy_deletions;
