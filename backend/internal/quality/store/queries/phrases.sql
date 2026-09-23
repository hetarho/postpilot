-- The daily per-field phrase lists (QUAL-41). One row per field, written whole by the batch
-- and read whole by the write freeze. refreshed_at is NULL for a field whose first fetch
-- failed, and next_refresh_at is when the batch tries it again.
--
-- Keep this file ASCII: sqlc slices the query text by byte offset.

-- name: GetFieldPhraseList :one
SELECT field, phrases, corpus_size, refreshed_at, next_refresh_at
FROM field_phrase_lists
WHERE field = ?;

-- name: ReplaceFieldPhraseList :exec
-- The whole row, inserted when the field has none. Only the batch writes this table, so its
-- read-then-replace has no competing writer.
INSERT INTO field_phrase_lists (field, phrases, corpus_size, refreshed_at, next_refresh_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(field) DO UPDATE SET
    phrases = excluded.phrases, corpus_size = excluded.corpus_size,
    refreshed_at = excluded.refreshed_at, next_refresh_at = excluded.next_refresh_at;
