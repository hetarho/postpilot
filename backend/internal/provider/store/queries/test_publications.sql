-- name: GetTestModelPublication :one
SELECT test_id,winner_candidate_id,request_key,fingerprint,stage,model_ref,receipt
FROM provider_test_publications
WHERE user_id=? AND ((test_id=? AND winner_candidate_id=? AND action='adopt_model') OR request_key=?);

-- name: LockTestModelPublication :exec
UPDATE model_selections SET updated_at=updated_at WHERE 0;

-- name: InsertTestModelPublication :exec
INSERT INTO provider_test_publications(user_id,test_id,winner_candidate_id,action,request_key,fingerprint,stage,model_ref,receipt,created_at)
VALUES(?,?,?,'adopt_model',?,?,?,?,?,?);
