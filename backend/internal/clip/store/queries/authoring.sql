-- name: GetAuthoringPublication :one
SELECT publication_key,target_id FROM video_template_authoring_publications
WHERE user_id=? AND session_id=? AND revision=?;

-- name: InsertAuthoringPublication :exec
INSERT INTO video_template_authoring_publications(user_id,session_id,revision,publication_key,target_id,created_at)
VALUES(?,?,?,?,?,?);

-- name: LockAuthoringPublication :exec
UPDATE video_templates SET name=name WHERE 0;
