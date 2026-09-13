-- name: SaveAttemptCheckpoint :execrows
INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json)
SELECT p.id,p.user_id,j.id,sqlc.arg(checkpoint_json) FROM clip_projects p JOIN generation_jobs j ON j.clip_project_id=p.id AND j.user_id=p.user_id
WHERE p.id=sqlc.arg(project_id) AND p.user_id=sqlc.arg(user_id) AND j.id=sqlc.arg(job_id)
AND p.deleting=0 AND p.finalized_at IS NULL AND j.status IN ('queued','running')
AND j.id=(SELECT latest.id FROM generation_jobs latest WHERE latest.clip_project_id=p.id ORDER BY latest.created_at DESC,latest.id DESC LIMIT 1)
ON CONFLICT(project_id) DO UPDATE SET user_id=excluded.user_id,job_id=excluded.job_id,checkpoint_json=excluded.checkpoint_json;

-- name: GetAttemptCheckpoint :one
SELECT c.checkpoint_json FROM clip_attempt_checkpoints c JOIN clip_projects p ON p.id=c.project_id AND p.user_id=c.user_id
JOIN generation_jobs j ON j.id=c.job_id AND j.user_id=c.user_id AND j.clip_project_id=c.project_id
WHERE c.project_id=sqlc.arg(project_id) AND c.user_id=sqlc.arg(user_id) AND c.job_id=sqlc.arg(job_id)
AND p.deleting=0 AND p.finalized_at IS NULL AND j.id=(SELECT latest.id FROM generation_jobs latest WHERE latest.clip_project_id=p.id ORDER BY latest.created_at DESC,latest.id DESC LIMIT 1);
