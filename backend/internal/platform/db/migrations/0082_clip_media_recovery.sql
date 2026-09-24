-- +goose Up
ALTER TABLE clip_media_stages ADD COLUMN retry_not_before TEXT;
ALTER TABLE clip_media_stages ADD COLUMN failure_detail TEXT CHECK(failure_detail IN ('workspace_limit','input_too_large','analysis_too_large'));
ALTER TABLE clip_media_stages ADD COLUMN reconciled_at TEXT;
CREATE INDEX clip_media_recovery ON clip_media_stages(id) WHERE reconciled_at IS NULL;

-- Canonical objects also need the upload-grant fence when their project is
-- deleted or a newer result replaces them. The outbox outlives every cascade.
DROP TRIGGER clip_media_artifact_deletion;
-- +goose StatementBegin
CREATE TRIGGER clip_media_artifact_deletion BEFORE DELETE ON clip_media_artifacts
BEGIN
    INSERT INTO clip_media_deletions(object_key,not_before,created_at)
    VALUES(OLD.object_key,COALESCE(OLD.put_expires_at,OLD.created_at),strftime('%Y-%m-%dT%H:%M:%fZ','now'))
    ON CONFLICT(object_key) DO UPDATE SET not_before=MAX(not_before,excluded.not_before);
    DELETE FROM clip_object_deletions WHERE object_key=OLD.object_key;
END;
-- +goose StatementEnd

-- +goose Down
SELECT 1;
