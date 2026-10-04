-- name: InsertVoiceCheck :exec
-- One row per check (VOICE-43): its frozen inputs, then its piece or its failure.
INSERT INTO voice_checks (id, user_id, voice_id, prompt_key, material_id, analysis_created_at, projection, write_model, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'queued', ?, ?);

-- name: DeleteVoiceCheck :exec
-- A check whose enqueue failed leaves no result (VOICE-44).
DELETE FROM voice_checks WHERE id = ? AND user_id = ?;

-- name: GetVoiceCheck :one
SELECT id, voice_id, prompt_key, material_id, analysis_created_at, projection, write_model, status,
       piece, error_reason, error_params, technical_detail, created_at, updated_at
FROM voice_checks
WHERE id = ? AND user_id = ?;

-- name: ListVoiceChecks :many
-- The list never shows the frozen projection, so it is not read; it comes back empty only so the
-- row keeps GetVoiceCheck's shape.
SELECT id, voice_id, prompt_key, material_id, analysis_created_at, CAST('' AS TEXT) AS projection, write_model, status,
       piece, error_reason, error_params, technical_detail, created_at, updated_at
FROM voice_checks
WHERE voice_id = ? AND user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: MarkVoiceCheckRunning :execrows
UPDATE voice_checks SET status = 'running', updated_at = ?
WHERE id = ? AND user_id = ? AND status IN ('queued', 'running');

-- name: FinishVoiceCheck :execrows
UPDATE voice_checks SET status = 'done', piece = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND status = 'running';

-- name: FailVoiceCheck :execrows
UPDATE voice_checks SET status = 'failed', error_reason = ?, error_params = ?, technical_detail = ?, updated_at = ?
WHERE id = ? AND user_id = ? AND status IN ('queued', 'running');
