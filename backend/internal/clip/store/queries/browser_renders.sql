-- name: BeginBrowserRender :exec
INSERT INTO clip_browser_renders(id,user_id,project_id,plan_revision,ratio,duration_ms,has_audio,created_at) VALUES(?,?,?,?,?,?,?,?);
-- name: GetBrowserRender :one
SELECT * FROM clip_browser_renders WHERE id=? AND user_id=?;
-- name: SaveBrowserRenderVerdict :execrows
UPDATE clip_browser_renders SET verdict_json=?,reported_at=? WHERE id=? AND user_id=? AND verdict_json IS NULL;
-- name: ReserveBrowserRenderUpload :execrows
UPDATE clip_browser_renders SET upload_bytes=? WHERE id=? AND user_id=? AND upload_bytes=0 AND stored_at IS NULL;
-- name: CompleteBrowserRender :execrows
UPDATE clip_browser_renders SET stored_at=? WHERE id=? AND user_id=? AND stored_at IS NULL;
