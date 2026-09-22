-- +goose Up
-- A comparison now records where it was started (MODEL-31). The origin decides the verdict
-- form its review offers (MODEL-36): an EDITOR write comparison's verdict applies its winner
-- to the post, a LAB comparison's verdict is a ranking pick that applies nothing.
--
-- Backfill: every write comparison that exists today was started from a surface whose verdict
-- applied, so it is 'editor'; observe and analyze could only ever be started in the lab.
ALTER TABLE model_experiments ADD COLUMN origin TEXT NOT NULL DEFAULT 'lab'
    CHECK (origin IN ('editor','lab'));
UPDATE model_experiments SET origin = 'editor' WHERE stage = 'write';

-- Whether this verdict owes a content application at all. Until now that was implied by
-- 'write and decided', because a write verdict always applied; a lab pick decides without
-- owing one, so an unresolved comparison has to say so explicitly.
--
-- Backfill: every decided write verdict owed an application, whether it completed, failed,
-- or was interrupted before either.
ALTER TABLE model_experiments ADD COLUMN apply_requested INTEGER NOT NULL DEFAULT 0
    CHECK (apply_requested IN (0,1));
UPDATE model_experiments SET apply_requested = 1
WHERE applied_at IS NOT NULL
   OR apply_error_reason IS NOT NULL
   OR (stage = 'write' AND status = 'decided');

-- The unresolved-per-post guard follows the same definition: a decided comparison still
-- blocks its post only while an application it asked for, or an adoption it asked for, has
-- not completed. A lab pick resolves the post the moment it is decided.
DROP INDEX one_unresolved_write_experiment_per_post;
CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
    OR (status = 'decided' AND (
          (apply_requested = 1 AND applied_at IS NULL)
          OR (adoption_requested = 1 AND adopted_at IS NULL)
       ))
  );

-- +goose Down
DROP INDEX one_unresolved_write_experiment_per_post;
CREATE UNIQUE INDEX one_unresolved_write_experiment_per_post
ON model_experiments(user_id, post_slug)
WHERE stage = 'write' AND post_slug IS NOT NULL
  AND (
    status IN ('queued', 'running', 'review', 'partial', 'failed')
    OR (status = 'decided' AND (applied_at IS NULL OR (adoption_requested = 1 AND adopted_at IS NULL)))
  );
ALTER TABLE model_experiments DROP COLUMN apply_requested;
ALTER TABLE model_experiments DROP COLUMN origin;
