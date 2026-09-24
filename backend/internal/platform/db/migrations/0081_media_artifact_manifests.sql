-- +goose Up
ALTER TABLE clip_media_artifacts ADD COLUMN exact_bytes INTEGER NOT NULL DEFAULT 0 CHECK(exact_bytes >= 0 AND exact_bytes <= max_bytes);
ALTER TABLE clip_media_artifacts ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '' CHECK(metadata_json = '' OR json_valid(metadata_json));
ALTER TABLE clip_media_artifacts ADD COLUMN actual_bytes INTEGER CHECK(actual_bytes > 0);
ALTER TABLE clip_media_artifacts ADD COLUMN worker_digest TEXT NOT NULL DEFAULT '' CHECK(length(worker_digest) IN (0,64));
ALTER TABLE clip_media_artifacts ADD COLUMN put_expires_at TEXT;
ALTER TABLE clip_media_artifacts ADD COLUMN accepted_at TEXT;
ALTER TABLE clip_media_artifacts ADD COLUMN canonical INTEGER NOT NULL DEFAULT 0 CHECK(canonical IN (0,1));

-- An object identity outlives every cascade through project, job and attempt.
CREATE TABLE clip_media_deletions (
    object_key TEXT PRIMARY KEY,
    not_before TEXT NOT NULL,
    created_at TEXT NOT NULL
);
-- +goose StatementBegin
CREATE TRIGGER clip_media_artifact_deletion BEFORE DELETE ON clip_media_artifacts
WHEN OLD.canonical = 0
BEGIN
    INSERT INTO clip_media_deletions(object_key,not_before,created_at)
    VALUES(OLD.object_key,COALESCE(OLD.put_expires_at,OLD.created_at),strftime('%Y-%m-%dT%H:%M:%fZ','now'))
    ON CONFLICT(object_key) DO UPDATE SET not_before=MAX(not_before,excluded.not_before);
END;
-- +goose StatementEnd

-- +goose Down
-- Forward-only: removing this evidence could orphan an in-flight private PUT.
SELECT 1;
