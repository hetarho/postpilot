-- +goose Up
-- CLIP-166, CLIP-168: a video template carries a starting design selection — the intro preset,
-- the outro preset and the caption styles — with the types, defaults and value sets
-- clip_projects uses (0062). A project created with the template, or switched to it, takes it;
-- nothing else reads it. Every existing template starts at the shared defaults.
ALTER TABLE video_templates ADD COLUMN intro_preset TEXT NOT NULL DEFAULT '';
ALTER TABLE video_templates ADD COLUMN outro_preset TEXT NOT NULL DEFAULT '';
ALTER TABLE video_templates ADD COLUMN allowed_caption_styles TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE video_templates DROP COLUMN allowed_caption_styles;
ALTER TABLE video_templates DROP COLUMN outro_preset;
ALTER TABLE video_templates DROP COLUMN intro_preset;
