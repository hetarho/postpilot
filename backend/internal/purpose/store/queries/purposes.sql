-- Purposes. The account owns the directory; every statement names user_id, so a
-- same-shaped id from another account reaches nothing rather than someone else's brief.
--
-- posts is another context's table and is never joined here for its own sake: the reads
-- below touch only posts.purpose_id, the foreign key posts itself declares, which is what
-- the count and the detach are about (ARCHITECTURE section 2.2).

-- name: InsertPurpose :exec
INSERT INTO purposes (id, user_id, name, description, instructions, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListPurposes :many
SELECT p.id, p.user_id, p.name, p.description, p.instructions, p.created_at, p.updated_at,
       (SELECT count(*) FROM posts po WHERE po.purpose_id = p.id AND po.user_id = p.user_id) AS post_count
FROM purposes p
WHERE p.user_id = ?
ORDER BY p.name, p.id;

-- name: GetPurpose :one
SELECT p.id, p.user_id, p.name, p.description, p.instructions, p.created_at, p.updated_at,
       (SELECT count(*) FROM posts po WHERE po.purpose_id = p.id AND po.user_id = p.user_id) AS post_count
FROM purposes p
WHERE p.id = ? AND p.user_id = ?;

-- An edit is one statement PER PRESENT FIELD, run together in one transaction, rather than
-- one statement that writes all three. A field the request did not send is then never named
-- by any statement at all, so two fields edited from two tabs cannot overwrite each other
-- and no read-modify-write can put a stale value back. (A single COALESCE statement would
-- say the same thing, but sqlc types a NOT NULL column's parameter as a plain string, which
-- leaves no way to express "absent".)

-- name: UpdatePurposeName :execrows
UPDATE purposes SET name = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdatePurposeDescription :execrows
UPDATE purposes SET description = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdatePurposeInstructions :execrows
UPDATE purposes SET instructions = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: CountPostsForPurpose :one
SELECT count(*) FROM posts WHERE purpose_id = ? AND user_id = ?;

-- name: DeletePurpose :execrows
-- The schema's BEFORE DELETE trigger detaches the posts that named it, in this statement's
-- transaction. The posts themselves are never touched from here.
DELETE FROM purposes WHERE id = ? AND user_id = ?;
