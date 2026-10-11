-- +goose Up
-- A post's photos stand in one owner order (POST-116): every photo read lists them by
-- `position`, a confirm appends after the post's last photo, and a reorder rewrites the
-- post's positions 0..n-1. Each post's existing photos keep the order they were listed in,
-- by created_at then id. A delete leaves a gap, which is harmless: only the order is read.
ALTER TABLE images ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
UPDATE images SET position = (
    SELECT COUNT(*) FROM images AS earlier
    WHERE earlier.post_slug = images.post_slug
      AND (earlier.created_at < images.created_at
           OR (earlier.created_at = images.created_at AND earlier.id < images.id))
);
CREATE INDEX idx_images_post_position ON images(post_slug, position);

-- +goose Down
DROP INDEX idx_images_post_position;
ALTER TABLE images DROP COLUMN position;
