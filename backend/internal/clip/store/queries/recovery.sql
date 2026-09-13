-- name: GetClipRecovery :one
SELECT r.job_id,r.state_json FROM clip_recovery_states r JOIN clip_projects p ON p.id=r.project_id AND p.user_id=r.user_id
WHERE r.project_id=sqlc.arg(project_id) AND r.user_id=sqlc.arg(user_id) AND p.deleting=0 AND p.finalized_at IS NULL;

-- name: SaveClipRecovery :execrows
INSERT INTO clip_recovery_states(project_id,user_id,job_id,state_json)
SELECT p.id,p.user_id,j.id,sqlc.arg(state_json) FROM clip_projects p JOIN generation_jobs j ON j.clip_project_id=p.id AND j.user_id=p.user_id
WHERE p.id=sqlc.arg(project_id) AND p.user_id=sqlc.arg(user_id) AND j.id=sqlc.arg(job_id)
AND p.deleting=0 AND p.finalized_at IS NULL AND j.status IN ('queued','running')
AND j.id=(SELECT latest.id FROM generation_jobs latest WHERE latest.clip_project_id=p.id ORDER BY latest.created_at DESC,latest.id DESC LIMIT 1)
ON CONFLICT(project_id) DO UPDATE SET user_id=excluded.user_id,job_id=excluded.job_id,state_json=excluded.state_json;
