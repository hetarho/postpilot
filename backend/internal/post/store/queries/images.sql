-- Photos attached to a post. The row is created only after the object is known to
-- exist in storage.

-- name: CreateImage :exec
-- A photo joins after the post's last photo (POST-116). The position is read inside the insert
-- itself, so two confirms in one post never take the same one.
INSERT INTO images (id, post_slug, filename, r2_key, width, height, bytes, created_at, captured_at, position)
VALUES (
    sqlc.arg(id), sqlc.arg(post_slug), sqlc.arg(filename), sqlc.arg(r2_key), sqlc.arg(width), sqlc.arg(height), sqlc.arg(bytes), sqlc.arg(created_at), sqlc.narg(captured_at),
    (SELECT COALESCE(MAX(position), -1) + 1 FROM images WHERE post_slug = sqlc.arg(post_slug))
);

-- name: ListImagesByPost :many
-- The owner's order (POST-116), which every photo read and generation follow.
SELECT id, post_slug, filename, r2_key, width, height, bytes, created_at, rotation, rotation_by_owner, position, captured_at
FROM images WHERE post_slug = ? ORDER BY position, id;

-- name: GetImage :one
SELECT id, post_slug, filename, r2_key, width, height, bytes, created_at, rotation, rotation_by_owner, position, captured_at
FROM images WHERE id = ?;

-- name: SetImagePosition :execrows
-- One photo's place in the owner's order (POST-116); a reorder writes every photo of the post
-- in one transaction. A published post's photos are locked with it (POST-74), so zero rows is
-- a published post.
UPDATE images SET position = sqlc.arg(position)
WHERE id = sqlc.arg(id) AND post_slug = sqlc.arg(post_slug)
  AND EXISTS (SELECT 1 FROM posts WHERE posts.slug = images.post_slug AND posts.status <> 'published');

-- name: SetImageRotation :execrows
-- The owner turns a photo (POST-107). A published post's photos are locked with it (POST-74),
-- so zero rows is a photo already gone or a post that is published, as DeleteImage reads it.
UPDATE images SET rotation = ?, rotation_by_owner = 1
WHERE id = ? AND EXISTS (SELECT 1 FROM posts WHERE posts.slug = images.post_slug AND posts.status <> 'published');

-- name: SetObservedImageRotation :execrows
-- An observation turns its photo only while the owner never has (GEN-79).
UPDATE images SET rotation = sqlc.arg(rotation)
WHERE post_slug = sqlc.arg(post_slug) AND filename = sqlc.arg(filename) AND rotation_by_owner = 0 AND rotation <> sqlc.arg(rotation);

-- name: DeleteImage :execrows
-- A published post's photos are locked with it (POST-74): zero rows is a row already gone or
-- a post that is published, and the service re-reads the post to tell which. The default-deny
-- tests in published_lock_store_test.go pin the lock. A correlated EXISTS looks the one post
-- up by its key; an IN list would scan every post for each delete.
DELETE FROM images WHERE id = ? AND EXISTS (SELECT 1 FROM posts WHERE posts.slug = images.post_slug AND posts.status <> 'published');

-- name: ImageFilenameTaken :one
SELECT EXISTS (SELECT 1 FROM images WHERE post_slug = ? AND filename = ?);

-- name: ListAllImageKeys :many
SELECT r2_key FROM images;

-- name: ImageKeyInUse :one
SELECT EXISTS (SELECT 1 FROM images WHERE r2_key = ?);
