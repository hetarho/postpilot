-- ASCII only: sqlc slices these statements by byte offset but counts in runes, so a
-- multi-byte character anywhere above rotates every generated query text.
--
-- Writing guidelines. The account owns them; every statement names user_id, so a
-- same-shaped id from another account reaches nothing rather than someone else's rule.
--
-- templates is another context's table and is never joined here: the scope links carry only
-- ids, and the names shown on screen are projected through the template directory port
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

-- name: ListGuidelineTemplateLinks :many
SELECT guideline_id, template_id
FROM guideline_templates
WHERE user_id = ?
ORDER BY guideline_id, template_id;

-- name: ListGuidelineScope :many
SELECT template_id
FROM guideline_templates
WHERE guideline_id = ? AND user_id = ?
ORDER BY template_id;

-- A text edit and a scope replacement are separate statements run in one transaction, so an
-- edit that carries only one of them never names the other at all, and two tabs editing the two
-- halves cannot overwrite each other, and no read-modify-write can put a stale value back.

-- name: UpdateGuidelineText :execrows
UPDATE guidelines SET text = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdateGuidelineScope :execrows
UPDATE guidelines SET scope = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: DeleteGuidelineScope :exec
DELETE FROM guideline_templates WHERE guideline_id = ? AND user_id = ?;

-- name: InsertGuidelineScopeLink :exec
INSERT INTO guideline_templates (guideline_id, template_id, user_id) VALUES (?, ?, ?);

-- name: DeleteGuideline :execrows
-- The schema cascades this guideline's own scope links. No template row is ever touched.
DELETE FROM guidelines WHERE id = ? AND user_id = ?;

-- name: ListApplicableGuidelineTexts :many
-- The texts that apply to one post, in injection order. An empty template id is a post with
-- no template: it matches no link row, so the result is the global group alone.
SELECT g.text
FROM guidelines g
WHERE g.user_id = ?
  AND (
    g.scope = 'global'
    OR EXISTS (
      SELECT 1 FROM guideline_templates gt
      WHERE gt.guideline_id = g.id AND gt.user_id = g.user_id AND gt.template_id = ?
    )
  )
ORDER BY CASE g.scope WHEN 'global' THEN 0 ELSE 1 END, g.created_at, g.id;

-- Guideline candidates (change 26). A candidate is one completed revision's instruction,
-- recorded verbatim. Rows in every state are kept: 'approved' and 'dismissed' rows are what
-- stop the same instruction from being recorded again, so nothing here deletes one.

-- name: CandidateByText :one
SELECT id, user_id, text, post_slug, status, occurrences, first_seen_at, last_seen_at
FROM guideline_candidates
WHERE user_id = ? AND text = ?;

-- name: GuidelineByText :one
SELECT id FROM guidelines WHERE user_id = ? AND text = ?;

-- name: CountPendingCandidates :one
SELECT count(*) FROM guideline_candidates WHERE user_id = ? AND status = 'pending';

-- name: InsertCandidate :exec
INSERT INTO guideline_candidates (id, user_id, text, post_slug, status, occurrences, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, 1, ?, ?);

-- name: BumpCandidate :execrows
-- A repeat. post_slug is deliberately NOT rewritten: the candidate names where it was first
-- seen, and rewriting it would make the link jump between posts on every repeat.
UPDATE guideline_candidates
SET occurrences = occurrences + 1, last_seen_at = ?
WHERE id = ? AND user_id = ?;

-- name: ListPendingCandidates :many
-- Review order: the most-repeated correction first, then the most recent. Exactly the order
-- idx_guideline_candidates_review serves.
SELECT id, user_id, text, post_slug, status, occurrences, first_seen_at, last_seen_at
FROM guideline_candidates
WHERE user_id = ? AND status = 'pending'
ORDER BY occurrences DESC, last_seen_at DESC, id;

-- name: SetCandidateStatus :execrows
-- Only a PENDING row moves. A candidate the user already ruled on is terminal: without this
-- guard a stale tab could approve one twice, or dismiss one that was already approved.
UPDATE guideline_candidates
SET status = ?
WHERE id = ? AND user_id = ? AND status = 'pending';

-- name: SetCandidateStatusByText :exec
-- Approval by text, which is what marks the candidate a just-completed revision recorded
-- without the client having to learn its id. Only a pending row is moved: an already
-- dismissed one stays dismissed.
UPDATE guideline_candidates
SET status = ?
WHERE user_id = ? AND text = ? AND status = 'pending';

-- name: DropCandidatePostSlug :exec
-- Post deletion drops the link and keeps the text: nothing references a candidate's origin.
UPDATE guideline_candidates SET post_slug = NULL WHERE user_id = ? AND post_slug = ?;
