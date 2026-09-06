-- Videos attached to a post. Like an image row, it is written only after the object is
-- known to exist in storage.

-- name: CreateVideo :exec
INSERT INTO videos (id, post_slug, filename, r2_key, content_type, bytes, duration_ms, width, height, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListVideosByPost :many
SELECT id, post_slug, filename, r2_key, content_type, bytes, duration_ms, width, height, created_at
FROM videos WHERE post_slug = ? ORDER BY created_at, id;

-- name: GetVideo :one
SELECT id, post_slug, filename, r2_key, content_type, bytes, duration_ms, width, height, created_at
FROM videos WHERE id = ?;

-- name: DeleteVideo :exec
DELETE FROM videos WHERE id = ?;

-- name: VideoFilenameTaken :one
SELECT EXISTS (SELECT 1 FROM videos WHERE post_slug = ? AND filename = ?);

-- name: CountVideosByPost :one
SELECT COUNT(*) FROM videos WHERE post_slug = ?;

-- name: ListAllVideoKeys :many
SELECT r2_key FROM videos;

-- name: VideoKeyInUse :one
SELECT EXISTS (SELECT 1 FROM videos WHERE r2_key = ?);
