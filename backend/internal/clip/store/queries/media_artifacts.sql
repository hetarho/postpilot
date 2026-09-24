-- name: GetMediaArtifact :one
SELECT * FROM clip_media_artifacts WHERE attempt_id=? AND slot=?;

-- name: ListMediaArtifacts :many
SELECT * FROM clip_media_artifacts WHERE attempt_id=? ORDER BY slot;

-- name: InsertMediaOutput :exec
INSERT INTO clip_media_artifacts(attempt_id,slot,object_key,content_type,max_bytes,exact_bytes,metadata_json,worker_digest,put_expires_at,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(attempt_id,slot) DO NOTHING;

-- name: ExtendMediaPutExpiry :exec
UPDATE clip_media_artifacts SET put_expires_at=MAX(COALESCE(put_expires_at,created_at),sqlc.arg(expires_at)) WHERE attempt_id=sqlc.arg(attempt_id) AND slot=sqlc.arg(slot);

-- name: AcceptMediaArtifact :execrows
UPDATE clip_media_artifacts SET state='accepted',actual_bytes=exact_bytes,accepted_at=sqlc.arg(now)
WHERE attempt_id=sqlc.arg(attempt_id) AND slot=sqlc.arg(slot) AND metadata_json=sqlc.arg(metadata) AND state='reserved' AND exact_bytes>0;
