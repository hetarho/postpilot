-- name: LinkSourceJob :execrows
UPDATE clip_source_batches SET state='consuming',job_id=? WHERE id=? AND user_id=? AND state='ready' AND expires_at>?;
-- name: BatchForJob :one
SELECT * FROM clip_source_batches WHERE user_id=? AND job_id=?;
-- name: ListConsumingBatches :many
SELECT * FROM clip_source_batches WHERE state='consuming';
-- name: AddProxy :exec
INSERT INTO clip_proxy_leases(object_key,batch_id) VALUES (?,?);
-- name: RemoveProxy :exec
DELETE FROM clip_proxy_leases WHERE object_key=?;
-- name: SaveGeneration :execrows
UPDATE clip_projects SET analysis_json=?,edit_plan_json=?,result_key=?,result_content_type=?,result_bytes=?,result_duration_ms=?,result_created_at=?,updated_at=?,edit_plan_revision=edit_plan_revision+1,rendered_plan_revision=edit_plan_revision+1 WHERE user_id=? AND id=? AND deleting=0;
-- name: EnqueueObjectDeletion :exec
INSERT INTO clip_object_deletions(object_key,created_at) VALUES (?,?) ON CONFLICT(object_key) DO NOTHING;
-- name: DeletionKeys :many
SELECT object_key FROM clip_object_deletions ORDER BY created_at,object_key;
-- name: RemoveDeletion :exec
DELETE FROM clip_object_deletions WHERE object_key=?;
-- name: ResultKeys :many
SELECT result_key FROM clip_projects WHERE result_key IS NOT NULL;
-- name: SaveCorrection :execrows
UPDATE clip_projects SET edit_plan_json=?,edit_plan_revision=edit_plan_revision+1,updated_at=? WHERE id=? AND user_id=? AND deleting=0 AND edit_plan_revision=?;
-- name: SaveRender :execrows
UPDATE clip_projects SET result_key=?,result_content_type=?,result_bytes=?,result_duration_ms=?,result_created_at=?,updated_at=?,rendered_plan_revision=edit_plan_revision WHERE id=? AND user_id=? AND deleting=0 AND edit_plan_revision=?;
-- name: HasActiveClipJob :one
SELECT COUNT(*) FROM generation_jobs WHERE clip_project_id=? AND status IN ('queued','running');
