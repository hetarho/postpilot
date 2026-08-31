-- ASCII only: sqlc slices these statements by byte offset but counts in runes, so a
-- multi-byte character anywhere above rotates every generated query text.
--
-- Writing guidelines. The account owns them; every statement names user_id, so a
-- same-shaped id from another account reaches nothing rather than someone else's rule.
--
-- purposes is another context's table and is never joined here: the scope links carry only
-- ids, and the names shown on screen are projected through the purpose directory port
-- (ARCHITECTURE section 2.2). The ordering below is the INJECTION order: the global group
-- first, then the scoped group, each by creation time. So the list, the prompt, and the
-- experiment snapshot cannot disagree about what the writer sees first.

-- name: InsertGuideline :exec
INSERT INTO guidelines (id, user_id, text, scope, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: CountGuidelines :one
SELECT count(*) FROM guidelines WHERE user_id = ?;

-- name: ListGuidelines :many
SELECT id, user_id, text, scope, created_at, updated_at
FROM guidelines
WHERE user_id = ?
ORDER BY CASE scope WHEN 'global' THEN 0 ELSE 1 END, created_at, id;

-- name: GetGuideline :one
SELECT id, user_id, text, scope, created_at, updated_at
FROM guidelines
WHERE id = ? AND user_id = ?;

-- name: ListGuidelinePurposeLinks :many
SELECT guideline_id, purpose_id
FROM guideline_purposes
WHERE user_id = ?
ORDER BY guideline_id, purpose_id;

-- name: ListGuidelineScope :many
SELECT purpose_id
FROM guideline_purposes
WHERE guideline_id = ? AND user_id = ?
ORDER BY purpose_id;

-- A text edit and a scope replacement are separate statements run in one transaction, so an
-- edit that carries only one of them never names the other at all, and two tabs editing the two
-- halves cannot overwrite each other, and no read-modify-write can put a stale value back.

-- name: UpdateGuidelineText :execrows
UPDATE guidelines SET text = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdateGuidelineScope :execrows
UPDATE guidelines SET scope = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: DeleteGuidelineScope :exec
DELETE FROM guideline_purposes WHERE guideline_id = ? AND user_id = ?;

-- name: InsertGuidelineScopeLink :exec
INSERT INTO guideline_purposes (guideline_id, purpose_id, user_id) VALUES (?, ?, ?);

-- name: DeleteGuideline :execrows
-- The schema cascades this guideline's own scope links. No purpose row is ever touched.
DELETE FROM guidelines WHERE id = ? AND user_id = ?;

-- name: ListApplicableGuidelineTexts :many
-- The texts that apply to one post, in injection order. An empty purpose id is a post with
-- no purpose: it matches no link row, so the result is the global group alone.
SELECT g.text
FROM guidelines g
WHERE g.user_id = ?
  AND (
    g.scope = 'global'
    OR EXISTS (
      SELECT 1 FROM guideline_purposes gp
      WHERE gp.guideline_id = g.id AND gp.user_id = g.user_id AND gp.purpose_id = ?
    )
  )
ORDER BY CASE g.scope WHEN 'global' THEN 0 ELSE 1 END, g.created_at, g.id;
