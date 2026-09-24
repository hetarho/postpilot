-- name: GetContinuation :one
SELECT * FROM job_continuations WHERE job_id=?;

-- name: ParkJob :execrows
INSERT INTO job_continuations(job_id,wait_key,state,resume_policy,created_at,updated_at)
SELECT id,sqlc.arg(wait_key),'waiting',sqlc.arg(resume_policy),sqlc.arg(now),sqlc.arg(now)
FROM generation_jobs WHERE id=sqlc.arg(job_id) AND status='running' AND dispatch_ready=1 AND cancel_requested_at IS NULL
ON CONFLICT(job_id) DO UPDATE SET wait_key=excluded.wait_key,state='waiting',resume_policy=excluded.resume_policy,
ready_at=NULL,created_at=excluded.created_at,updated_at=excluded.updated_at
WHERE job_continuations.state='claimed' AND job_continuations.wait_key!=excluded.wait_key;

-- name: WakeContinuation :execrows
UPDATE job_continuations SET state='ready',ready_at=sqlc.arg(now),updated_at=sqlc.arg(now)
WHERE job_id=sqlc.arg(job_id) AND wait_key=sqlc.arg(wait_key) AND state='waiting'
AND EXISTS(SELECT 1 FROM generation_jobs j WHERE j.id=job_continuations.job_id AND j.status='running' AND j.cancel_requested_at IS NULL);

-- name: ClaimContinuation :execrows
UPDATE job_continuations SET state='claimed',updated_at=sqlc.arg(now)
WHERE job_id=sqlc.arg(job_id) AND state='ready';

-- name: HasPendingWait :one
SELECT EXISTS(SELECT 1 FROM job_continuations c JOIN generation_jobs j ON j.id=c.job_id
WHERE c.job_id=? AND c.state IN ('waiting','ready') AND j.status='running');

-- name: ReadyReplaySafeContinuations :exec
UPDATE job_continuations SET state='ready',ready_at=sqlc.arg(now),updated_at=sqlc.arg(now)
WHERE state='claimed' AND resume_policy='replay_safe'
AND EXISTS(SELECT 1 FROM generation_jobs j WHERE j.id=job_continuations.job_id AND j.status='running');

-- name: AcknowledgeWaitCancellation :execrows
UPDATE generation_jobs SET status='cancelled',finished_at=sqlc.arg(now),updated_at=sqlc.arg(now),
error=NULL,error_reason=NULL,error_params=NULL,technical_detail=NULL
WHERE generation_jobs.id=sqlc.arg(job_id) AND generation_jobs.status='running' AND generation_jobs.cancel_requested_at IS NOT NULL
AND EXISTS(SELECT 1 FROM job_continuations c WHERE c.job_id=generation_jobs.id AND c.wait_key=sqlc.arg(wait_key) AND c.state IN ('waiting','ready'));

-- name: WaitingContinuations :many
SELECT c.* FROM job_continuations c JOIN generation_jobs j ON j.id=c.job_id
WHERE j.status='running' AND c.state IN ('waiting','ready') AND c.job_id>sqlc.arg(after_id)
AND substr(c.wait_key,1,length(sqlc.arg(prefix)))=sqlc.arg(prefix)
ORDER BY c.job_id LIMIT 100;

-- name: FailWaitingContinuation :execrows
UPDATE generation_jobs SET status='failed',finished_at=sqlc.arg(now),updated_at=sqlc.arg(now),
error=NULL,error_reason=sqlc.arg(reason),error_params=sqlc.narg(params),technical_detail=NULL
WHERE id=sqlc.arg(job_id) AND status='running' AND cancel_requested_at IS NULL
AND EXISTS(SELECT 1 FROM job_continuations c WHERE c.job_id=generation_jobs.id AND c.wait_key=sqlc.arg(wait_key) AND c.state IN ('waiting','ready'));
