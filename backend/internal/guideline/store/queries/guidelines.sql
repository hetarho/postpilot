-- ASCII only: sqlc slices these statements by byte offset but counts in runes, so a
-- multi-byte character anywhere above rotates every generated query text.
--
-- Writing guidelines. The account owns them; every statement names user_id, so a
-- same-shaped id from another account reaches nothing rather than someone else's rule.
--
-- templates is another context's table and is never joined here: the scope links carry only
-- ids, and the names shown on screen are projected through the template directory port
-- (ARCHITECTURE section 2.2). A field link carries the field's ASCII id, whose list lives in
-- code. The ordering below is the INJECTION order (GUIDE-14): the global group first, then the
-- template group, then the field group, each by creation time. So the list, the prompt, and
-- the experiment snapshot cannot disagree about what the writer sees first.

-- A guideline is of one kind for good, a post's or a clip's (GUIDE-2); the cap and the list count
-- within the kind (GUIDE-5).

-- name: InsertGuideline :exec
INSERT INTO guidelines (id, user_id, kind, text, scope, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CountGuidelines :one
SELECT count(*) FROM guidelines WHERE user_id = ? AND kind = ?;

-- name: ListGuidelines :many
SELECT id, user_id, kind, text, scope, created_at, updated_at
FROM guidelines
WHERE user_id = ? AND kind = ?
ORDER BY CASE scope WHEN 'global' THEN 0 WHEN 'templates' THEN 1 ELSE 2 END, created_at, id;

-- name: GetGuideline :one
SELECT id, user_id, kind, text, scope, created_at, updated_at
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

-- Field links come back in id order. Display order follows the product's list, which is the
-- client's to apply.

-- name: ListGuidelineFieldLinks :many
SELECT guideline_id, field
FROM guideline_fields
WHERE user_id = ?
ORDER BY guideline_id, field;

-- name: ListGuidelineFields :many
SELECT field
FROM guideline_fields
WHERE guideline_id = ? AND user_id = ?
ORDER BY field;

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

-- name: DeleteGuidelineFieldLinks :exec
DELETE FROM guideline_fields WHERE guideline_id = ? AND user_id = ?;

-- name: InsertGuidelineFieldLink :exec
INSERT INTO guideline_fields (guideline_id, field, user_id) VALUES (?, ?, ?);

-- A clip guideline's video templates: the same shape as a post guideline's templates, in a table
-- of their own because the composite key names another context's table.

-- name: ListGuidelineVideoTemplateLinks :many
SELECT guideline_id, video_template_id
FROM guideline_video_templates
WHERE user_id = ?
ORDER BY guideline_id, video_template_id;

-- name: ListGuidelineVideoTemplates :many
SELECT video_template_id
FROM guideline_video_templates
WHERE guideline_id = ? AND user_id = ?
ORDER BY video_template_id;

-- name: DeleteGuidelineVideoTemplates :exec
DELETE FROM guideline_video_templates WHERE guideline_id = ? AND user_id = ?;

-- name: InsertGuidelineVideoTemplateLink :exec
INSERT INTO guideline_video_templates (guideline_id, video_template_id, user_id) VALUES (?, ?, ?);

-- name: DeleteGuideline :execrows
-- The schema cascades this guideline's own scope links. No template row is ever touched.
DELETE FROM guidelines WHERE id = ? AND user_id = ?;

-- name: ListApplicableGuidelineTexts :many
-- The texts that apply to one post, in injection order. An empty template id is a post with
-- no template and an empty field a post with no field: each matches no link row, so the
-- result holds only the groups the post has.
SELECT g.text
FROM guidelines g
WHERE g.user_id = ?
  AND g.kind = 'post'
  AND (
    g.scope = 'global'
    OR EXISTS (
      SELECT 1 FROM guideline_templates gt
      WHERE gt.guideline_id = g.id AND gt.user_id = g.user_id AND gt.template_id = ?
    )
    OR EXISTS (
      SELECT 1 FROM guideline_fields gf
      WHERE gf.guideline_id = g.id AND gf.user_id = g.user_id AND gf.field = ?
    )
  )
ORDER BY CASE g.scope WHEN 'global' THEN 0 WHEN 'templates' THEN 1 ELSE 2 END, g.created_at, g.id;

-- name: ListApplicableClipGuidelineTexts :many
-- The clip texts that apply to one clip: the global ones, then those linked to its video
-- template, each by creation time. An empty template id is a clip with none.
SELECT g.text
FROM guidelines g
WHERE g.user_id = ?
  AND g.kind = 'clip'
  AND (
    g.scope = 'global'
    OR EXISTS (
      SELECT 1 FROM guideline_video_templates gv
      WHERE gv.guideline_id = g.id AND gv.user_id = g.user_id AND gv.video_template_id = ?
    )
  )
ORDER BY CASE g.scope WHEN 'global' THEN 0 ELSE 1 END, g.created_at, g.id;

-- Guideline candidates (GUIDE-7). A candidate is one completed revision's instruction,
-- recorded verbatim. Rows in every state are kept: 'approved' and 'dismissed' rows are what
-- stop the same instruction from being recorded again, so nothing here deletes one.

-- A candidate is of one kind like the guideline it may become, and the dedupe, the guideline
-- check and the pending bound all count within it (GUIDE-10).

-- name: CandidateByText :one
SELECT id, user_id, kind, text, post_slug, clip_id, status, occurrences, first_seen_at, last_seen_at
FROM guideline_candidates
WHERE user_id = ? AND kind = ? AND text = ?;

-- name: GuidelineByText :one
SELECT id FROM guidelines WHERE user_id = ? AND kind = ? AND text = ?;

-- name: CountPendingCandidates :one
SELECT count(*) FROM guideline_candidates WHERE user_id = ? AND kind = ? AND status = 'pending';

-- name: InsertCandidate :exec
INSERT INTO guideline_candidates (id, user_id, kind, text, post_slug, clip_id, status, occurrences, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?);

-- name: BumpCandidate :execrows
-- A repeat. post_slug is deliberately NOT rewritten: the candidate names where it was first
-- seen, and rewriting it would make the link jump between posts on every repeat.
UPDATE guideline_candidates
SET occurrences = occurrences + 1, last_seen_at = ?
WHERE id = ? AND user_id = ?;

-- name: ListPendingCandidates :many
-- Review order: the most-repeated correction first, then the most recent. Exactly the order
-- idx_guideline_candidates_review serves.
SELECT id, user_id, kind, text, post_slug, clip_id, status, occurrences, first_seen_at, last_seen_at
FROM guideline_candidates
WHERE user_id = ? AND kind = ? AND status = 'pending'
ORDER BY occurrences DESC, last_seen_at DESC, id;

-- name: SetCandidateStatus :execrows
-- Only a PENDING row moves. A candidate the user already ruled on is terminal: without this
-- guard a stale tab could approve one twice, or dismiss one that was already approved.
UPDATE guideline_candidates
SET status = ?
WHERE id = ? AND user_id = ? AND status = 'pending';

-- name: ApproveCandidate :execrows
-- Approval by id, which only a candidate of the created guideline's kind can take (GUIDE-11).
UPDATE guideline_candidates
SET status = 'approved'
WHERE id = ? AND user_id = ? AND kind = ? AND status = 'pending';

-- name: SetCandidateStatusByText :exec
-- Approval by text, which is what marks the candidate a just-completed revision recorded
-- without the client having to learn its id. Only a pending row of the kind is moved: an
-- already dismissed one stays dismissed.
UPDATE guideline_candidates
SET status = ?
WHERE user_id = ? AND kind = ? AND text = ? AND status = 'pending';

-- name: DropCandidatePostSlug :exec
-- Post deletion drops the link and keeps the text: nothing references a candidate's origin.
UPDATE guideline_candidates SET post_slug = NULL WHERE user_id = ? AND post_slug = ?;

-- name: DropCandidateClipID :exec
-- A clip project's deletion drops its link the same way (GUIDE-13).
UPDATE guideline_candidates SET clip_id = NULL WHERE user_id = ? AND clip_id = ?;

-- name: ListDefaultsOff :many
-- The defaults an account switched off for one kind. A key the product no longer carries may
-- still be here; the service ignores it.
SELECT default_key FROM guideline_defaults_off WHERE user_id = ? AND kind = ? ORDER BY default_key;

-- name: InsertDefaultOff :exec
-- Switching off twice is the same as once.
INSERT INTO guideline_defaults_off (user_id, kind, default_key, created_at) VALUES (?, ?, ?, ?)
ON CONFLICT (user_id, kind, default_key) DO NOTHING;

-- name: DeleteDefaultOff :exec
-- Switching on is removing the off row; one that is already on stays on.
DELETE FROM guideline_defaults_off WHERE user_id = ? AND kind = ? AND default_key = ?;
