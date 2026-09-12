-- name: StageAttemptResult :exec
INSERT INTO clip_attempt_results(job_id,user_id,project_id,expected_revision,analysis_json,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(job_id) DO NOTHING;
-- name: GetAttemptResult :one
SELECT * FROM clip_attempt_results WHERE job_id=?;
-- name: PendingAttemptResults :many
SELECT * FROM clip_attempt_results ORDER BY result_created_at,job_id;
-- name: DeleteAttemptResult :exec
DELETE FROM clip_attempt_results WHERE job_id=?;
