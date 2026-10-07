-- name: InsertEnqueueReceipt :exec
INSERT INTO job_enqueue_receipts(job_id,user_id,fingerprint,state,created_at) VALUES(?,?,?,'claimed',?)
ON CONFLICT(job_id) DO NOTHING;

-- name: CommitEnqueueReceipt :execrows
UPDATE job_enqueue_receipts SET state='committed'
WHERE job_id=? AND user_id=? AND fingerprint=? AND state='claimed';

-- name: GetEnqueueReceipt :one
SELECT job_id,user_id,fingerprint,state FROM job_enqueue_receipts WHERE job_id=?;

-- name: AbandonEnqueueReceipt :execrows
UPDATE job_enqueue_receipts SET state='abandoned'
WHERE job_id=? AND state IN ('claimed','committed')
AND NOT EXISTS(SELECT 1 FROM generation_jobs WHERE generation_jobs.id=job_enqueue_receipts.job_id);
