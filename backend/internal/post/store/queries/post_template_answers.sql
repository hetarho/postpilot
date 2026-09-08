-- The answers a post gives to its template's data fields (POST-62). Keyed by the label the
-- answer was typed under, so nothing here joins a template.
--
-- Both queries are scoped by the SLUG alone, like the images and videos of the same post: the
-- answers belong to the post aggregate, and the service establishes ownership by loading the
-- owned post before it writes. (An ownership-scoped upsert would have to be an
-- INSERT/SELECT/ON CONFLICT, which sqlc's SQLite parser cannot read.)
--
-- Keep this file ASCII: sqlc slices the query text by byte offset, and a multi-byte character
-- anywhere in the file shifts every constant it generates from it.

-- Upsert, never delete: clearing an answer is an empty `answer`, which the freeze reads the
-- same way it reads a switched-off field.
-- name: UpsertPostTemplateAnswer :exec
INSERT INTO post_template_answers (post_slug, label, answer, enabled, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (post_slug, label) DO UPDATE SET
    answer = excluded.answer, enabled = excluded.enabled, updated_at = excluded.updated_at;

-- Ordered by label so a read is stable; the write screen renders them in the TEMPLATE's body
-- order, which is the template's business and not this table's.
-- name: ListPostTemplateAnswers :many
SELECT label, answer, enabled FROM post_template_answers
WHERE post_slug = ? ORDER BY label;
