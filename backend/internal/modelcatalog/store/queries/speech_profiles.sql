-- name: ListSpeechProfiles :many
SELECT r.* FROM speech_profiles p
JOIN speech_profile_revisions r ON r.profile_id = p.id AND r.revision = p.current_revision
ORDER BY p.created_at, p.id;

-- name: GetSpeechRevision :one
SELECT * FROM speech_profile_revisions WHERE profile_id = ? AND revision = ?;

-- name: CreateSpeechProfile :exec
INSERT INTO speech_profiles(id, current_revision, created_at, updated_at) VALUES (?, ?, ?, ?);

-- name: AdvanceSpeechProfile :execrows
UPDATE speech_profiles SET current_revision = sqlc.arg(revision), updated_at = sqlc.arg(stamp)
WHERE id = sqlc.arg(id) AND current_revision = sqlc.arg(expected_revision);

-- name: InsertSpeechRevision :exec
INSERT INTO speech_profile_revisions(profile_id, revision, provider_id, design_model_id, speech_model_id, label, level, enabled, binding_json, prices_json, created_at, catalog_grade, tariff_revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ClaimSpeechCombination :execrows
INSERT OR IGNORE INTO speech_catalog_registrations(provider_id, design_model_id, speech_model_id, profile_id)
VALUES (?, ?, ?, ?);

-- name: GetSpeechCombination :one
SELECT profile_id FROM speech_catalog_registrations
WHERE provider_id = ? AND design_model_id = ? AND speech_model_id = ?;

-- name: GetSpeechTariff :one
SELECT t.* FROM speech_account_tariffs t JOIN speech_account_tariff_current c ON c.revision = t.revision WHERE c.id = 1;

-- name: InsertSpeechTariff :exec
INSERT INTO speech_account_tariffs(revision, connection_scope, design_usd_per_unit, speech_usd_per_unit, confirmation_usd, source, complete, checked_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateSpeechTariffPointer :exec
INSERT INTO speech_account_tariff_current(id, revision) VALUES (1, ?);

-- name: AdvanceSpeechTariffPointer :execrows
UPDATE speech_account_tariff_current SET revision = sqlc.arg(revision) WHERE id = 1 AND revision = sqlc.arg(expected_revision);

-- name: RecordSpeechVoiceReadiness :execrows
UPDATE speech_profile_revisions SET voice_evidence = sqlc.arg(evidence)
WHERE profile_id = sqlc.arg(profile_id) AND revision = sqlc.arg(revision)
AND EXISTS (SELECT 1 FROM speech_profiles WHERE id = sqlc.arg(profile_id) AND current_revision = sqlc.arg(revision));

-- name: RecordSpeechExportReadiness :execrows
UPDATE speech_profile_revisions SET export_evidence = sqlc.arg(evidence)
WHERE profile_id = sqlc.arg(profile_id) AND revision = sqlc.arg(revision) AND voice_evidence <> ''
AND EXISTS (SELECT 1 FROM speech_profiles WHERE id = sqlc.arg(profile_id) AND current_revision = sqlc.arg(revision));

-- name: CreateSpeechQualification :exec
INSERT INTO speech_qualification_sessions(id, owner_id, profile_id, revision, maximum_usd, expires_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetSpeechQualification :one
SELECT * FROM speech_qualification_sessions WHERE owner_id = ? AND id = ?;
