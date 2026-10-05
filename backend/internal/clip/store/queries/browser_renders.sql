-- name: BeginBrowserRender :exec
INSERT INTO clip_browser_renders(id,user_id,project_id,plan_revision,ratio,duration_ms,has_audio,created_at,speech_json) VALUES(?,?,?,?,?,?,?,?,?);
-- name: GetBrowserRender :one
SELECT * FROM clip_browser_renders WHERE id=? AND user_id=?;
-- name: SaveBrowserRenderVerdict :execrows
UPDATE clip_browser_renders SET verdict_json=?,reported_at=? WHERE id=? AND user_id=? AND verdict_json IS NULL;
-- name: ReserveBrowserRenderUpload :execrows
UPDATE clip_browser_renders SET upload_bytes=? WHERE id=? AND user_id=? AND upload_bytes=0 AND stored_at IS NULL;
-- name: CompleteBrowserRender :execrows
UPDATE clip_browser_renders SET stored_at=? WHERE id=? AND user_id=? AND stored_at IS NULL;
-- name: CancelBrowserRender :execrows
UPDATE clip_browser_renders SET cancelled_at=? WHERE id=? AND user_id=? AND cancelled_at IS NULL AND stored_at IS NULL;
-- name: BindBrowserRenderSampling :execrows
UPDATE clip_browser_renders SET sample_job_id=sqlc.arg(job_id) WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND cancelled_at IS NULL AND (sample_job_id IS NULL OR sample_job_id=sqlc.arg(job_id));
-- name: SaveBrowserRenderGrounds :execrows
UPDATE clip_browser_renders SET grounds_json=sqlc.arg(grounds_json), sampled_at=sqlc.arg(sampled_at) WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND sample_job_id=sqlc.arg(job_id) AND plan_revision=sqlc.arg(plan_revision) AND sampled_at IS NULL AND cancelled_at IS NULL;
