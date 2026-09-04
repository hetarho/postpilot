-- Templates. The account owns the directory; every statement names user_id, so a
-- same-shaped id from another account reaches nothing rather than someone else's template.
--
-- posts is another context's table and is never joined here for its own sake: the reads
-- below touch only posts.template_id, the foreign key posts itself declares, which is what
-- the count and the detach are about (ARCHITECTURE section 2.2).

-- name: InsertTemplate :exec
INSERT INTO templates (id, user_id, name, description, body, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListTemplates :many
SELECT t.id, t.user_id, t.name, t.description, t.body, t.created_at, t.updated_at,
       (SELECT count(*) FROM posts po WHERE po.template_id = t.id AND po.user_id = t.user_id) AS post_count
FROM templates t
WHERE t.user_id = ?
ORDER BY t.name, t.id;

-- name: GetTemplate :one
SELECT t.id, t.user_id, t.name, t.description, t.body, t.created_at, t.updated_at,
       (SELECT count(*) FROM posts po WHERE po.template_id = t.id AND po.user_id = t.user_id) AS post_count
FROM templates t
WHERE t.id = ? AND t.user_id = ?;

-- name: CountTemplates :one
SELECT count(*) FROM templates WHERE user_id = ?;

-- An edit is one statement PER PRESENT FIELD, run together in one transaction, rather than
-- one statement that writes all three. A field the request did not send is then never named
-- by any statement at all, so two fields edited from two tabs cannot overwrite each other
-- and no read-modify-write can put a stale value back. (A single COALESCE statement would
-- say the same thing, but sqlc types a NOT NULL column's parameter as a plain string, which
-- leaves no way to express "absent".)

-- name: UpdateTemplateName :execrows
UPDATE templates SET name = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdateTemplateDescription :execrows
UPDATE templates SET description = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdateTemplateBody :execrows
UPDATE templates SET body = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: CountPostsForTemplate :one
SELECT count(*) FROM posts WHERE template_id = ? AND user_id = ?;

-- name: DeleteTemplate :execrows
-- The schema's BEFORE DELETE trigger detaches the posts that named it, in this statement's
-- transaction. The posts themselves are never touched from here.
DELETE FROM templates WHERE id = ? AND user_id = ?;
