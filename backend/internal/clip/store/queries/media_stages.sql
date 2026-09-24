-- name: InsertMediaStage :exec
INSERT INTO clip_media_stages(id,parent_job_id,user_id,project_id,expected_revision,stage_key,operation,contract_version,input_digest,input_payload,renderer_version,asset_version,created_at,queue_deadline_at,deadline_at,lease_ttl_ns,attempt_limit)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING;

-- name: GetMediaStage :one
SELECT * FROM clip_media_stages WHERE id=?;

-- name: GetMediaStageByParent :one
SELECT * FROM clip_media_stages WHERE parent_job_id=? AND stage_key=?;

-- name: ClaimMediaStage :one
UPDATE clip_media_stages SET state='running',current_attempt_id=sqlc.arg(attempt_id),attempt_count=attempt_count+1
WHERE id=(SELECT s.id FROM clip_media_stages s LEFT JOIN clip_media_attempts a ON a.id=s.current_attempt_id
 WHERE s.operation=sqlc.arg(operation) AND s.contract_version=sqlc.arg(contract_version)
 AND s.renderer_version=sqlc.arg(renderer_version) AND s.asset_version=sqlc.arg(asset_version)
 AND s.deadline_at>sqlc.arg(now) AND s.attempt_count<s.attempt_limit
 AND (s.attempt_count>0 OR s.queue_deadline_at>sqlc.arg(now))
 AND (s.retry_not_before IS NULL OR s.retry_not_before<=sqlc.arg(now))
 AND (s.state='queued' OR (s.state='running' AND a.lease_expires_at<=sqlc.arg(now)))
 ORDER BY s.created_at,s.id LIMIT 1)
RETURNING *;

-- name: ExpireMediaAttempts :exec
UPDATE clip_media_attempts SET outcome='expired',finished_at=sqlc.arg(now)
WHERE stage_id=sqlc.arg(stage_id) AND outcome IS NULL AND lease_expires_at<=sqlc.arg(now);

-- name: InsertMediaAttempt :exec
INSERT INTO clip_media_attempts(id,stage_id,ordinal,worker_id,token_hash,lease_expires_at,started_at,selected_profile,runtime_manifest)
VALUES(?,?,?,?,?,?,?,?,?);

-- name: GetMediaAttempt :one
SELECT * FROM clip_media_attempts WHERE id=?;

-- name: RenewMediaAttempt :one
UPDATE clip_media_attempts SET lease_expires_at=sqlc.arg(expires_at),progress=MAX(progress,sqlc.arg(progress))
WHERE clip_media_attempts.id=sqlc.arg(attempt_id) AND clip_media_attempts.stage_id=sqlc.arg(stage_id) AND clip_media_attempts.worker_id=sqlc.arg(worker_id) AND clip_media_attempts.token_hash=sqlc.arg(token_hash)
AND outcome IS NULL AND lease_expires_at>sqlc.arg(now)
AND EXISTS(SELECT 1 FROM clip_media_stages s WHERE s.id=clip_media_attempts.stage_id AND s.current_attempt_id=clip_media_attempts.id AND s.state='running' AND s.deadline_at>sqlc.arg(now))
RETURNING *;

-- name: AcceptMediaStage :one
UPDATE clip_media_stages SET state='succeeded',accepted_result=sqlc.arg(result)
WHERE clip_media_stages.id=sqlc.arg(stage_id) AND clip_media_stages.state='running' AND clip_media_stages.current_attempt_id=sqlc.arg(attempt_id) AND clip_media_stages.deadline_at>sqlc.arg(now)
AND EXISTS(SELECT 1 FROM clip_media_attempts a WHERE a.id=clip_media_stages.current_attempt_id AND a.worker_id=sqlc.arg(worker_id) AND a.token_hash=sqlc.arg(token_hash) AND a.outcome IS NULL AND a.lease_expires_at>sqlc.arg(now))
RETURNING *;

-- name: FinishMediaAttempt :exec
UPDATE clip_media_attempts SET outcome='succeeded',finished_at=?,progress=1000 WHERE id=?;

