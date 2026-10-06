-- name: GetAuthoringPublication :one
SELECT publication_key,target_id, kind FROM guideline_authoring_publications
WHERE user_id=? AND session_id=? AND revision=?;

-- name: InsertAuthoringPublication :exec
INSERT INTO guideline_authoring_publications(user_id,session_id,revision,publication_key,target_id, kind,created_at)
VALUES(?,?,?,?,?,?,?);

-- name: LockAuthoringPublication :exec
UPDATE guidelines SET title=title WHERE 0;
