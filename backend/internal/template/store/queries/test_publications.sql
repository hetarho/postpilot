-- name: GetTestPublication :one
SELECT test_id,winner_candidate_id,action,request_key,fingerprint,target_id,receipt FROM template_test_publications
WHERE user_id=? AND ((test_id=? AND winner_candidate_id=? AND action=?) OR request_key=?);

-- name: InsertTestPublication :exec
INSERT INTO template_test_publications(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,target_id,receipt,created_at)
VALUES(?,?,?,?,?,?,?,?,?);
