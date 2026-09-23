-- Photos attached to a post. The row is created only after the object is known to
-- exist in storage.

-- name: CreateImage :exec
INSERT INTO images (id, post_slug, filename, r2_key, width, height, bytes, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListImagesByPost :many
SELECT id, post_slug, filename, r2_key, width, height, bytes, created_at
FROM images WHERE post_slug = ? ORDER BY created_at, id;

-- name: GetImage :one
SELECT id, post_slug, filename, r2_key, width, height, bytes, created_at
FROM images WHERE id = ?;

-- name: DeleteImage :execrows
-- A published post's photos are locked with it (POST-74): zero rows is a row already gone or
-- a post that is published, and the service re-reads the post to tell which.
DELETE FROM images WHERE id = ? AND post_slug IN (SELECT slug FROM posts WHERE status <> 'published');

-- name: ImageFilenameTaken :one
SELECT EXISTS (SELECT 1 FROM images WHERE post_slug = ? AND filename = ?);

-- name: ListAllImageKeys :many
SELECT r2_key FROM images;

-- name: ImageKeyInUse :one
SELECT EXISTS (SELECT 1 FROM images WHERE r2_key = ?);
