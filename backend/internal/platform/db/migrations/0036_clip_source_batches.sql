-- +goose Up
-- Project deletion intent fences admission; it is not an input attachment.
ALTER TABLE clip_projects ADD COLUMN deleting INTEGER NOT NULL DEFAULT 0 CHECK (deleting IN (0,1));
-- Deliberately no project FK: cleanup identities must survive deleting their project.
CREATE TABLE clip_source_batches (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('uploading','ready','consuming','cleanup_pending')),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    UNIQUE(id,user_id)
);
CREATE UNIQUE INDEX clip_source_active_project ON clip_source_batches(user_id,project_id) WHERE state != 'cleanup_pending';
CREATE INDEX clip_source_expiry ON clip_source_batches(state,expires_at);
CREATE TABLE clip_source_leases (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    declared_bytes INTEGER NOT NULL CHECK (declared_bytes > 0),
    actual_bytes INTEGER NOT NULL DEFAULT 0 CHECK (actual_bytes >= 0),
    duration_ms INTEGER NOT NULL CHECK (duration_ms > 0),
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    state TEXT NOT NULL CHECK (state IN ('pending','ready')),
    ordinal INTEGER NOT NULL,
    UNIQUE(batch_id,fingerprint),
    UNIQUE(batch_id,ordinal),
    FOREIGN KEY(batch_id,user_id) REFERENCES clip_source_batches(id,user_id) ON DELETE CASCADE
);
-- +goose Down
DROP TABLE clip_source_leases;
DROP TABLE clip_source_batches;
ALTER TABLE clip_projects DROP COLUMN deleting;
