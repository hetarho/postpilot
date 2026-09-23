-- Post measurements (QUAL-4): what one content revision of one post measures on its own,
-- M3 and M4. The row is keyed by the post and stamped with the revision and the measure
-- version it describes, so a row whose stamp differs is stale and is measured again on the
-- next read. Every statement names user_id.
--
-- Keep this file ASCII: sqlc slices the query text by byte offset, and a multi-byte character
-- anywhere in the file shifts every constant it generates from it.

-- name: GetPostMeasurement :one
SELECT user_id, content_revision, measure_version, char_count, photo_count, distinct_block_types,
       avg_sentence_length, repetition_share, top_noun, title_relevance, computed_at
FROM post_measurements
WHERE post_slug = ? AND user_id = ?;

-- name: UpsertPostMeasurement :exec
-- A newer revision replaces the row. The composite foreign key refuses an insert for another
-- account's post, and the WHERE keeps the update half from moving a row across accounts.
INSERT INTO post_measurements (post_slug, user_id, content_revision, measure_version, char_count,
    photo_count, distinct_block_types, avg_sentence_length, repetition_share, top_noun,
    title_relevance, computed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(post_slug) DO UPDATE SET
    content_revision = excluded.content_revision, measure_version = excluded.measure_version,
    char_count = excluded.char_count, photo_count = excluded.photo_count,
    distinct_block_types = excluded.distinct_block_types,
    avg_sentence_length = excluded.avg_sentence_length,
    repetition_share = excluded.repetition_share, top_noun = excluded.top_noun,
    title_relevance = excluded.title_relevance, computed_at = excluded.computed_at
WHERE post_measurements.user_id = excluded.user_id;
