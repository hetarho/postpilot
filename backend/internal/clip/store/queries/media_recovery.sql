-- name: MediaRecoveryStages :many
SELECT * FROM clip_media_stages WHERE reconciled_at IS NULL AND id>sqlc.arg(after_id) ORDER BY id LIMIT 100;

-- name: SetMediaRecoveryState :exec
UPDATE clip_media_stages SET state=sqlc.arg(state),failure_detail=NULL,failure=sqlc.narg(failure),retry_not_before=sqlc.narg(retry_not_before)
WHERE id=sqlc.arg(id) AND state!='succeeded';

-- name: MarkMediaReconciled :exec
UPDATE clip_media_stages SET reconciled_at=? WHERE id=?;

-- name: StopMediaAttempt :exec
UPDATE clip_media_attempts SET outcome=sqlc.arg(outcome),finished_at=sqlc.arg(now)
WHERE id=sqlc.arg(id) AND outcome IS NULL;

-- name: RetireMediaStageArtifacts :exec
DELETE FROM clip_media_artifacts WHERE canonical=0 AND attempt_id IN
(SELECT id FROM clip_media_attempts WHERE stage_id=sqlc.arg(stage_id) AND (sqlc.arg(all_attempts)=1 OR id!=sqlc.arg(current_attempt_id)));

-- name: RetireSupersededMediaResults :exec
DELETE FROM clip_media_artifacts WHERE object_key IN
(SELECT a.object_key FROM clip_media_artifacts a WHERE a.canonical=1
 AND NOT EXISTS(SELECT 1 FROM clip_projects p WHERE p.result_key=a.object_key)
 AND NOT EXISTS(SELECT 1 FROM clip_attempt_results r WHERE r.result_key=a.object_key)
 ORDER BY a.object_key LIMIT 100);

-- name: DueMediaDeletions :many
SELECT * FROM clip_media_deletions d WHERE d.not_before<=sqlc.arg(cutoff)
 AND NOT EXISTS(SELECT 1 FROM clip_projects p WHERE p.result_key=d.object_key)
 AND NOT EXISTS(SELECT 1 FROM clip_attempt_results r WHERE r.result_key=d.object_key)
 AND NOT EXISTS(SELECT 1 FROM clip_media_artifacts a WHERE a.object_key=d.object_key)
ORDER BY d.not_before,d.object_key LIMIT 100;

-- name: RemoveMediaDeletion :exec
DELETE FROM clip_media_deletions WHERE object_key=?;

-- name: QueueOrphanMediaDeletion :exec
INSERT INTO clip_media_deletions(object_key,not_before,created_at)
SELECT sqlc.arg(object_key),sqlc.arg(now),sqlc.arg(now)
WHERE NOT EXISTS(SELECT 1 FROM clip_media_artifacts WHERE object_key=sqlc.arg(object_key))
 AND NOT EXISTS(SELECT 1 FROM clip_projects WHERE result_key=sqlc.arg(object_key))
 AND NOT EXISTS(SELECT 1 FROM clip_attempt_results WHERE result_key=sqlc.arg(object_key))
ON CONFLICT(object_key) DO NOTHING;
