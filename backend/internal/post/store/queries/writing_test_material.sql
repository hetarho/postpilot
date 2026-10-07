-- name: WritingTestOwnedImages :many
SELECT i.id,i.post_slug,i.filename,i.r2_key,i.width,i.height,i.bytes,i.created_at,i.rotation,i.rotation_by_owner
FROM images AS i JOIN posts AS p ON p.slug=i.post_slug
WHERE p.user_id=? AND i.id IN (SELECT value FROM json_each(?)) ORDER BY i.created_at,i.id;
-- name: WritingTestOwnedVideos :many
SELECT v.id,v.post_slug,v.filename,v.r2_key,v.content_type,v.bytes,v.duration_ms,v.width,v.height,v.created_at
FROM videos AS v JOIN posts AS p ON p.slug=v.post_slug
WHERE p.user_id=? AND v.id IN (SELECT value FROM json_each(?)) ORDER BY v.created_at,v.id;
