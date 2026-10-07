-- name: LockTestStylePublication :exec
UPDATE voices SET name=name WHERE 0;

-- name: FindTestStylePublication :many
SELECT request_key,fingerprint,target_id,receipt FROM voice_test_publications
WHERE user_id=? AND ((test_id=? AND winner_candidate_id=? AND action=?) OR request_key=?);

-- name: InsertTestStylePublication :exec
INSERT INTO voice_test_publications(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,target_id,receipt,created_at)
VALUES(?,?,?,?,?,?,?,?,?);

-- name: TestStyleSourcePresent :one
SELECT COUNT(*) FROM voice_samples WHERE id=? AND user_id=? AND voice_id=?;

-- name: ReadTestPublicationReceipt :one
SELECT receipt FROM voice_test_publications WHERE user_id=? AND test_id=? AND winner_candidate_id=? AND action=?;
