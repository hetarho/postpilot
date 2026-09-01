-- Posts. Every query is scoped by user_id: ownership is enforced in SQL, not by a
-- caller remembering to check it.

-- name: CreatePost :exec
INSERT INTO posts (slug, user_id, voice_id, purpose_id, title, memo, target_language, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?);

-- name: UpdatePostDraft :execrows
UPDATE posts SET title = sqlc.arg(title), memo = sqlc.arg(memo),
    target_language = COALESCE(sqlc.narg(target_language), target_language),
    updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id);

-- name: UpdatePostObservations :execrows
UPDATE posts SET observations = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: UpdateGeneratedContent :execrows
UPDATE posts SET content = sqlc.arg(content), machine_baseline = sqlc.arg(machine_baseline), machine_baseline_voice_id = voice_id,
    content_language = sqlc.arg(content_language),
    content_revision = content_revision + 1,
    machine_baseline_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = sqlc.arg(updated_at)
WHERE slug = sqlc.arg(slug) AND user_id = sqlc.arg(user_id)
  AND (content IS NULL OR content <> sqlc.arg(content) OR status <> 'review'
       OR machine_baseline_revision <> content_revision
       OR content_language IS NULL OR content_language <> sqlc.arg(content_language));

-- name: SavePostContent :execrows
UPDATE posts SET content = ?, content_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = ?
WHERE slug = ? AND user_id = ? AND content_revision = ?;

-- name: SavePostGenerationOptions :execrows
UPDATE posts SET target_length = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- Finalizing also copies the confirmed AI title into posts.title (spec/policy/posts.md). ONE
-- statement, still guarded by the exact revision, so the copy is atomic with the finalization and
-- a concurrent content save simply matches zero rows. The caller resolves which title to write: an
-- empty content title leaves the user's working title in place. The slug is never re-minted.
-- name: FinalizePost :execrows
UPDATE posts SET status = 'finalized', finalized_revision = content_revision,
    title = ?, finalized_at = ?, updated_at = ?
WHERE slug = ? AND user_id = ? AND content_revision = ?
  AND content IS NOT NULL;

-- name: GetPost :one
SELECT slug, user_id, voice_id, title, memo, observations, content, status, created_at, updated_at,
       content_revision, machine_baseline, machine_baseline_revision, machine_baseline_voice_id,
       target_length, finalized_revision, finalized_at, purpose_id, target_language, content_language
FROM posts WHERE slug = ?;

-- name: GetLearningSnapshot :one
SELECT slug, user_id, voice_id, content, content_revision, machine_baseline, machine_baseline_revision,
       machine_baseline_voice_id, target_length, status, finalized_revision, finalized_at, updated_at,
       target_language, content_language
FROM posts WHERE slug = ? AND user_id = ?;

-- name: PostSlugExists :one
SELECT EXISTS (SELECT 1 FROM posts WHERE slug = ?);

-- name: ListPostsByUser :many
SELECT slug, title, content, status, updated_at, voice_id, purpose_id, target_language, content_language
FROM posts WHERE user_id = ? ORDER BY updated_at DESC, slug DESC;

-- name: ReassignPostVoice :execrows
-- The reassignment keeps every byte of the post and drops only what belonged to the old
-- voice: the machine baseline, and with it the eligibility to learn from this post until a
-- fresh machine result is written under the new voice.
UPDATE posts SET voice_id = ?, machine_baseline = NULL, machine_baseline_revision = 0,
    machine_baseline_voice_id = NULL,
    updated_at = ?
WHERE slug = ? AND user_id = ? AND voice_id <> ?;

-- name: AssignPostPurpose :execrows
-- Assignment is not a reassignment: unlike the voice, a purpose is never learned from, so
-- this touches no content, revision, machine baseline or finalization column and is allowed
-- in every status. NULL is the clear.
UPDATE posts SET purpose_id = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: CountPostsByVoice :one
SELECT count(*) FROM posts WHERE voice_id = ? AND user_id = ?;

-- name: DeletePost :execrows
DELETE FROM posts WHERE slug = ? AND user_id = ?;
