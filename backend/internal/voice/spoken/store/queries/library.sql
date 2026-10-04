-- name: ListDrafts :many
SELECT * FROM spoken_voice_drafts WHERE owner_id = ? ORDER BY updated_at DESC, id;
-- name: GetDraft :one
SELECT * FROM spoken_voice_drafts WHERE id = ? AND owner_id = ?;
-- name: InsertDraft :exec
INSERT INTO spoken_voice_drafts (id, owner_id, revision, name, description, preview_text, profile_json, qualification_session_id, generation_id, selected_candidate_id, confirmed_voice_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: UpdateDraft :execrows
UPDATE spoken_voice_drafts SET name = sqlc.arg(name), description = sqlc.arg(description), preview_text = sqlc.arg(preview_text), profile_json = sqlc.arg(profile), qualification_session_id = sqlc.arg(qualification), generation_id = sqlc.arg(generation), selected_candidate_id = sqlc.arg(selected), revision = revision + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND owner_id = sqlc.arg(owner) AND revision = sqlc.arg(expected) AND confirmed_voice_id = '';
-- name: DeleteDraft :execrows
DELETE FROM spoken_voice_drafts WHERE id = ? AND owner_id = ? AND revision = ?;
-- name: ListCandidates :many
SELECT c.*, a.samples, a.sample_rate FROM spoken_voice_candidates c JOIN spoken_audio_assets a ON a.id = c.asset_id
WHERE c.draft_id = ? AND c.owner_id = ? AND c.generation_id = ? AND a.revoked_at IS NULL ORDER BY c.ordinal;
-- name: DeleteCandidates :exec
DELETE FROM spoken_voice_candidates WHERE draft_id = ? AND owner_id = ?;
-- name: InsertCandidate :exec
INSERT INTO spoken_voice_candidates (id, owner_id, draft_id, generation_id, supplier_handle, ordinal, asset_id) VALUES (?, ?, ?, ?, ?, ?, ?);
-- name: SetCandidatesReady :execrows
UPDATE spoken_voice_drafts SET generation_id = sqlc.arg(generation), selected_candidate_id = '', revision = revision + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND owner_id = sqlc.arg(owner) AND revision = sqlc.arg(expected) AND confirmed_voice_id = '';
-- name: SelectCandidate :execrows
UPDATE spoken_voice_drafts SET selected_candidate_id = sqlc.arg(candidate), revision = revision + 1, updated_at = sqlc.arg(at)
WHERE spoken_voice_drafts.id = sqlc.arg(draft) AND spoken_voice_drafts.owner_id = sqlc.arg(owner) AND revision = sqlc.arg(expected) AND confirmed_voice_id = ''
AND EXISTS (SELECT 1 FROM spoken_voice_candidates c WHERE c.id = sqlc.arg(candidate) AND c.owner_id = sqlc.arg(owner) AND c.draft_id = sqlc.arg(draft) AND c.generation_id = spoken_voice_drafts.generation_id);
-- name: MarkCandidateAuditioned :execrows
UPDATE spoken_voice_candidates SET auditioned_at = sqlc.arg(at)
WHERE spoken_voice_candidates.id = sqlc.arg(candidate) AND spoken_voice_candidates.owner_id = sqlc.arg(owner) AND spoken_voice_candidates.draft_id = sqlc.arg(draft)
AND EXISTS (SELECT 1 FROM spoken_audio_access p WHERE p.id = sqlc.arg(playback) AND p.owner_id = sqlc.arg(owner) AND p.asset_id = spoken_voice_candidates.asset_id AND p.served_at IS NOT NULL AND p.expires_at > sqlc.arg(at));
-- name: AdvanceDraftRevision :execrows
UPDATE spoken_voice_drafts SET revision = revision + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND owner_id = sqlc.arg(owner) AND revision = sqlc.arg(expected) AND confirmed_voice_id = '';

-- name: ListVoices :many
SELECT v.*, a.samples, a.sample_rate FROM spoken_voices v JOIN spoken_audio_assets a ON a.id = v.sample_asset_id
WHERE v.owner_id = ? AND (v.removed_at IS NULL OR sqlc.arg(include_removed) = 1) ORDER BY v.created_at DESC, v.id;
-- name: GetVoice :one
SELECT v.*, a.samples, a.sample_rate FROM spoken_voices v JOIN spoken_audio_assets a ON a.id = v.sample_asset_id WHERE v.id = ? AND v.owner_id = ?;
-- name: InsertVoice :exec
INSERT INTO spoken_voices (id, owner_id, revision, name, description, preview_text, profile_json, supplier_handle, draft_id, candidate_id, sample_asset_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: SetConfirmed :execrows
UPDATE spoken_voice_drafts SET confirmed_voice_id = sqlc.arg(voice), revision = revision + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(draft) AND owner_id = sqlc.arg(owner) AND revision = sqlc.arg(expected) AND selected_candidate_id = sqlc.arg(candidate) AND confirmed_voice_id = '';
-- name: RenameVoice :execrows
UPDATE spoken_voices SET name = ? , revision = revision + 1 WHERE id = ? AND owner_id = ? AND revision = ?;
-- name: RemoveVoice :execrows
UPDATE spoken_voices SET removed_at = ?, revision = revision + 1 WHERE id = ? AND owner_id = ? AND revision = ? AND removed_at IS NULL;
-- name: RecordVoiceUse :exec
INSERT INTO spoken_voice_uses (id, owner_id, voice_id, voice_revision, created_at) VALUES (?, ?, ?, ?, ?);
-- name: GetVoiceUse :one
SELECT owner_id, voice_id, voice_revision FROM spoken_voice_uses WHERE id = ?;

-- name: GetAsset :one
SELECT * FROM spoken_audio_assets WHERE id = ? AND owner_id = ?;
-- name: InsertAsset :exec
INSERT INTO spoken_audio_assets (id, owner_id, object_key, sha256, format, bytes, samples, sample_rate, channels, provenance_digest, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
-- name: RevokeAsset :execrows
UPDATE spoken_audio_assets SET revoked_at = ? WHERE id = ? AND owner_id = ? AND revoked_at IS NULL;
-- name: PrepareCleanup :exec
INSERT OR IGNORE INTO spoken_audio_cleanup (id, object_key, created_at) VALUES (?, ?, ?);
-- name: PendingCleanup :many
SELECT * FROM spoken_audio_cleanup WHERE created_at < ? ORDER BY created_at, id LIMIT 100;
-- name: CompleteCleanup :exec
DELETE FROM spoken_audio_cleanup WHERE id = ?;
-- name: DiscardRetainedCleanup :exec
DELETE FROM spoken_audio_cleanup WHERE spoken_audio_cleanup.id = sqlc.arg(id)
AND EXISTS (SELECT 1 FROM spoken_audio_assets WHERE spoken_audio_assets.id = sqlc.arg(id));
-- name: AssetRetained :one
SELECT COUNT(*) FROM spoken_audio_assets WHERE id = ?;
-- name: DeleteOwnerVoices :exec
DELETE FROM spoken_voices WHERE owner_id = ?;
-- name: DeleteOwnerDrafts :exec
DELETE FROM spoken_voice_drafts WHERE owner_id = ?;
-- name: DeleteOwnerAssets :exec
DELETE FROM spoken_audio_assets WHERE owner_id = ?;

-- name: GetRequest :one
SELECT input_digest, result_id, result_revision FROM spoken_voice_requests WHERE owner_id = ? AND operation = ? AND request_key = ?;
-- name: SaveRequest :exec
INSERT INTO spoken_voice_requests (owner_id, operation, request_key, input_digest, result_id, result_revision) VALUES (?, ?, ?, ?, ?, ?);

-- name: PurgePlayback :exec
DELETE FROM spoken_audio_access WHERE expires_at <= ?;
-- name: SavePlayback :exec
INSERT INTO spoken_audio_access (id, owner_id, asset_id, expires_at) VALUES (?, ?, ?, ?);
-- name: GetPlayback :one
SELECT * FROM spoken_audio_access WHERE id = ? AND owner_id = ? AND expires_at > ?;
-- name: MarkPlaybackServed :execrows
UPDATE spoken_audio_access SET served_at = ? WHERE id = ? AND owner_id = ? AND expires_at > ?;
