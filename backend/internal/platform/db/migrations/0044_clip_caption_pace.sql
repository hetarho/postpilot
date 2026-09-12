-- +goose Up
ALTER TABLE video_templates ADD COLUMN caption_pace TEXT NOT NULL DEFAULT '' CHECK (caption_pace IN ('', 'steady', 'rapid'));

-- +goose Down
ALTER TABLE video_templates DROP COLUMN caption_pace;
