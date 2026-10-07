-- name: DeleteOtherPostStageCaptures :exec
DELETE FROM post_request_captures WHERE post_slug = ? AND stage = ? AND job_id <> ?;

-- name: DeletePostStageCaptures :exec
DELETE FROM post_request_captures WHERE post_slug = ? AND user_id = ? AND stage = ?;

-- name: WritePostRequestCapture :exec
INSERT INTO post_request_captures (
 post_slug, user_id, job_id, call_id, call_sequence, attachment_id, stage,
 input_revision, content_revision, source_fingerprint, source_plan_fingerprint, payload
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(post_slug, job_id, call_id) DO NOTHING;

-- name: ListPostRequestCaptures :many
SELECT * FROM post_request_captures WHERE post_slug = ? AND user_id = ? AND stage = ? ORDER BY call_sequence, call_id;

-- name: BindPostRequestCapture :execrows
UPDATE post_request_captures SET result_revision = sqlc.narg(result_revision),
 result_hash = sqlc.narg(result_hash), plan_fingerprint = sqlc.narg(plan_fingerprint)
WHERE post_slug = sqlc.arg(post_slug) AND user_id = sqlc.arg(user_id) AND job_id = sqlc.arg(job_id)
 AND content_revision = sqlc.arg(content_revision) AND source_fingerprint = sqlc.arg(source_fingerprint);

-- name: PurgePostRequestCaptures :exec
DELETE FROM post_request_captures WHERE post_slug = ? AND user_id = ?;

-- name: FencePostRequestCapturePurge :exec
INSERT INTO post_request_capture_purges(post_slug, user_id, job_id)
SELECT c.post_slug, c.user_id, c.job_id FROM post_request_captures c WHERE c.post_slug = ? AND c.user_id = ?
ON CONFLICT(post_slug, job_id) DO NOTHING;

-- name: FencePostRequestCaptureRunPurge :exec
INSERT INTO post_request_capture_purges(post_slug, user_id, job_id) VALUES(?, ?, ?)
ON CONFLICT(post_slug, job_id) DO NOTHING;

-- name: PostRequestCapturePurged :one
SELECT EXISTS(SELECT 1 FROM post_request_capture_purges WHERE post_slug = ? AND user_id = ? AND job_id = ?);

-- name: PurgeWithdrawnPostRequestCaptures :exec
DELETE FROM post_request_captures WHERE post_slug = ?;
