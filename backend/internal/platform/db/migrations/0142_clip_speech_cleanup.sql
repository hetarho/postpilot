-- +goose Up
-- The upload journal and cascade-triggered intents outlive both account and project.
CREATE TABLE clip_speech_cleanup (
    id TEXT PRIMARY KEY, object_key TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL
);
-- +goose StatementBegin
CREATE TRIGGER clip_speech_asset_cleanup BEFORE DELETE ON clip_speech_assets
BEGIN
    INSERT INTO clip_speech_cleanup(id,object_key,created_at)
    VALUES(OLD.id,OLD.object_key,strftime('%Y-%m-%dT%H:%M:%f000000Z','now'))
    ON CONFLICT(id) DO UPDATE SET object_key=excluded.object_key,created_at=excluded.created_at;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_speech_asset_cleanup;
DROP TABLE clip_speech_cleanup;
