-- name: SourceProjectAccess :one
SELECT source_batch_id,source_access_revoked_at,source_retention_expires_at,deleting,finalized_at FROM clip_projects WHERE id=? AND user_id=?;
-- name: SelectSourceBatch :execrows
UPDATE clip_projects SET source_batch_id=? WHERE id=? AND user_id=? AND deleting=0 AND finalized_at IS NULL AND source_access_revoked_at IS NULL;
-- name: FenceSourceAccess :execrows
UPDATE clip_projects SET source_access_revoked_at=COALESCE(source_access_revoked_at,?) WHERE id=? AND user_id=?;
-- name: SetSourceRetention :exec
UPDATE clip_source_leases SET retention_expires_at=sqlc.arg(expires_at)
WHERE batch_id IN (SELECT clip_source_batches.id FROM clip_source_batches WHERE clip_source_batches.project_id=sqlc.arg(project_id) AND clip_source_batches.user_id=sqlc.arg(user_id) AND clip_source_batches.state!='cleanup_pending')
AND state='ready' AND cleanup_pending=0 AND retention_expires_at>sqlc.arg(now)
AND EXISTS (SELECT 1 FROM clip_projects WHERE clip_projects.id=sqlc.arg(project_id) AND clip_projects.user_id=sqlc.arg(user_id) AND clip_projects.deleting=0 AND clip_projects.source_access_revoked_at IS NULL);
-- name: SetBoundSourceRetention :exec
UPDATE clip_source_leases SET retention_expires_at=MAX(COALESCE(retention_expires_at,''),sqlc.arg(expires_at))
WHERE batch_id=sqlc.arg(batch_id) AND state='ready' AND cleanup_pending=0
AND EXISTS (SELECT 1 FROM clip_source_batches b JOIN clip_projects p ON p.id=b.project_id AND p.user_id=b.user_id WHERE b.id=sqlc.arg(batch_id) AND b.state!='cleanup_pending' AND p.deleting=0 AND p.source_access_revoked_at IS NULL);
-- name: RefreshProjectRetention :exec
UPDATE clip_projects SET source_retention_expires_at=(SELECT MAX(l.retention_expires_at) FROM clip_source_leases l JOIN clip_source_batches b ON b.id=l.batch_id WHERE b.project_id=clip_projects.id AND b.user_id=clip_projects.user_id AND b.state!='cleanup_pending' AND l.cleanup_pending=0) WHERE clip_projects.id=? AND clip_projects.user_id=? AND clip_projects.source_access_revoked_at IS NULL;
-- name: InsertSourceAttempt :exec
INSERT INTO clip_source_attempts(job_id,user_id,project_id,batch_id,manifest_json,bound_at) VALUES (?,?,?,?,?,?);
-- name: GetSourceAttempt :one
SELECT * FROM clip_source_attempts WHERE user_id=? AND job_id=?;
-- name: ReleaseSourceAttempt :execrows
UPDATE clip_source_attempts SET released_at=? WHERE user_id=? AND job_id=? AND released_at IS NULL;
-- name: CountActiveSourceAttempts :one
SELECT COUNT(*) FROM clip_source_attempts WHERE batch_id=? AND released_at IS NULL;
-- name: UnreleasedSourceAttempts :many
SELECT * FROM clip_source_attempts WHERE released_at IS NULL ORDER BY bound_at,job_id;
-- name: MarkExpiredSourceLeases :exec
UPDATE clip_source_leases SET cleanup_pending=1
WHERE cleanup_pending=0 AND EXISTS(SELECT 1 FROM clip_source_batches b WHERE b.id=clip_source_leases.batch_id AND b.state!='consuming') AND NOT EXISTS (SELECT 1 FROM clip_source_attempts a WHERE a.batch_id=clip_source_leases.batch_id AND a.released_at IS NULL)
AND (retention_expires_at<=sqlc.arg(now) OR (state='pending' AND EXISTS(SELECT 1 FROM clip_source_batches b WHERE b.id=clip_source_leases.batch_id AND b.expires_at<=sqlc.arg(now))));
-- name: MarkBatchLeasesCleanup :exec
UPDATE clip_source_leases SET cleanup_pending=1 WHERE batch_id=?;
-- name: ListPartialSourceCleanup :many
SELECT * FROM clip_source_batches b WHERE b.state NOT IN ('cleanup_pending','consuming') AND (EXISTS(SELECT 1 FROM clip_source_leases l WHERE l.batch_id=b.id AND l.cleanup_pending=1) OR EXISTS(SELECT 1 FROM clip_proxy_leases p WHERE p.batch_id=b.id)) AND NOT EXISTS(SELECT 1 FROM clip_source_attempts a WHERE a.batch_id=b.id AND a.released_at IS NULL);
-- name: RestoreSourceBatch :exec
UPDATE clip_source_batches SET state='ready' WHERE id=? AND state='consuming' AND NOT EXISTS(SELECT 1 FROM clip_source_attempts a WHERE a.batch_id=clip_source_batches.id AND a.released_at IS NULL);
