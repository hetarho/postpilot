-- name: FinalizeClipProject :execrows
UPDATE clip_projects SET finalized_at=sqlc.arg(now),finalized_plan_revision=edit_plan_revision,finalized_result_key=result_key,source_access_revoked_at=COALESCE(source_access_revoked_at,sqlc.arg(now)),updated_at=sqlc.arg(now),
published_url=sqlc.narg(published_url),published_at=CASE WHEN sqlc.narg(published_url) IS NULL THEN NULL ELSE sqlc.arg(now) END
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND deleting=0 AND finalized_at IS NULL
AND edit_plan_revision=sqlc.arg(expected_revision) AND rendered_plan_revision=edit_plan_revision AND confirmed_plan_revision=edit_plan_revision
AND result_id=sqlc.arg(expected_result_id) AND result_key IS NOT NULL;
-- CLIP-75, CLIP-229: confirmation records the plan revision; the caller excludes active jobs.
-- name: ConfirmClipProject :execrows
UPDATE clip_projects SET confirmed_plan_revision=sqlc.arg(expected_revision),confirmed_at=sqlc.arg(now),updated_at=sqlc.arg(now)
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND deleting=0 AND finalized_at IS NULL
AND edit_plan_revision=sqlc.arg(expected_revision);
-- CLIP-232: the posted URL of a finalized project; nothing else changes.
-- name: SetClipPublishedURL :execrows
UPDATE clip_projects SET published_url=sqlc.arg(url),published_at=sqlc.arg(now)
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND deleting=0 AND finalized_at IS NOT NULL;
-- CLIP-231: advisory ticks, writable before and after finalization.
-- name: SetClipChecklistTicks :execrows
UPDATE clip_projects SET checklist_ticks=sqlc.arg(ticks),updated_at=sqlc.arg(now)
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id);
