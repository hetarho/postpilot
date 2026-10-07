-- name: InsertAuthoringSession :exec
INSERT INTO configuration_authoring_sessions(id,user_id,kind,target_id,request_id,revision,phase,snapshot,created_at,updated_at,saved_baseline,working_source,draft_state,has_unpublished_changes,saved_available,publication_pending,target_conflict,display_name,candidate_count)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: GetAuthoringSession :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND id=?;
-- name: GetAuthoringSessionByRequest :one
SELECT * FROM configuration_authoring_sessions WHERE user_id=? AND request_id=?;
-- name: LatestAuthoringSession :one
SELECT * FROM configuration_authoring_sessions
WHERE user_id=sqlc.arg(user_id) AND kind=sqlc.arg(kind)
AND (target_id=sqlc.arg(target_id) OR (kind<>'writing_voice' AND target_id='' AND CAST(COALESCE(json_extract(snapshot,'$.saved.id'),'') AS TEXT)=sqlc.arg(target_id)))
ORDER BY updated_at DESC,id DESC LIMIT 1;
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

-- name: GetAuthoringRequestCapture :one
SELECT request_capture FROM configuration_authoring_operations
WHERE user_id=sqlc.arg(user_id) AND session_id=sqlc.arg(session_id)
 AND capture_revision=sqlc.arg(revision) AND mode=sqlc.arg(mode)
 AND capture_purged=0 AND request_capture IS NOT NULL
ORDER BY created_at DESC,id DESC LIMIT 1;
-- name: WriteAuthoringRequestCapture :execrows
UPDATE configuration_authoring_operations SET request_capture=sqlc.arg(request_capture),capture_revision=sqlc.arg(revision)
WHERE user_id=sqlc.arg(user_id) AND session_id=sqlc.arg(session_id) AND id=sqlc.arg(operation_id)
 AND base_revision=sqlc.arg(base_revision) AND capture_purged=0 AND request_capture IS NULL
 AND status IN ('pending','admitted') AND (job_id='' OR job_id=sqlc.arg(job_id));
-- name: BindAuthoringRequestCaptureRevision :exec
UPDATE configuration_authoring_operations SET capture_revision=sqlc.arg(revision)
WHERE user_id=sqlc.arg(user_id) AND id=sqlc.arg(operation_id)
 AND capture_purged=0 AND request_capture IS NOT NULL;
-- name: PurgeAuthoringRequestCaptures :exec
UPDATE configuration_authoring_operations SET request_capture=NULL,capture_revision=NULL,capture_purged=1
WHERE user_id=? AND session_id=?;

-- name: GetAuthoringMutation :one
SELECT * FROM configuration_authoring_mutations WHERE user_id=? AND operation_key=?;
-- name: InsertAuthoringMutation :exec
INSERT INTO configuration_authoring_mutations(user_id,session_id,operation_key,action,expected_revision,fingerprint,response,created_at) VALUES(?,?,?,?,?,?,?,?);
-- name: ListAuthoringSummaries :many
-- A fresh saved baseline must not conceal an existing meaningful private session.
-- Within that work class, preserve the newest current work; unsaved creations keep
-- their independent identities and ordinary summary pagination/order.
WITH candidates AS (
 SELECT c.*,
 CAST(CASE WHEN c.kind<>'writing_voice' AND c.target_id='' THEN COALESCE(json_extract(c.snapshot,'$.saved.id'),'') ELSE c.target_id END AS TEXT) AS summary_target_id,
 CASE WHEN c.has_unpublished_changes=1 OR c.publication_pending=1 OR c.target_conflict=1
  OR COALESCE(json_extract(c.snapshot,'$.active_job_id'),'')<>''
  OR COALESCE(json_extract(c.snapshot,'$.active_request_id'),'')<>''
  OR COALESCE(json_extract(c.snapshot,'$.pending_request'),'')<>''
  OR (c.phase<>'saved' AND COALESCE(json_array_length(c.snapshot,'$.candidates'),0)>0)
 THEN 1 ELSE 0 END AS meaningful_work
 FROM configuration_authoring_sessions AS c
 WHERE c.user_id=sqlc.arg(user_id) AND c.kind=sqlc.arg(kind)
), ranked AS (
 SELECT c.*,ROW_NUMBER() OVER (
  PARTITION BY c.summary_target_id,CASE WHEN c.summary_target_id='' THEN c.id ELSE '' END
  ORDER BY c.meaningful_work DESC,c.updated_at DESC,c.id DESC
 ) AS target_position
 FROM candidates AS c
)
SELECT c.id,c.kind,
CAST(c.summary_target_id AS TEXT) AS target_id,
c.revision,c.saved_available,c.has_unpublished_changes,
CAST(COALESCE(json_extract(c.snapshot,'$.active_job_id'),'') AS TEXT) AS active_job_id,
c.publication_pending,c.target_conflict,c.display_name,c.draft_state,c.updated_at,
CAST(COALESCE(json_extract(c.snapshot,'$.saved'),'null') AS TEXT) AS last_publication
FROM ranked AS c
WHERE c.target_position=1
AND (CAST(sqlc.arg(unsaved_only) AS INTEGER)=0 OR (c.target_id='' AND c.saved_available=0))
AND (CAST(sqlc.arg(cursor_time) AS TEXT)='' OR c.updated_at<sqlc.arg(cursor_time) OR (c.updated_at=sqlc.arg(cursor_time) AND c.id<sqlc.arg(cursor_id)))
ORDER BY c.updated_at DESC,c.id DESC LIMIT sqlc.arg(page_limit);
-- name: ConfirmAuthoringSaveMutations :exec
UPDATE configuration_authoring_mutations SET response=? WHERE user_id=? AND session_id=? AND action='save' AND expected_revision=?;
