-- +goose Up
-- GUIDE-46: the owner's optional name for a guideline in the list. Empty is none; it never
-- reaches a prompt, so no applicable-texts query selects it.
ALTER TABLE guidelines ADD COLUMN title TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE guidelines DROP COLUMN title;
