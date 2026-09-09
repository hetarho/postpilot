-- +goose Up
-- The per-post tag count (POST-63). Nullable on purpose and without a SQL default: a post
-- never saved with one reads as the application default (config.PostTagCountDefault, 4) in
-- the store mapper, so no backfill runs and "never set" stays visible to a later query.
ALTER TABLE posts ADD COLUMN tag_count INTEGER;

-- +goose Down
ALTER TABLE posts DROP COLUMN tag_count;
