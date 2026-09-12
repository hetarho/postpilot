-- name: SourceProjectWritable :one
SELECT CASE WHEN finalized_at IS NOT NULL OR deleting=1 OR source_access_revoked_at IS NOT NULL THEN 1 ELSE 0 END AS deleting FROM clip_projects WHERE id = ? AND user_id = ?;
-- name: FenceSourceProjectDeletion :execrows
UPDATE clip_projects SET deleting=1 WHERE id=? AND user_id=?;
-- name: ListProjectSourceBatches :many
SELECT * FROM clip_source_batches WHERE project_id=? AND user_id=? ORDER BY created_at,id;
-- name: InsertSourceBatch :exec
INSERT INTO clip_source_batches(id,user_id,project_id,state,created_at,expires_at,put_expires_at) VALUES (?,?,?,?,?,?,?);
-- name: InsertSourceLease :exec
INSERT INTO clip_source_leases(id,canonical_id,batch_id,user_id,object_key,filename,content_type,fingerprint,declared_bytes,duration_ms,width,height,state,ordinal) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: GetSourceBatch :one
SELECT * FROM clip_source_batches WHERE id=? AND user_id=?;
-- name: ListSourceLeases :many
SELECT * FROM clip_source_leases WHERE batch_id=? AND user_id=? ORDER BY ordinal;
-- name: MarkSourceCleanup :execrows
UPDATE clip_source_batches SET state='cleanup_pending' WHERE id=? AND user_id=?;
-- name: SetSourceLeaseReady :execrows
UPDATE clip_source_leases SET state='ready',actual_bytes=?,retention_expires_at=? WHERE canonical_id=? AND batch_id=? AND user_id=? AND state='pending' AND cleanup_pending=0;
-- name: MarkSourceBatchReady :exec
UPDATE clip_source_batches SET state='ready' WHERE clip_source_batches.id=? AND clip_source_batches.user_id=? AND clip_source_batches.state='uploading' AND NOT EXISTS (SELECT 1 FROM clip_source_leases WHERE clip_source_leases.batch_id=clip_source_batches.id AND clip_source_leases.state!='ready');
-- name: MarkExpiredSources :exec
UPDATE clip_source_batches SET state='cleanup_pending' WHERE state IN ('uploading','ready') AND expires_at<=? AND NOT EXISTS (SELECT 1 FROM clip_source_leases l WHERE l.batch_id=clip_source_batches.id AND l.cleanup_pending=0) AND NOT EXISTS(SELECT 1 FROM clip_source_attempts a WHERE a.batch_id=clip_source_batches.id AND a.released_at IS NULL);
-- name: ListSourceCleanup :many
SELECT * FROM clip_source_batches WHERE state='cleanup_pending' AND NOT EXISTS (SELECT 1 FROM clip_source_attempts a WHERE a.batch_id=clip_source_batches.id AND a.released_at IS NULL) ORDER BY created_at,id;
-- name: RemoveSourceBatch :exec
DELETE FROM clip_source_batches WHERE clip_source_batches.id=? AND clip_source_batches.user_id=? AND state='cleanup_pending' AND put_expires_at<=? AND NOT EXISTS (SELECT 1 FROM clip_source_attempts a WHERE a.batch_id=clip_source_batches.id AND a.released_at IS NULL);
-- name: SourceKeyExists :one
SELECT EXISTS(SELECT clip_source_leases.object_key FROM clip_source_leases WHERE clip_source_leases.object_key=sqlc.arg(key) UNION ALL SELECT clip_proxy_leases.object_key FROM clip_proxy_leases WHERE clip_proxy_leases.object_key=sqlc.arg(key)) AS present;

-- name: ListBatchProxies :many
SELECT object_key FROM clip_proxy_leases WHERE batch_id=?;
