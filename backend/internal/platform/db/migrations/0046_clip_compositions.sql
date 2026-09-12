-- +goose Up
ALTER TABLE video_templates ADD COLUMN composition_body TEXT;
ALTER TABLE video_templates ADD COLUMN composition_legacy INTEGER NOT NULL DEFAULT 0 CHECK (composition_legacy IN (0,1));
ALTER TABLE clip_projects ADD COLUMN composition_inputs_json TEXT;
ALTER TABLE clip_projects ADD COLUMN composition_snapshot_json TEXT;

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN composition_snapshot_json;
ALTER TABLE clip_projects DROP COLUMN composition_inputs_json;
ALTER TABLE video_templates DROP COLUMN composition_legacy;
ALTER TABLE video_templates DROP COLUMN composition_body;