-- name: ReserveMediaArtifact :execrows
INSERT INTO clip_media_artifacts(attempt_id,slot,object_key,content_type,max_bytes,created_at)
SELECT a.id,sqlc.arg(slot),sqlc.arg(object_key),sqlc.arg(content_type),sqlc.arg(max_bytes),sqlc.arg(now)
FROM clip_media_attempts a JOIN clip_media_stages s ON s.id=a.stage_id AND s.current_attempt_id=a.id
WHERE s.id=sqlc.arg(stage_id) AND a.id=sqlc.arg(attempt_id) AND a.worker_id=sqlc.arg(worker_id) AND a.token_hash=sqlc.arg(token_hash)
AND s.state='running' AND s.deadline_at>sqlc.arg(now) AND a.lease_expires_at>sqlc.arg(now) AND a.outcome IS NULL
ON CONFLICT(attempt_id,slot) DO UPDATE SET slot=excluded.slot
WHERE clip_media_artifacts.object_key=excluded.object_key AND clip_media_artifacts.content_type=excluded.content_type AND clip_media_artifacts.max_bytes=excluded.max_bytes;

-- name: FailMediaStage :one
UPDATE clip_media_stages SET state='failed',failure=sqlc.arg(failure),failure_detail=sqlc.narg(failure_detail)
WHERE clip_media_stages.id=sqlc.arg(stage_id) AND clip_media_stages.state='running' AND clip_media_stages.current_attempt_id=sqlc.arg(attempt_id) AND clip_media_stages.deadline_at>sqlc.arg(now)
AND EXISTS(SELECT 1 FROM clip_media_attempts a WHERE a.id=clip_media_stages.current_attempt_id AND a.worker_id=sqlc.arg(worker_id) AND a.token_hash=sqlc.arg(token_hash) AND a.outcome IS NULL AND a.lease_expires_at>sqlc.arg(now))
RETURNING *;

-- name: FailMediaAttempt :exec
UPDATE clip_media_attempts SET outcome='failed',finished_at=? WHERE id=?;

-- name: MediaWaitingCount :one
SELECT COUNT(*) FROM clip_media_stages s LEFT JOIN clip_media_attempts a ON a.id=s.current_attempt_id
WHERE s.deadline_at>sqlc.arg(now) AND s.attempt_count<s.attempt_limit
AND (s.attempt_count>0 OR s.queue_deadline_at>sqlc.arg(now))
AND (s.state='queued' OR (s.state='running' AND a.lease_expires_at<=sqlc.arg(now)));

-- name: MediaActiveCount :one
SELECT COUNT(*) FROM clip_media_stages s JOIN clip_media_attempts a ON a.id=s.current_attempt_id
WHERE s.state='running' AND s.deadline_at>sqlc.arg(now) AND a.outcome IS NULL AND a.lease_expires_at>sqlc.arg(now);

-- name: MediaOwnActiveCount :one
SELECT COUNT(*) FROM clip_media_stages s JOIN clip_media_attempts a ON a.id=s.current_attempt_id
WHERE s.state='running' AND s.deadline_at>sqlc.arg(now) AND a.outcome IS NULL AND a.lease_expires_at>sqlc.arg(now) AND a.worker_id=sqlc.arg(worker_id);

-- name: MediaIncompatibleCount :one
SELECT COUNT(*) FROM clip_media_stages s LEFT JOIN clip_media_attempts a ON a.id=s.current_attempt_id
WHERE s.operation=sqlc.arg(operation) AND s.deadline_at>sqlc.arg(now) AND s.attempt_count<s.attempt_limit
AND (s.attempt_count>0 OR s.queue_deadline_at>sqlc.arg(now))
AND (s.state='queued' OR (s.state='running' AND a.lease_expires_at<=sqlc.arg(now)))
AND (s.contract_version!=sqlc.arg(contract_version) OR s.renderer_version!=sqlc.arg(renderer_version) OR s.asset_version!=sqlc.arg(asset_version));
