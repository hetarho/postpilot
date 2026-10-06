-- name: InsertAuthoringSession :exec
INSERT INTO configuration_authoring_sessions(id,user_id,kind,target_id,request_id,revision,phase,snapshot,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?);
-- name: GetAuthoringSession :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND id=?;
-- name: GetAuthoringSessionByRequest :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND request_id=?;
-- name: LatestAuthoringSession :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND kind=? AND target_id=? ORDER BY updated_at DESC,id DESC LIMIT 1;
-- name: UpdateAuthoringSession :execrows
UPDATE configuration_authoring_sessions SET revision=?,phase=?,snapshot=?,updated_at=? WHERE user_id=? AND id=? AND revision=?;
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
