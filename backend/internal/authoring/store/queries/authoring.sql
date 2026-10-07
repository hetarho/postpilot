-- name: InsertAuthoringSession :exec
INSERT INTO configuration_authoring_sessions(id,user_id,kind,target_id,request_id,revision,phase,snapshot,created_at,updated_at,saved_baseline,working_source,draft_state,has_unpublished_changes,saved_available,publication_pending,target_conflict,display_name,candidate_count)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: GetAuthoringSession :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND id=?;
-- name: GetAuthoringSessionByRequest :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND request_id=?;
-- name: LatestAuthoringSession :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND kind=? AND target_id=? ORDER BY updated_at DESC,id DESC LIMIT 1;
-- name: UpdateAuthoringSession :execrows
UPDATE configuration_authoring_sessions SET revision=?,phase=?,snapshot=?,updated_at=?,saved_baseline=?,working_source=?,draft_state=?,has_unpublished_changes=?,saved_available=?,publication_pending=?,target_conflict=?,display_name=?,candidate_count=? WHERE user_id=? AND id=? AND revision=?;
-- name: InsertAuthoringOperation :exec
INSERT INTO configuration_authoring_operations(id,user_id,session_id,request_id,fingerprint,base_revision,mode,payload,job_id,status,failure_reason,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?);
-- name: GetAuthoringOperation :one
SELECT * FROM configuration_authoring_operations WHERE user_id=? AND session_id=? AND request_id=?;
-- name: GetAuthoringOperationByID :one
SELECT * FROM configuration_authoring_operations WHERE user_id=? AND id=?;
-- name: SetAuthoringOperation :execrows
UPDATE configuration_authoring_operations SET job_id=?,status=?,failure_reason=? WHERE user_id=? AND id=?;
-- name: ActiveAuthoringOperation :one
SELECT * FROM configuration_authoring_operations WHERE user_id=? AND session_id=? AND status IN ('pending','admitted') LIMIT 1;

-- name: GetAuthoringMutation :one
SELECT * FROM configuration_authoring_mutations WHERE user_id=? AND operation_key=?;
-- name: InsertAuthoringMutation :exec
INSERT INTO configuration_authoring_mutations(user_id,session_id,operation_key,action,expected_revision,fingerprint,response,created_at) VALUES(?,?,?,?,?,?,?,?);
-- name: ListAuthoringSummaries :many
SELECT c.id,c.kind,c.target_id,c.revision,c.saved_available,c.has_unpublished_changes,
CAST(COALESCE(json_extract(c.snapshot,'$.active_job_id'),'') AS TEXT) AS active_job_id,
c.publication_pending,c.target_conflict,c.display_name,c.draft_state,c.updated_at,
CAST(COALESCE(json_extract(c.snapshot,'$.saved'),'null') AS TEXT) AS last_publication
FROM configuration_authoring_sessions AS c
WHERE c.user_id=sqlc.arg(user_id) AND c.kind=sqlc.arg(kind)
AND (CAST(sqlc.arg(unsaved_only) AS INTEGER)=0 OR (c.target_id='' AND c.saved_available=0))
AND (c.target_id='' OR NOT EXISTS (
 SELECT 1 FROM configuration_authoring_sessions AS newer
 WHERE newer.user_id=c.user_id AND newer.kind=c.kind AND newer.target_id=c.target_id
 AND (newer.updated_at>c.updated_at OR (newer.updated_at=c.updated_at AND newer.id>c.id))
))
AND (CAST(sqlc.arg(cursor_time) AS TEXT)='' OR c.updated_at<sqlc.arg(cursor_time) OR (c.updated_at=sqlc.arg(cursor_time) AND c.id<sqlc.arg(cursor_id)))
ORDER BY c.updated_at DESC,c.id DESC LIMIT sqlc.arg(page_limit);
-- name: ConfirmAuthoringSaveMutations :exec
UPDATE configuration_authoring_mutations SET response=? WHERE user_id=? AND session_id=? AND action='save' AND expected_revision=?;
