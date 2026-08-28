-- name: UpsertProfile :exec
INSERT INTO voice_profiles (user_id, styleguide, rules, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    styleguide = excluded.styleguide,
    rules = excluded.rules,
    updated_at = excluded.updated_at;

-- name: GetProfile :one
SELECT user_id, styleguide, rules, updated_at
FROM voice_profiles
WHERE user_id = ?;

-- name: SetStyleguideIfCorpusVersion :execrows
UPDATE voice_profiles
SET styleguide = ?, updated_at = ?
WHERE user_id = ? AND corpus_version = ?;

-- name: SetRules :exec
INSERT INTO voice_profiles (user_id, styleguide, rules, updated_at)
VALUES (?, '', ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
    rules = excluded.rules,
    updated_at = excluded.updated_at;

-- name: InsertSample :exec
INSERT INTO voice_samples (id, user_id, label, body, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: BumpCorpusVersion :exec
INSERT INTO voice_profiles (user_id, styleguide, rules, corpus_version, updated_at)
VALUES (?, '', '', 1, ?)
ON CONFLICT(user_id) DO UPDATE SET
    corpus_version = voice_profiles.corpus_version + 1,
    updated_at = excluded.updated_at;

-- name: GetCorpusVersion :one
SELECT corpus_version FROM voice_profiles WHERE user_id = ?;

-- name: ListSamples :many
SELECT id, label, length(body) AS chars, created_at
FROM voice_samples
WHERE user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: ListSampleBodies :many
SELECT id, label, body, created_at
FROM voice_samples
WHERE user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: GetSampleBody :one
SELECT id, label, body, created_at
FROM voice_samples
WHERE id = ? AND user_id = ?;

-- name: DeleteSample :execrows
DELETE FROM voice_samples WHERE id = ? AND user_id = ?;

-- name: CountSamples :one
SELECT count(*) FROM voice_samples WHERE user_id = ?;
