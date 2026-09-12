-- name: ListVideoTemplates :many
SELECT * FROM video_templates WHERE user_id = ? ORDER BY name, id;
-- name: GetVideoTemplate :one
SELECT * FROM video_templates WHERE id = ? AND user_id = ?;
-- name: InsertVideoTemplate :exec
INSERT INTO video_templates(id, user_id, name, information_fields, cut_guidance, copy_styles, accent, preset, caption_pace, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: CountTemplateProjects :one
SELECT count(*) FROM clip_projects WHERE video_template_id = ? AND user_id = ?;
-- name: ProjectsForTemplate :many
SELECT * FROM clip_projects WHERE video_template_id = ? AND user_id = ? ORDER BY id;
-- name: SaveTemplateComposition :execrows
UPDATE video_templates SET composition_body=?,composition_legacy=? WHERE id=? AND user_id=?;
-- name: SaveProjectComposition :execrows
UPDATE clip_projects SET composition_snapshot_json=?,composition_inputs_json=? WHERE id=? AND user_id=?;
-- name: TouchCompositionRevision :execrows
UPDATE clip_projects SET edit_plan_revision=edit_plan_revision+1,updated_at=? WHERE id=? AND user_id=? AND deleting=0;
-- name: DeleteVideoTemplate :execrows
DELETE FROM video_templates WHERE id = ? AND user_id = ?;
-- name: ListClipProjects :many
SELECT * FROM clip_projects WHERE user_id = ? ORDER BY updated_at DESC, id;
-- name: GetClipProject :one
SELECT * FROM clip_projects WHERE id = ? AND user_id = ?;
-- name: InsertClipProject :exec
INSERT INTO clip_projects(id, user_id, title, video_template_id, ratio, target_duration_ms, disclosure, cta, hide_disclosure, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: DeleteClipProject :execrows
DELETE FROM clip_projects WHERE id = ? AND user_id = ?;
-- name: ListClipAnswers :many
SELECT label, answer FROM clip_project_answers WHERE project_id = ? AND user_id = ? ORDER BY label;
-- name: UpsertClipAnswer :exec
INSERT INTO clip_project_answers(project_id, user_id, label, answer, updated_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(project_id, label) DO UPDATE SET answer = excluded.answer, updated_at = excluded.updated_at WHERE user_id = excluded.user_id;
-- name: UpdateVideoTemplateName :execrows
UPDATE video_templates SET name = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateVideoTemplateInformationFields :execrows
UPDATE video_templates SET information_fields = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateVideoTemplateCutGuidance :execrows
UPDATE video_templates SET cut_guidance = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateVideoTemplateCopyStyles :execrows
UPDATE video_templates SET copy_styles = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateVideoTemplateAccent :execrows
UPDATE video_templates SET accent = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateVideoTemplatePreset :execrows
UPDATE video_templates SET preset = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateClipTitle :execrows
UPDATE clip_projects SET title = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateClipVideoTemplateID :execrows
UPDATE clip_projects SET video_template_id = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateClipTargetDurationMS :execrows
UPDATE clip_projects SET target_duration_ms = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateClipDisclosure :execrows
UPDATE clip_projects SET disclosure = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: UpdateClipCTA :execrows
UPDATE clip_projects SET cta = ?, updated_at = ? WHERE id = ? AND user_id = ?;
-- name: TouchClip :execrows
UPDATE clip_projects SET updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdateVideoTemplateCaptionPace :execrows
UPDATE video_templates SET caption_pace = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdateClipHideDisclosure :execrows
UPDATE clip_projects SET hide_disclosure = sqlc.arg(hide_disclosure),
    edit_plan_revision = edit_plan_revision + CASE WHEN edit_plan_json IS NOT NULL AND hide_disclosure != sqlc.arg(hide_disclosure) THEN 1 ELSE 0 END,
    updated_at = sqlc.arg(updated_at) WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);
