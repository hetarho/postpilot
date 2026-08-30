-- Posts. Every query is scoped by user_id: ownership is enforced in SQL, not by a
-- caller remembering to check it.

-- name: CreatePost :exec
INSERT INTO posts (slug, user_id, voice_id, title, memo, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 'draft', ?, ?);

-- name: UpdatePostDraft :execrows
UPDATE posts SET title = ?, memo = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: UpdatePostObservations :execrows
UPDATE posts SET observations = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: UpdateGeneratedContent :execrows
UPDATE posts SET content = ?, machine_baseline = ?, machine_baseline_voice_id = voice_id,
    content_revision = content_revision + 1,
    machine_baseline_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = ?
WHERE slug = ? AND user_id = ?
  AND (content IS NULL OR content <> ? OR status <> 'review'
       OR machine_baseline_revision <> content_revision);

-- name: SavePostContent :execrows
UPDATE posts SET content = ?, content_revision = content_revision + 1,
    status = 'review', finalized_revision = NULL, finalized_at = NULL, updated_at = ?
WHERE slug = ? AND user_id = ? AND content_revision = ?;

-- name: SavePostGenerationOptions :execrows
UPDATE posts SET target_length = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: FinalizePost :execrows
UPDATE posts SET status = 'finalized', finalized_revision = content_revision,
    finalized_at = ?, updated_at = ?
WHERE slug = ? AND user_id = ? AND content_revision = ?
  AND content IS NOT NULL;

-- name: GetPost :one
SELECT slug, user_id, voice_id, title, memo, observations, content, status, created_at, updated_at,
       content_revision, machine_baseline, machine_baseline_revision, machine_baseline_voice_id,
       target_length, finalized_revision, finalized_at
FROM posts WHERE slug = ?;

-- name: GetLearningSnapshot :one
SELECT slug, user_id, voice_id, content, content_revision, machine_baseline, machine_baseline_revision,
       machine_baseline_voice_id, target_length, status, finalized_revision, finalized_at, updated_at
FROM posts WHERE slug = ? AND user_id = ?;

-- name: PostSlugExists :one
SELECT EXISTS (SELECT 1 FROM posts WHERE slug = ?);

-- name: ListPostsByUser :many
SELECT slug, title, content, status, updated_at, voice_id
FROM posts WHERE user_id = ? ORDER BY updated_at DESC, slug DESC;

-- name: ReassignPostVoice :execrows
-- The reassignment keeps every byte of the post and drops only what belonged to the old
-- voice: the machine baseline, and with it the eligibility to learn from this post until a
-- fresh machine result is written under the new voice.
UPDATE posts SET voice_id = ?, machine_baseline = NULL, machine_baseline_revision = 0,
    machine_baseline_voice_id = NULL,
    updated_at = ?
WHERE slug = ? AND user_id = ? AND voice_id <> ?;

-- name: CountPostsByVoice :one
SELECT count(*) FROM posts WHERE voice_id = ? AND user_id = ?;

-- name: DeletePost :execrows
DELETE FROM posts WHERE slug = ? AND user_id = ?;
