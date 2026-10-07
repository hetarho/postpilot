-- name: GetWritingTest :one
SELECT * FROM writing_tests WHERE user_id=? AND id=?;
-- name: GetWritingTestByRequest :one
SELECT * FROM writing_tests WHERE user_id=? AND operation_key=?;
-- name: ListWritingTests :many
SELECT * FROM writing_tests WHERE user_id=sqlc.arg(user_id)
AND (CAST(sqlc.arg(cursor_time) AS TEXT)='' OR created_at<sqlc.arg(cursor_time) OR (created_at=sqlc.arg(cursor_time) AND id<sqlc.arg(cursor_id)))
ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(page_limit);
-- name: InsertWritingTest :exec
INSERT INTO writing_tests(id,user_id,operation_key,fingerprint,kind,factor,model_stage,count,status,revision,source_post_slug,context,common_snapshot,common_hash,prompt_version,purge_fence,job_id,winner_candidate_id,confirmed_credits,reserved_credits,failure_reason,created_at,updated_at,content_expires_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: UpdateWritingTest :execrows
UPDATE writing_tests SET status=?,revision=?,context=?,purge_fence=?,job_id=?,winner_candidate_id=?,confirmed_credits=?,reserved_credits=?,failure_reason=?,updated_at=?,content_expires_at=?,common_snapshot=?,source_post_slug=?
WHERE user_id=? AND id=? AND revision=?;
-- name: ListWritingTestCandidates :many
SELECT * FROM writing_test_candidates WHERE user_id=? AND test_id=? ORDER BY seed_position,id;
-- name: InsertWritingTestCandidate :exec
INSERT INTO writing_test_candidates(id,user_id,test_id,seed_position,source_kind,source_id,source_revision,semantic_key,frozen_variant,status)
VALUES(?,?,?,?,?,?,?,?,?,?);
-- name: UpdateWritingTestCandidate :execrows
UPDATE writing_test_candidates SET status=?,output=?,accounting=?,failure_reason=?,started_at=COALESCE(?,started_at),finished_at=?
WHERE user_id=? AND test_id=? AND id=?;
-- name: PurgeWritingTestCandidate :exec
UPDATE writing_test_candidates SET frozen_variant=NULL,output=NULL WHERE user_id=? AND test_id=?;
-- name: ListWritingTestMatches :many
SELECT * FROM writing_test_matches WHERE user_id=? AND test_id=? ORDER BY round,match_index,id;
-- name: GetWritingTestDecision :one
SELECT * FROM writing_test_matches WHERE user_id=? AND decision_key=?;
-- name: InsertWritingTestMatch :exec
INSERT INTO writing_test_matches(id,user_id,test_id,round,match_index,left_candidate_id,right_candidate_id,winner_candidate_id,decision_key,decided_at)
VALUES(?,?,?,?,?,?,?,?,?,?);
-- name: DecideWritingTestMatch :execrows
UPDATE writing_test_matches SET winner_candidate_id=?,decision_key=?,decided_at=?
WHERE user_id=? AND test_id=? AND id=? AND winner_candidate_id IS NULL;
-- name: ListWritingTestPublications :many
SELECT * FROM writing_test_publications WHERE user_id=? AND test_id=? ORDER BY created_at,id;
-- name: GetWritingTestQuote :one
SELECT * FROM writing_test_quotes WHERE user_id=? AND id=?;
-- name: InsertWritingTestQuote :exec
INSERT INTO writing_test_quotes(id,user_id,test_id,source_post_slug,fingerprint,request,plan,estimated_credits,free,consumed_request_key,consumed_test_id,created_at,expires_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: ConsumeWritingTestQuote :execrows
UPDATE writing_test_quotes SET consumed_request_key=?,consumed_test_id=? WHERE user_id=? AND id=? AND consumed_request_key='';
-- name: GetWritingTestAttempt :one
SELECT * FROM writing_test_attempts WHERE user_id=? AND test_id=? AND id=?;
-- name: GetWritingTestAttemptByRequest :one
SELECT * FROM writing_test_attempts WHERE user_id=? AND request_key=?;
-- name: InsertWritingTestAttempt :exec
INSERT INTO writing_test_attempts(id,user_id,test_id,request_key,fingerprint,epoch,purge_fence,quote_id,job_id,status,candidate_ids,created_at,updated_at,non_metered)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: UpdateWritingTestAttempt :execrows
UPDATE writing_test_attempts SET status=?,job_id=?,confirmed_credits=?,settled=?,failure_reason=?,updated_at=? WHERE user_id=? AND test_id=? AND id=? AND status=?;
-- name: ListWritingTestAttempts :many
SELECT * FROM writing_test_attempts WHERE user_id=? AND test_id=? ORDER BY created_at,id;
-- name: ListInterruptedWritingTestAttempts :many
SELECT * FROM writing_test_attempts WHERE status IN ('prepared','queued','running');
-- name: ListWritingTestCheckpoints :many
SELECT * FROM writing_test_attempt_checkpoints WHERE user_id=? AND test_id=? AND attempt_id=? ORDER BY slot_id;
-- name: UpsertWritingTestCheckpoint :exec
INSERT INTO writing_test_attempt_checkpoints(user_id,test_id,attempt_id,slot_id,checkpoint,updated_at)
VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,test_id,attempt_id,slot_id) DO UPDATE SET checkpoint=excluded.checkpoint,updated_at=excluded.updated_at;
-- name: PurgeWritingTestCheckpoints :exec
UPDATE writing_test_attempt_checkpoints SET checkpoint=NULL WHERE user_id=? AND test_id=?;
-- name: GetWritingTestOperation :one
SELECT * FROM writing_test_operations WHERE user_id=? AND operation_key=?;
-- name: InsertWritingTestOperation :exec
INSERT INTO writing_test_operations(user_id,operation_key,test_id,action,fingerprint,created_at) VALUES(?,?,?,?,?,?);
-- name: ListWritingTestsForPurge :many
SELECT * FROM writing_tests WHERE purge_fence=0 AND content_expires_at IS NOT NULL AND content_expires_at<=? AND status IN ('completed','cancelled','failed','partial');
-- name: ListWritingTestsForSource :many
SELECT * FROM writing_tests WHERE user_id=? AND source_post_slug=?;
-- name: PurgeWritingTestQuotesForSource :exec
UPDATE writing_test_quotes SET source_post_slug=NULL,request='{}',plan=NULL WHERE user_id=? AND source_post_slug=?;
-- name: PurgeExpiredWritingTestQuotes :exec
UPDATE writing_test_quotes SET request='{}',plan=NULL WHERE expires_at<=? AND consumed_request_key='';
-- name: PurgeWritingTestQuoteForTest :exec
UPDATE writing_test_quotes SET request='{}',plan=NULL WHERE user_id=? AND (test_id=? OR consumed_test_id=?);
-- name: ListUnsettledWritingTestAttempts :many
SELECT * FROM writing_test_attempts WHERE settled=0 ORDER BY created_at,id;
