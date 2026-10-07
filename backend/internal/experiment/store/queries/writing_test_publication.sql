-- name: LockWritingTestPublication :exec
UPDATE writing_test_publications SET status=status WHERE 0;
-- name: FindWritingTestPublication :many
SELECT * FROM writing_test_publications WHERE user_id=? AND ((test_id=? AND winner_candidate_id=? AND action=?) OR request_key=?);
-- name: InsertWritingTestPublication :exec
INSERT INTO writing_test_publications(id,user_id,test_id,winner_candidate_id,action,request_key,fingerprint,target_id,status,created_at)
VALUES(?,?,?,?,?,?,?,'','pending',?);
-- name: ConfirmWritingTestPublication :execrows
UPDATE writing_test_publications SET target_id=?,status='confirmed',confirmed_at=?
WHERE user_id=? AND test_id=? AND id=? AND status='pending';
