-- +goose Up
-- Presence is meaningful: NULL asks the writer for a natural length, while every
-- existing positive value remains an explicit per-post preference.
ALTER TABLE posts RENAME COLUMN target_length TO target_length_legacy;
ALTER TABLE posts ADD COLUMN target_length INTEGER CHECK (target_length IS NULL OR target_length > 0);
UPDATE posts SET target_length = target_length_legacy;
ALTER TABLE posts DROP COLUMN target_length_legacy;

ALTER TABLE posts ADD COLUMN finalized_revision INTEGER;
ALTER TABLE posts ADD COLUMN finalized_at TEXT;

-- Applying a candidate and adopting its model cross aggregate boundaries. These
-- markers make the second step retryable without repeating the verdict or post write.
ALTER TABLE model_experiments ADD COLUMN adoption_error TEXT;
ALTER TABLE model_experiments ADD COLUMN adopted_at TEXT;
ALTER TABLE model_experiments ADD COLUMN adoption_requested INTEGER NOT NULL DEFAULT 0
  CHECK (adoption_requested IN (0, 1));

-- A failed requested adoption is still unresolved work for this post. Keep the
-- existing one-pending-write guard authoritative for direct/concurrent RPCs too.
DROP INDEX one_unresolved_write_experiment_per_post;
CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
	OR (status = 'decided' AND (applied_at IS NULL OR (adoption_requested = 1 AND adopted_at IS NULL)))
  );

-- +goose Down
DROP INDEX one_unresolved_write_experiment_per_post;
CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
    OR (status = 'decided' AND applied_at IS NULL)
  );

ALTER TABLE model_experiments DROP COLUMN adopted_at;
ALTER TABLE model_experiments DROP COLUMN adoption_error;
ALTER TABLE model_experiments DROP COLUMN adoption_requested;

ALTER TABLE posts DROP COLUMN finalized_at;
ALTER TABLE posts DROP COLUMN finalized_revision;

ALTER TABLE posts RENAME COLUMN target_length TO target_length_optional;
ALTER TABLE posts ADD COLUMN target_length INTEGER NOT NULL DEFAULT 1200;
UPDATE posts SET target_length = COALESCE(target_length_optional, 1200);
ALTER TABLE posts DROP COLUMN target_length_optional;
