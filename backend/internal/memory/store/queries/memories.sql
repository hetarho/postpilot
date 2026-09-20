-- ASCII only: sqlc slices these statements by byte offset but counts in runes, so a
-- multi-byte character anywhere above rotates every generated query text.
--
-- Writing memories. The account owns them; every statement names user_id, so a same-shaped
-- id from another account reaches nothing rather than someone else's facts.
--
-- posts is another context's table and is never joined here: a source link carries the
-- post's slug and nothing else, and the post context tells this one when a post is gone
-- (ARCHITECTURE section 2.2). The list ordering is the INJECTION order: most recently used
-- first, then most recently created, so the management screen and the write prompt cannot
-- disagree about which fact the writer sees first.

-- name: InsertMemory :exec
INSERT INTO memories (id, user_id, text, kind, created_at, updated_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CountMemories :one
SELECT count(*) FROM memories WHERE user_id = ?;

-- name: MemoryByText :one
SELECT id, user_id, text, kind, created_at, updated_at, last_seen_at
FROM memories
WHERE user_id = ? AND text = ?;

-- name: ListMemories :many
SELECT id, user_id, text, kind, created_at, updated_at, last_seen_at
FROM memories
WHERE user_id = ?
ORDER BY last_seen_at DESC, created_at DESC, id;

-- name: GetMemory :one
SELECT id, user_id, text, kind, created_at, updated_at, last_seen_at
FROM memories
WHERE id = ? AND user_id = ?;

-- A re-approval advances last_seen_at and NOTHING else: updated_at belongs to an authored
-- edit, and retrieval breaks its ties on the former.

-- name: TouchMemory :execrows
UPDATE memories SET last_seen_at = ? WHERE id = ? AND user_id = ?;

-- A text edit, a kind edit and a tag replacement are separate statements run in one
-- transaction, so an edit that carries only one of them never names the others at all and
-- two tabs editing two halves cannot overwrite each other.

-- name: UpdateMemoryText :execrows
UPDATE memories SET text = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: UpdateMemoryKind :execrows
UPDATE memories SET kind = ?, updated_at = ? WHERE id = ? AND user_id = ?;

-- name: TouchMemoryUpdatedAt :execrows
UPDATE memories SET updated_at = ? WHERE id = ? AND user_id = ?;

-- name: DeleteMemory :execrows
-- The schema cascades this memory's own tags and source links. No post row is ever touched.
DELETE FROM memories WHERE id = ? AND user_id = ?;

-- name: ListMemoryTags :many
SELECT memory_id, tag
FROM memory_tags
WHERE user_id = ?
ORDER BY memory_id, rowid;

-- name: ListTagsOfMemory :many
SELECT tag FROM memory_tags WHERE memory_id = ? AND user_id = ? ORDER BY rowid;

-- name: InsertMemoryTag :exec
INSERT INTO memory_tags (memory_id, user_id, tag) VALUES (?, ?, ?);

-- name: DeleteMemoryTags :exec
DELETE FROM memory_tags WHERE memory_id = ? AND user_id = ?;

-- name: ListMemorySources :many
SELECT memory_id, post_slug
FROM memory_sources
WHERE user_id = ?
ORDER BY memory_id, created_at, post_slug;

-- name: ListSourcesOfMemory :many
SELECT post_slug
FROM memory_sources
WHERE memory_id = ? AND user_id = ?
ORDER BY created_at, post_slug;

-- name: InsertMemorySource :exec
-- Re-approving a fact from a post it is already linked to is not an error and not a second
-- link: the pair is the primary key, and the create's own last_seen_at bump is the record
-- that it was seen again.
INSERT OR IGNORE INTO memory_sources (memory_id, user_id, post_slug, created_at)
VALUES (?, ?, ?, ?);

-- The post-delete rule (MEM-17) in three statements over one transaction: read which
-- memories the post fed, drop the links, then delete exactly those that just lost their
-- last one. A memory written by hand has no links at all and is untouched by the third
-- statement, because it never appears in the first.

-- name: ListMemoryIDsForPost :many
SELECT memory_id FROM memory_sources WHERE user_id = ? AND post_slug = ?;

-- name: DeleteMemorySourcesForPost :exec
DELETE FROM memory_sources WHERE user_id = ? AND post_slug = ?;

-- name: DeleteMemoryIfUnsourced :execrows
DELETE FROM memories
WHERE memories.id = ? AND memories.user_id = ?
  AND NOT EXISTS (
    SELECT 1 FROM memory_sources AS remaining WHERE remaining.memory_id = memories.id
  );
