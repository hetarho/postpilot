-- +goose Up
ALTER TABLE posts ADD COLUMN content_origins TEXT;
ALTER TABLE posts ADD COLUMN storyline_origins TEXT;

-- +goose Down
ALTER TABLE posts DROP COLUMN storyline_origins;
ALTER TABLE posts DROP COLUMN content_origins;
