-- name: InsertWritingTestPhoto :exec
INSERT INTO writing_test_photos(id,user_id,filename,object_key,status,expires_at,created_at)
VALUES(?,?,?,?,'pending',?,?);
-- name: GetWritingTestPhoto :one
SELECT * FROM writing_test_photos WHERE user_id=? AND id=?;
-- name: ConfirmWritingTestPhoto :execrows
UPDATE writing_test_photos SET status='ready',bytes=?,width=?,height=? WHERE user_id=? AND id=? AND status='pending';
-- name: ListReadyWritingTestPhotos :many
SELECT * FROM writing_test_photos
WHERE user_id=sqlc.arg(user_id) AND status='ready' AND id IN (SELECT value FROM json_each(sqlc.arg(ids)))
ORDER BY created_at,id;
-- name: AttachWritingTestPhotos :execrows
UPDATE writing_test_photos SET test_id=sqlc.arg(test_id)
WHERE user_id=sqlc.arg(user_id) AND id IN (SELECT value FROM json_each(sqlc.arg(ids))) AND status='ready' AND test_id IS NULL;
-- name: CountUnattachedWritingTestPhotos :one
SELECT COUNT(*) FROM writing_test_photos WHERE user_id=? AND test_id IS NULL;
-- name: ListUnattachedWritingTestPhotosExpired :many
SELECT * FROM writing_test_photos WHERE test_id IS NULL AND expires_at<=? ORDER BY expires_at,id LIMIT ?;
-- name: DeleteUnattachedWritingTestPhoto :execrows
DELETE FROM writing_test_photos WHERE user_id=? AND id=? AND test_id IS NULL;
-- name: DeleteWritingTestPhotosForTest :exec
DELETE FROM writing_test_photos WHERE user_id=? AND test_id=?;
-- name: ListWritingTestPhotoKeys :many
SELECT object_key FROM writing_test_photos;
-- name: ListWritingTestPhotosForTest :many
SELECT * FROM writing_test_photos WHERE user_id=? AND test_id=? ORDER BY created_at,id;
