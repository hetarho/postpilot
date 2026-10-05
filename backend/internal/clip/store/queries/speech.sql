-- name: InsertClipSpeechAsset :execrows
INSERT INTO clip_speech_assets(id,owner_id,project_id,object_key,input_text,input_hash,binding_digest,speech_json,created_at,bytes_count)
SELECT ?,?,?,?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM clip_projects WHERE clip_projects.id = sqlc.arg(project_check) AND clip_projects.user_id = sqlc.arg(owner_check) AND deleting=0 AND finalized_at IS NULL);

-- name: GetClipSpeechAsset :one
SELECT * FROM clip_speech_assets WHERE id = ? AND owner_id = ? AND project_id = ?;

-- name: FindClipSpeechAsset :one
SELECT * FROM clip_speech_assets WHERE owner_id = ? AND project_id = ? AND input_hash = ? AND binding_digest = ? ORDER BY created_at DESC LIMIT 1;

-- name: ListClipSpeechAssets :many
SELECT * FROM clip_speech_assets WHERE owner_id = ? AND project_id = ? ORDER BY created_at;

-- name: PrepareClipSpeechCleanup :execrows
INSERT INTO clip_speech_cleanup(id,object_key,created_at)
SELECT ?,?,? WHERE EXISTS(SELECT 1 FROM clip_projects WHERE clip_projects.id=sqlc.arg(project) AND clip_projects.user_id=sqlc.arg(owner) AND deleting=0 AND finalized_at IS NULL)
ON CONFLICT(id) DO NOTHING;

-- name: PendingClipSpeechCleanup :many
SELECT * FROM clip_speech_cleanup WHERE created_at < ? ORDER BY created_at,id;
-- name: ClipSpeechAssetRetained :one
SELECT COUNT(*) FROM clip_speech_assets WHERE id=?;
-- name: DiscardRetainedClipSpeechCleanup :exec
DELETE FROM clip_speech_cleanup WHERE clip_speech_cleanup.id=sqlc.arg(id)
AND EXISTS(SELECT 1 FROM clip_speech_assets WHERE clip_speech_assets.id=sqlc.arg(id));
-- name: CompleteClipSpeechCleanup :exec
DELETE FROM clip_speech_cleanup WHERE id=?;
-- name: DeleteUnusedClipSpeechAsset :execrows
DELETE FROM clip_speech_assets WHERE clip_speech_assets.id=? AND clip_speech_assets.owner_id=? AND clip_speech_assets.project_id=?
AND EXISTS(SELECT 1 FROM clip_projects WHERE clip_projects.id=clip_speech_assets.project_id AND finalized_at IS NOT NULL)
AND NOT EXISTS(SELECT 1 FROM generation_jobs WHERE clip_project_id=clip_speech_assets.project_id AND status IN ('queued','running'));

-- name: ReserveClipSpeechRun :execrows
INSERT INTO clip_speech_jobs(id,owner_id,project_id,request_key,request_digest,operation_json,created_at)
SELECT ?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM clip_projects WHERE clip_projects.id=sqlc.arg(project_check) AND clip_projects.user_id=sqlc.arg(owner_check) AND clip_projects.edit_plan_revision=sqlc.arg(revision_check) AND clip_projects.finalized_at IS NULL)
ON CONFLICT(owner_id,project_id,request_key) DO NOTHING;

-- name: FindClipSpeechRun :one
SELECT operation_json,job_id FROM clip_speech_jobs WHERE owner_id=? AND project_id=? AND request_key=?;

-- name: GetClipSpeechRun :one
SELECT operation_json,job_id FROM clip_speech_jobs WHERE owner_id=? AND id=?;

-- name: BindClipSpeechRun :execrows
UPDATE clip_speech_jobs SET job_id=? WHERE owner_id=? AND id=? AND (job_id='' OR job_id=sqlc.arg(same_job));

-- name: ReserveClipSpeechCall :exec
INSERT INTO clip_speech_segments(id,owner_id,project_id,job_id,plan_revision,segment_id,input_hash,binding_digest,state,operation_json,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,'reserved',?,?,?) ON CONFLICT(job_id,segment_id) DO NOTHING;

-- name: ClaimClipSpeechCall :execrows
UPDATE clip_speech_segments SET state='claimed',updated_at=? WHERE owner_id=? AND project_id=? AND job_id=? AND segment_id=? AND input_hash=? AND binding_digest=? AND state='reserved';

-- name: GetClipSpeechCallState :one
SELECT state FROM clip_speech_segments WHERE owner_id=? AND project_id=? AND job_id=? AND segment_id=?;

-- name: FinishClipSpeechCall :execrows
UPDATE clip_speech_segments SET state=sqlc.arg(next_state),asset_id=COALESCE(sqlc.narg(asset),asset_id),updated_at=sqlc.arg(now)
WHERE owner_id=sqlc.arg(owner) AND project_id=sqlc.arg(project) AND job_id=sqlc.arg(job) AND segment_id=sqlc.arg(segment) AND state=sqlc.arg(expected_state);

-- name: ListIncompleteClipSpeechRuns :many
SELECT operation_json,job_id FROM clip_speech_jobs WHERE job_id='' OR EXISTS(SELECT 1 FROM clip_speech_segments WHERE clip_speech_segments.job_id=clip_speech_jobs.job_id AND state IN ('reserved','claimed','received')) ORDER BY created_at;
