-- Posts. Every query is scoped by user_id: ownership is enforced in SQL, not by a
-- caller remembering to check it.

-- name: CreatePost :exec
INSERT INTO posts (slug, user_id, title, memo, status, created_at, updated_at)
VALUES (?, ?, ?, ?, 'draft', ?, ?);

-- name: UpdatePostDraft :execrows
UPDATE posts SET title = ?, memo = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: UpdatePostObservations :execrows
UPDATE posts SET observations = ?, updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: UpdateGeneratedContent :execrows
UPDATE posts SET content = ?, machine_baseline = ?,
    content_revision = content_revision + 1,
    machine_baseline_revision = content_revision + 1,
    status = 'review', updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: UpdateGeneratedContentWithTarget :execrows
UPDATE posts SET content = ?, machine_baseline = ?, target_length = ?,
    content_revision = content_revision + 1,
    machine_baseline_revision = content_revision + 1,
    status = 'review', updated_at = ?
WHERE slug = ? AND user_id = ?;

-- name: SavePostContent :execrows
UPDATE posts SET content = ?, content_revision = content_revision + 1, target_length = ?, updated_at = ?
WHERE slug = ? AND user_id = ? AND content_revision = ?;

-- name: GetPost :one
SELECT slug, user_id, title, memo, observations, content, status, created_at, updated_at,
       content_revision, machine_baseline, machine_baseline_revision, target_length
FROM posts WHERE slug = ?;

-- name: GetLearningSnapshot :one
SELECT slug, user_id, content, content_revision, machine_baseline, machine_baseline_revision,
       target_length, updated_at
FROM posts WHERE slug = ? AND user_id = ?;

-- name: PostSlugExists :one
SELECT EXISTS (SELECT 1 FROM posts WHERE slug = ?);

-- name: ListPostsByUser :many
SELECT slug, title, content, status, updated_at
FROM posts WHERE user_id = ? ORDER BY updated_at DESC, slug DESC;

-- name: DeletePost :execrows
DELETE FROM posts WHERE slug = ? AND user_id = ?;
