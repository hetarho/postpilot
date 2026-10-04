-- name: GetOperation :one
SELECT * FROM spoken_voice_operations WHERE id = ? AND owner_id = ?;
-- name: GetOperationRequest :one
SELECT * FROM spoken_voice_operations WHERE owner_id = ? AND kind = ? AND idempotency_key = ?;
-- name: InsertOperation :exec
INSERT INTO spoken_voice_operations (id,owner_id,kind,state,job_id,idempotency_key,request_digest,scope_digest,candidate_id,received_handle,sample_asset_id,snapshot_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: UpdateOperation :execrows
UPDATE spoken_voice_operations SET state=sqlc.arg(next_state),job_id=sqlc.arg(job),received_handle=sqlc.arg(handle),snapshot_json=sqlc.arg(snapshot),updated_at=sqlc.arg(at)
WHERE id=sqlc.arg(id) AND owner_id=sqlc.arg(owner) AND state=sqlc.arg(expected);
-- name: RecoverableOperations :many
SELECT * FROM spoken_voice_operations WHERE state IN ('reserved','queued','claimed','received') ORDER BY created_at,id;
-- name: DeleteOwnerOperations :exec
DELETE FROM spoken_voice_operations WHERE owner_id = ?;
-- name: UnresolvedConfirmation :one
SELECT COUNT(*) FROM spoken_voice_operations WHERE owner_id = ? AND candidate_id = ? AND kind='voice_confirm'
AND (state IN ('reserved','queued','claimed','received','unresolved') OR (state='failed' AND received_handle<>'')
OR (state='cancelled' AND (received_handle<>'' OR json_extract(snapshot_json,'$.Calling[0]')=1)));
-- name: GetProbeAudio :one
SELECT * FROM spoken_probe_audio WHERE owner_id = ? AND voice_id = ? AND input_digest = ?;
-- name: InsertProbeAudio :exec
INSERT INTO spoken_probe_audio(owner_id,voice_id,input_digest,origin_operation_id,origin_job_id,asset_id,evidence_json,timing_json) VALUES (?,?,?,?,?,?,?,?);

-- name: QualificationOperations :many
SELECT * FROM spoken_voice_operations WHERE owner_id=sqlc.arg(owner) AND json_extract(snapshot_json,'$.QualificationSessionID')=sqlc.arg(session) ORDER BY created_at,id;
