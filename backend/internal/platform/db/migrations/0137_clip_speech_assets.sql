-- +goose Up
CREATE TABLE clip_speech_assets (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
  object_key TEXT NOT NULL UNIQUE,
  input_text TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  binding_digest TEXT NOT NULL,
  speech_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX clip_speech_assets_reuse_idx ON clip_speech_assets(owner_id, project_id, input_hash, binding_digest);

CREATE TABLE clip_speech_segments (
  id TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT NOT NULL REFERENCES clip_projects(id) ON DELETE CASCADE,
  job_id TEXT NOT NULL,
  plan_revision INTEGER NOT NULL CHECK (plan_revision > 0),
  segment_id TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  binding_digest TEXT NOT NULL,
  asset_id TEXT REFERENCES clip_speech_assets(id) ON DELETE SET NULL,
  state TEXT NOT NULL CHECK (state IN ('reserved','claimed','received','published','failed','cancelled','unresolved','obsolete')),
  operation_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (job_id, segment_id)
);
CREATE INDEX clip_speech_segments_owner_idx ON clip_speech_segments(owner_id, project_id, updated_at);

-- +goose Down
DROP TABLE clip_speech_segments;
DROP TABLE clip_speech_assets;
