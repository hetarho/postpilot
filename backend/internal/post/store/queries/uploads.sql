-- Uploads presigned but not yet confirmed. A row dies on confirm or by the sweep.

-- name: CreateUpload :exec
INSERT INTO uploads (id, post_slug, filename, r2_key, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetUpload :one
SELECT id, post_slug, filename, r2_key, expires_at, created_at
FROM uploads WHERE id = ?;

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = ?;

-- name: ListUploadsExpiredBefore :many
SELECT id, post_slug, filename, r2_key, expires_at, created_at
FROM uploads WHERE expires_at < ?;

-- name: UploadFilenameTaken :one
SELECT EXISTS (SELECT 1 FROM uploads WHERE post_slug = ? AND filename = ?);

-- name: ListAllUploadKeys :many
SELECT r2_key FROM uploads;

-- name: GetUploadByFilename :one
SELECT id, post_slug, filename, r2_key, expires_at, created_at
FROM uploads WHERE post_slug = ? AND filename = ?;
