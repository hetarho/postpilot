-- name: GetClipRecovery :one
SELECT r.job_id,r.state_json FROM clip_recovery_states r JOIN clip_projects p ON p.id=r.project_id AND p.user_id=r.user_id
WHERE r.project_id=sqlc.arg(project_id) AND r.user_id=sqlc.arg(user_id) AND p.deleting=0 AND p.finalized_at IS NULL;

-- name: SaveClipRecovery :execrows
-- The unique active-clip-job index makes this attempt the only live writer.
-- A terminal predecessor can have a later timestamp after a clock rollback.
INSERT INTO clip_recovery_states(project_id,user_id,job_id,state_json)
SELECT p.id,p.user_id,j.id,sqlc.arg(state_json) FROM clip_projects p JOIN generation_jobs j ON j.clip_project_id=p.id AND j.user_id=p.user_id
WHERE p.id=sqlc.arg(project_id) AND p.user_id=sqlc.arg(user_id) AND j.id=sqlc.arg(job_id)
AND p.deleting=0 AND p.finalized_at IS NULL AND j.status IN ('queued','running')
ON CONFLICT(project_id) DO UPDATE SET user_id=excluded.user_id,job_id=excluded.job_id,state_json=excluded.state_json;

-- name: CorrectClipSpokenRecovery :execrows
UPDATE clip_recovery_states SET state_json=sqlc.arg(state_json)
WHERE clip_recovery_states.project_id=sqlc.arg(project_id) AND clip_recovery_states.user_id=sqlc.arg(user_id) AND clip_recovery_states.state_json=sqlc.arg(expected_json)
AND EXISTS(SELECT 1 FROM clip_projects p WHERE p.id=clip_recovery_states.project_id AND p.user_id=clip_recovery_states.user_id AND p.deleting=0 AND p.finalized_at IS NULL AND p.edit_plan_json IS NULL)
AND NOT EXISTS(SELECT 1 FROM generation_jobs j WHERE j.clip_project_id=clip_recovery_states.project_id AND j.user_id=clip_recovery_states.user_id AND j.status IN ('queued','running'));
