-- name: GetTestPublication :one
SELECT test_id,winner_candidate_id,action,request_key,fingerprint,target_id,receipt FROM guideline_test_publications
WHERE user_id=? AND ((test_id=? AND winner_candidate_id=? AND action=?) OR request_key=?);

-- name: InsertTestPublication :exec
INSERT INTO guideline_test_publications(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,target_id,receipt,created_at)
VALUES(?,?,?,?,?,?,?,?,?);

-- name: ReadTestPublicationReceipt :one
SELECT receipt FROM guideline_test_publications WHERE user_id=? AND test_id=? AND winner_candidate_id=? AND action=?;
