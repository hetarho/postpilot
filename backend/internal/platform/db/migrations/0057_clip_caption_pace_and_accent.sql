-- +goose Up
-- The caption pace and the accent colour belong to the PROJECT (CLIP-139): they
-- are chosen in ① beside the sources, not carried by the reusable template.
-- Empty is "not chosen": the project falls back to what its frozen document
-- said, so every existing project renders exactly as it did.
ALTER TABLE clip_projects ADD COLUMN caption_pace TEXT NOT NULL DEFAULT '';
ALTER TABLE clip_projects ADD COLUMN accent TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE clip_projects DROP COLUMN caption_pace;
ALTER TABLE clip_projects DROP COLUMN accent;
