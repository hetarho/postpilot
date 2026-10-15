-- name: HasProjectImportWork :one
SELECT COUNT(*) FROM generation_jobs WHERE clip_project_id=?;
-- name: MoveConfirmedImportSources :execrows
UPDATE clip_source_leases SET batch_id=sqlc.arg(destination_id)
WHERE batch_id=sqlc.arg(batch_id) AND user_id=sqlc.arg(user_id) AND state='ready' AND cleanup_pending=0;
