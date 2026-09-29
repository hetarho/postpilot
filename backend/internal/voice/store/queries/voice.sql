-- Voices. The account owns the directory; every profile row below belongs to exactly one
-- voice, and every query names both so a same-account id from another voice cannot reach
-- this one's aggregate.
-- NOTE: keep this file ASCII. sqlc's SELECT * rewriting uses byte offsets where it means
-- rune offsets, so one multibyte character here corrupts every later expansion.

-- name: InsertVoice :exec
INSERT INTO voices (id, user_id, name, is_default, deleted_at, created_at, updated_at)
VALUES (?, ?, ?, ?, NULL, ?, ?);

-- Every directory read carries made (a published analysis exists, POST-23), the sample count
-- and the current analysis's publication time for the row's meta line (VOICE-52). The two
-- reads select the same columns, so their rows convert to one another.

-- name: ListVoices :many
SELECT v.id, v.user_id, v.name, v.is_default, v.deleted_at, v.created_at, v.updated_at,
       CAST(EXISTS (
           SELECT 1 FROM voice_profiles p
           WHERE p.voice_id = v.id AND p.user_id = v.user_id AND p.current_version > 0
       ) AS INTEGER) AS made,
       CAST((
           SELECT count(*) FROM voice_samples s
           WHERE s.voice_id = v.id AND s.user_id = v.user_id
       ) AS INTEGER) AS sample_count,
       CAST(coalesce((
           SELECT pv.created_at FROM voice_profiles p
           JOIN voice_profile_versions pv
             ON pv.voice_id = p.voice_id AND pv.user_id = p.user_id AND pv.version = p.current_version
           WHERE p.voice_id = v.id AND p.user_id = v.user_id
       ), '') AS TEXT) AS analyzed_at
FROM voices v WHERE v.user_id = ?
ORDER BY v.deleted_at IS NOT NULL, v.is_default DESC, v.name, v.id;

-- name: GetVoice :one
SELECT v.id, v.user_id, v.name, v.is_default, v.deleted_at, v.created_at, v.updated_at,
       CAST(EXISTS (
           SELECT 1 FROM voice_profiles p
           WHERE p.voice_id = v.id AND p.user_id = v.user_id AND p.current_version > 0
       ) AS INTEGER) AS made,
       CAST((
           SELECT count(*) FROM voice_samples s
           WHERE s.voice_id = v.id AND s.user_id = v.user_id
       ) AS INTEGER) AS sample_count,
       CAST(coalesce((
           SELECT pv.created_at FROM voice_profiles p
           JOIN voice_profile_versions pv
             ON pv.voice_id = p.voice_id AND pv.user_id = p.user_id AND pv.version = p.current_version
           WHERE p.voice_id = v.id AND p.user_id = v.user_id
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

-- name: InsertEmptyProfile :exec
-- Written with the directory row so a read never has to create a profile. It is the ONLY
-- insert path for this table now that both free-text editors are gone.
INSERT INTO voice_profiles (voice_id, user_id, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(voice_id) DO UPDATE SET updated_at = excluded.updated_at;

-- name: GetProfile :one
SELECT voice_id, user_id, current_version, corpus_version, updated_at
FROM voice_profiles
WHERE voice_id = ? AND user_id = ?;

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

-- name: InsertProfileVersion :exec
INSERT INTO voice_profile_versions
    (id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: SetProfileHead :exec
INSERT INTO voice_profiles(voice_id, user_id, corpus_version, current_version, updated_at)
VALUES (?, ?, 0, ?, ?)
ON CONFLICT(voice_id) DO UPDATE SET current_version=excluded.current_version, updated_at=excluded.updated_at;

-- name: GetProfileVersion :one
SELECT id, user_id, voice_id, version, snapshot, origin, restored_from_version, created_at FROM voice_profile_versions WHERE voice_id=? AND user_id=? AND version=?;

-- name: ListProfileVersions :many
-- `has_sample` rather than the snapshot itself: the list must be able to say whether a version
-- can be previewed without carrying every post body it ever produced (VOICE-29).
SELECT v.id, v.user_id, v.voice_id, v.version, v.snapshot, v.origin, v.restored_from_version, v.created_at,
       s.version AS sample_version
FROM voice_profile_versions v
LEFT JOIN voice_version_samples s
       ON s.voice_id = v.voice_id AND s.user_id = v.user_id AND s.version = v.version
WHERE v.voice_id=? AND v.user_id=? ORDER BY v.version DESC;

-- name: UpsertVersionSample :exec
-- One snapshot per version: a later generation under the same head REPLACES it.
INSERT INTO voice_version_samples (voice_id, user_id, version, content, created_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(voice_id, version) DO UPDATE SET
    content = excluded.content,
    created_at = excluded.created_at;

-- name: GetVersionSample :one
SELECT voice_id, user_id, version, content, created_at
FROM voice_version_samples
WHERE voice_id = ? AND user_id = ? AND version = ?;

-- name: ListManualOverrides :many
SELECT voice_id, user_id, layer, field, value, updated_at FROM voice_manual_overrides WHERE voice_id=? AND user_id=? ORDER BY layer, field;

-- name: UpsertManualOverride :exec
INSERT INTO voice_manual_overrides(voice_id, user_id, layer, field, value, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(voice_id,layer,field) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at;

-- name: DeleteManualOverride :execrows
DELETE FROM voice_manual_overrides WHERE voice_id=? AND user_id=? AND layer=? AND field=?;
