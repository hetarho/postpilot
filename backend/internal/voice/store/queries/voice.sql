-- Voices. The account owns the directory; every profile row below belongs to exactly one
-- voice, and every query names both so a same-account id from another voice cannot reach
-- this one's aggregate.
-- NOTE: keep this file ASCII. sqlc's SELECT * rewriting uses byte offsets where it means
-- rune offsets, so one multibyte character here corrupts every later expansion.

-- name: InsertVoice :exec
INSERT INTO voices (id, user_id, name, is_default, deleted_at, created_at, updated_at)
VALUES (?, ?, ?, ?, NULL, ?, ?);

-- Every directory read carries made (a current analysis exists, POST-23), the sample count
-- and the current analysis's publication time for the row's meta line (VOICE-52). The two
-- reads select the same columns, so their rows convert to one another.

-- name: ListVoices :many
SELECT v.id, v.user_id, v.name, v.is_default, v.deleted_at, v.created_at, v.updated_at,
       CAST(EXISTS (
           SELECT 1 FROM voice_analyses a
           WHERE a.voice_id = v.id AND a.user_id = v.user_id AND a.slot = 'current'
       ) AS INTEGER) AS made,
       CAST((
           SELECT count(*) FROM voice_samples s
           WHERE s.voice_id = v.id AND s.user_id = v.user_id
       ) AS INTEGER) AS sample_count,
       CAST(coalesce((
           SELECT a.created_at FROM voice_analyses a
           WHERE a.voice_id = v.id AND a.user_id = v.user_id AND a.slot = 'current'
       ), '') AS TEXT) AS analyzed_at
FROM voices v WHERE v.user_id = ?
ORDER BY v.deleted_at IS NOT NULL, v.is_default DESC, v.name, v.id;

-- name: GetVoice :one
SELECT v.id, v.user_id, v.name, v.is_default, v.deleted_at, v.created_at, v.updated_at,
       CAST(EXISTS (
           SELECT 1 FROM voice_analyses a
           WHERE a.voice_id = v.id AND a.user_id = v.user_id AND a.slot = 'current'
       ) AS INTEGER) AS made,
       CAST((
           SELECT count(*) FROM voice_samples s
           WHERE s.voice_id = v.id AND s.user_id = v.user_id
       ) AS INTEGER) AS sample_count,
       CAST(coalesce((
           SELECT a.created_at FROM voice_analyses a
           WHERE a.voice_id = v.id AND a.user_id = v.user_id AND a.slot = 'current'
       ), '') AS TEXT) AS analyzed_at
FROM voices v WHERE v.id = ? AND v.user_id = ?;

-- name: RenameVoice :execrows
UPDATE voices SET name = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: ClearDefaultVoice :exec
UPDATE voices SET is_default = 0, updated_at = ? WHERE user_id = ? AND is_default = 1;

-- name: SetDefaultVoice :execrows
UPDATE voices SET is_default = 1, updated_at = ?
WHERE id = ? AND user_id = ? AND deleted_at IS NULL;

-- name: SoftDeleteVoice :execrows
-- Deleting the default leaves the account with none (VOICE-13): a tombstone is never it.
UPDATE voices SET deleted_at = ?, updated_at = ?, is_default = 0
WHERE id = ? AND user_id = ? AND deleted_at IS NULL;

-- name: RestoreVoice :execrows
UPDATE voices SET deleted_at = NULL, updated_at = ?
WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL;

-- name: InsertSample :exec
INSERT INTO voice_samples (id, voice_id, user_id, kind, prompt_key, label, body, photo_key, photo_width, photo_height, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListSamples :many
SELECT id, kind, prompt_key, label, length(body) AS chars, photo_key, created_at
FROM voice_samples
WHERE voice_id = ? AND user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: ListSampleBodies :many
SELECT id, kind, prompt_key, label, body, photo_key, photo_width, photo_height, created_at
FROM voice_samples
WHERE voice_id = ? AND user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: GetSampleBody :one
SELECT id, kind, prompt_key, label, body, photo_key, photo_width, photo_height, created_at
FROM voice_samples
WHERE id = ? AND voice_id = ? AND user_id = ?;

-- name: DeleteSample :one
-- The row goes first and names the photo key, so the object can follow it (POST-39).
DELETE FROM voice_samples WHERE id = ? AND voice_id = ? AND user_id = ?
RETURNING photo_key;

-- name: CountSamples :one
SELECT count(*) FROM voice_samples WHERE voice_id = ? AND user_id = ?;

-- name: InsertPhotoUpload :exec
INSERT INTO voice_photo_uploads (id, user_id, voice_id, prompt_key, object_key, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetPhotoUpload :one
SELECT id, user_id, voice_id, prompt_key, object_key, expires_at, created_at
FROM voice_photo_uploads
WHERE id = ? AND voice_id = ? AND user_id = ?;

-- name: DeletePhotoUpload :exec
DELETE FROM voice_photo_uploads WHERE id = ?;

-- name: ListPhotoUploadsExpiredBefore :many
SELECT id, user_id, voice_id, prompt_key, object_key, expires_at, created_at
FROM voice_photo_uploads
WHERE expires_at < ?;

-- name: PhotoKeyInUse :one
SELECT CAST(EXISTS (SELECT 1 FROM voice_samples WHERE photo_key = ?) AS INTEGER) AS in_use;

-- name: ListSamplePhotoKeys :many
SELECT photo_key FROM voice_samples WHERE photo_key IS NOT NULL;

-- name: ListPhotoUploadKeys :many
SELECT object_key FROM voice_photo_uploads;

-- name: GetAnalysis :one
SELECT snapshot, material_ids, analyze_model, created_at
FROM voice_analyses
WHERE voice_id = ? AND user_id = ? AND slot = ?;

-- name: CountAnalysisSlot :one
SELECT count(*) FROM voice_analyses WHERE voice_id = ? AND user_id = ? AND slot = ?;

-- name: DeleteAnalysisSlot :exec
DELETE FROM voice_analyses WHERE voice_id = ? AND user_id = ? AND slot = ?;

-- name: MoveAnalysisSlot :execrows
UPDATE voice_analyses SET slot = sqlc.arg(to_slot)
WHERE voice_id = sqlc.arg(voice_id) AND user_id = sqlc.arg(user_id) AND slot = sqlc.arg(from_slot);

-- name: InsertCurrentAnalysis :exec
INSERT INTO voice_analyses (voice_id, user_id, slot, snapshot, material_ids, analyze_model, created_at)
VALUES (?, ?, 'current', ?, ?, ?, ?);
