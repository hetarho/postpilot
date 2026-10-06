-- name: GetCandidateAdoption :one
SELECT voice_id FROM writing_voice_candidate_adoptions
WHERE user_id=? AND job_id=? AND candidate_id=?;

-- name: InsertCandidateAdoption :exec
INSERT INTO writing_voice_candidate_adoptions(user_id,job_id,candidate_id,voice_id,created_at)
VALUES(?,?,?,?,?);

-- name: LockCandidateAdoption :exec
UPDATE voices SET name=name WHERE 0;
