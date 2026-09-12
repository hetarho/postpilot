-- name: FinalizeClipProject :execrows
UPDATE clip_projects SET finalized_at=sqlc.arg(now),finalized_plan_revision=edit_plan_revision,finalized_result_key=result_key,source_access_revoked_at=COALESCE(source_access_revoked_at,sqlc.arg(now)),updated_at=sqlc.arg(now)
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND deleting=0 AND finalized_at IS NULL
AND edit_plan_revision=sqlc.arg(expected_revision) AND rendered_plan_revision=edit_plan_revision
AND result_id=sqlc.arg(expected_result_id) AND result_key IS NOT NULL;
