-- name: InsertAnalysisPreparation :exec
INSERT INTO clip_analysis_preparations(id,user_id,project_id,batch_id,quote_id,profile_version,expected_revision,metadata_json,manifest_digest,created_at,expires_at,queue_deadline_at,deadline_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: GetAnalysisPreparation :one
SELECT * FROM clip_analysis_preparations WHERE id=? AND user_id=?;
-- name: AnalysisPreparationForQuote :one
SELECT * FROM clip_analysis_preparations WHERE quote_id=? AND user_id=?;
-- name: AnalysisPreparationForStage :one
SELECT * FROM clip_analysis_preparations WHERE id=?;
-- name: LiveAnalysisPreparations :many
SELECT * FROM clip_analysis_preparations WHERE reconciled_at IS NULL ORDER BY id LIMIT 100;
-- name: AnalysisPreparationCapacity :one
SELECT COUNT(*) FROM clip_analysis_preparations WHERE state IN ('preparing','verifying') AND expires_at>?;
-- name: AnalysisPreparationAccountCapacity :one
SELECT COUNT(*) FROM clip_analysis_preparations WHERE user_id=? AND state IN ('preparing','verifying') AND expires_at>?;
-- name: BindAnalysisQuoteDigest :execrows
UPDATE clip_generation_quotes SET input_digest=sqlc.arg(bound_digest)
WHERE id=sqlc.arg(quote_id) AND user_id=sqlc.arg(user_id) AND input_digest=sqlc.arg(original_digest) AND consumed_job_id IS NULL AND expires_at>sqlc.arg(now);
-- name: InsertAnalysisCopy :exec
INSERT INTO clip_analysis_copies(preparation_id,slot,source_id,source_fingerprint,ordinal,offset_ms,duration_ms,width,height,has_audio) VALUES(?,?,?,?,?,?,?,?,?,?);
-- name: AnalysisCopies :many
SELECT * FROM clip_analysis_copies WHERE preparation_id=? ORDER BY source_id,ordinal;
-- name: ReserveAnalysisCopy :execrows
UPDATE clip_analysis_copies SET object_key=COALESCE(object_key,sqlc.arg(object_key)),exact_bytes=sqlc.arg(exact_bytes),sha256=sqlc.arg(sha256),state='reserved',put_expires_at=MAX(COALESCE(put_expires_at,''),sqlc.arg(put_expires_at))
WHERE preparation_id=sqlc.arg(preparation_id) AND slot=sqlc.arg(slot)
AND (state='expected' OR (state='reserved' AND exact_bytes=sqlc.arg(exact_bytes) AND sha256=sqlc.arg(sha256)))
AND EXISTS(SELECT 1 FROM clip_analysis_preparations p WHERE p.id=clip_analysis_copies.preparation_id AND p.state='preparing' AND p.expires_at>sqlc.arg(now));
-- name: BindAnalysisParent :execrows
UPDATE clip_analysis_preparations SET parent_job_id=COALESCE(parent_job_id,sqlc.arg(parent_job_id))
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='preparing' AND expires_at>sqlc.arg(now)
AND (parent_job_id IS NULL OR parent_job_id=sqlc.arg(parent_job_id));
-- name: SubmitAnalysisPreparation :execrows
UPDATE clip_analysis_preparations SET state='verifying',manifest_digest=sqlc.arg(manifest_digest),deadline_at=MIN(expires_at,sqlc.arg(deadline_at)),queue_deadline_at=MIN(expires_at,sqlc.arg(deadline_at),sqlc.arg(queue_deadline_at))
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='preparing' AND expires_at>sqlc.arg(now);
-- name: AcceptEmptyAnalysisPreparation :execrows
UPDATE clip_analysis_preparations SET state='accepted',progress=1000
WHERE id=sqlc.arg(id) AND user_id=sqlc.arg(user_id) AND state='preparing' AND expires_at>sqlc.arg(now)
AND NOT EXISTS(SELECT 1 FROM clip_analysis_copies c WHERE c.preparation_id=clip_analysis_preparations.id);
-- name: ClaimAnalysisPreparation :one
UPDATE clip_analysis_preparations SET current_attempt_id=sqlc.arg(attempt_id),attempt_count=attempt_count+1
WHERE id=(SELECT p.id FROM clip_analysis_preparations p LEFT JOIN clip_analysis_verification_attempts a ON a.id=p.current_attempt_id
WHERE p.state='verifying' AND p.profile_version=sqlc.arg(profile_version) AND p.expires_at>sqlc.arg(now) AND p.deadline_at>sqlc.arg(now)
AND p.attempt_count<CAST(sqlc.arg(max_attempts) AS INTEGER) AND (p.attempt_count>0 OR p.queue_deadline_at>sqlc.arg(now))
AND (a.id IS NULL OR a.outcome IS NOT NULL OR a.lease_expires_at<=sqlc.arg(now))
AND (SELECT COUNT(*) FROM clip_analysis_verification_attempts active JOIN clip_analysis_preparations owner ON owner.current_attempt_id=active.id
WHERE active.outcome IS NULL AND active.lease_expires_at>sqlc.arg(now))<CAST(sqlc.arg(active_limit) AS INTEGER)
ORDER BY p.created_at,p.id LIMIT 1)
RETURNING *;
-- name: InsertAnalysisVerificationAttempt :exec
INSERT INTO clip_analysis_verification_attempts(id,preparation_id,ordinal,worker_id,token_hash,lease_expires_at,started_at,runtime_manifest) VALUES(?,?,?,?,?,?,?,?);
-- name: GetAnalysisVerificationAttempt :one
SELECT * FROM clip_analysis_verification_attempts WHERE id=?;
-- name: ExpireAnalysisVerificationAttempts :exec
UPDATE clip_analysis_verification_attempts SET outcome='expired',finished_at=sqlc.arg(now)
WHERE preparation_id=sqlc.arg(preparation_id) AND outcome IS NULL AND lease_expires_at<=sqlc.arg(now);
-- name: RenewAnalysisVerification :execrows
UPDATE clip_analysis_verification_attempts SET lease_expires_at=sqlc.arg(lease_expires_at)
WHERE id=sqlc.arg(id) AND worker_id=sqlc.arg(worker_id) AND token_hash=sqlc.arg(token_hash) AND outcome IS NULL AND lease_expires_at>sqlc.arg(now);
-- name: UpdateAnalysisProgress :exec
UPDATE clip_analysis_preparations SET progress=MAX(progress,sqlc.arg(progress)) WHERE id=sqlc.arg(id);
-- name: VerifyAnalysisCopy :execrows
UPDATE clip_analysis_copies SET state='verified',verification_json=sqlc.arg(verification_json)
WHERE preparation_id=sqlc.arg(preparation_id) AND slot=sqlc.arg(slot) AND state='reserved' AND exact_bytes=sqlc.arg(exact_bytes) AND sha256=sqlc.arg(sha256);
-- name: AcceptAnalysisVerification :execrows
UPDATE clip_analysis_preparations SET state='accepted',accepted_result=sqlc.arg(accepted_result),progress=1000
WHERE id=sqlc.arg(id) AND state='verifying' AND current_attempt_id=sqlc.arg(attempt_id) AND expires_at>sqlc.arg(now) AND deadline_at>sqlc.arg(now);
-- name: StopAnalysisVerificationAttempt :exec
UPDATE clip_analysis_verification_attempts SET outcome=COALESCE(outcome,sqlc.arg(outcome)),finished_at=COALESCE(finished_at,sqlc.arg(now)) WHERE id=sqlc.arg(id);
-- name: SetAnalysisPreparationState :exec
UPDATE clip_analysis_preparations SET state=sqlc.arg(state),failure=sqlc.narg(failure) WHERE id=sqlc.arg(id);
-- name: ConsumeAnalysisPreparation :execrows
UPDATE clip_analysis_preparations SET state='consumed' WHERE id=sqlc.arg(id) AND parent_job_id=sqlc.arg(parent_job_id) AND state IN ('accepted','consumed') AND expires_at>sqlc.arg(now);
-- name: QueueAnalysisCopyCleanup :exec
INSERT INTO clip_analysis_copy_deletions(object_key,delete_after)
SELECT object_key,MAX(COALESCE(put_expires_at,''),sqlc.arg(now)) FROM clip_analysis_copies WHERE preparation_id=sqlc.arg(preparation_id) AND object_key IS NOT NULL AND state!='deleted'
ON CONFLICT(object_key) DO UPDATE SET delete_after=MAX(delete_after,excluded.delete_after);
-- name: RetireAnalysisCopies :exec
UPDATE clip_analysis_copies SET state='cleanup' WHERE preparation_id=? AND object_key IS NOT NULL AND state!='deleted';
-- name: MarkAnalysisPreparationCleanup :exec
UPDATE clip_analysis_preparations SET cleanup_at=? WHERE id=?;
-- name: DueAnalysisCopyDeletions :many
SELECT * FROM clip_analysis_copy_deletions WHERE delete_after<=? ORDER BY delete_after,object_key LIMIT 100;
-- name: RemoveAnalysisCopyDeletion :exec
DELETE FROM clip_analysis_copy_deletions WHERE object_key=?;
-- name: MarkAnalysisCopyDeleted :exec
UPDATE clip_analysis_copies SET state='deleted' WHERE object_key=?;
-- name: AnalysisObjectKeyExists :one
SELECT COUNT(*) FROM clip_analysis_copies WHERE object_key=? AND state!='deleted';
-- name: QueueAnalysisOrphanDeletion :exec
INSERT INTO clip_analysis_copy_deletions(object_key,delete_after) VALUES(?,?) ON CONFLICT DO NOTHING;
-- name: AnalysisVerificationWaitingCount :one
SELECT COUNT(*) FROM clip_analysis_preparations p LEFT JOIN clip_analysis_verification_attempts a ON a.id=p.current_attempt_id
WHERE p.state='verifying' AND p.expires_at>sqlc.arg(now) AND p.deadline_at>sqlc.arg(now)
AND p.attempt_count<CAST(sqlc.arg(max_attempts) AS INTEGER)
AND (p.attempt_count>0 OR p.queue_deadline_at>sqlc.arg(now))
AND (a.id IS NULL OR a.outcome IS NOT NULL OR a.lease_expires_at<=sqlc.arg(now));
-- name: AnalysisVerificationActiveCount :one
SELECT COUNT(*) FROM clip_analysis_verification_attempts a JOIN clip_analysis_preparations p ON p.current_attempt_id=a.id
WHERE a.outcome IS NULL AND a.lease_expires_at>sqlc.arg(now);
-- name: AnalysisVerificationOwnActiveCount :one
SELECT COUNT(*) FROM clip_analysis_verification_attempts a JOIN clip_analysis_preparations p ON p.current_attempt_id=a.id
WHERE a.outcome IS NULL AND a.lease_expires_at>sqlc.arg(now) AND a.worker_id=sqlc.arg(worker_id);

-- name: MarkAnalysisPreparationReconciled :exec
UPDATE clip_analysis_preparations SET reconciled_at=? WHERE id=?;
