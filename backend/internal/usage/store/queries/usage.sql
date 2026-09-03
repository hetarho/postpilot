-- Queries for the usage context. sqlc compiles these into internal/usage/store/sqlc;
-- internal/usage/store maps the generated rows to domain types.

-- name: LotsInConsumptionOrder :many
-- The one ordering the balance is ever read in: soonest expiry first, non-expiring last.
-- `expires_at IS NULL` sorts 0 before 1, which is what puts a bonus behind the monthly
-- grant that would otherwise lapse unspent.
SELECT id, user_id, kind, granted, remaining, expires_at, created_at
FROM credit_lots
WHERE user_id = ?
  AND remaining > 0
  AND (expires_at IS NULL OR expires_at > ?)
ORDER BY expires_at IS NULL, expires_at, created_at, id;

-- name: ActiveMonthlyLot :one
SELECT id, user_id, kind, granted, remaining, expires_at, created_at
FROM credit_lots
WHERE user_id = ? AND kind = 'monthly' AND expires_at IS NOT NULL AND expires_at > ?
ORDER BY expires_at DESC
LIMIT 1;

-- name: InsertLot :exec
INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: SpendFromLot :exec
-- The `remaining >= ?` guard is in the statement rather than in a read before it: two
-- writers that each read the same lot must not both pass their own arithmetic.
UPDATE credit_lots SET remaining = remaining - ? WHERE id = ? AND remaining >= ?;

-- name: RefundToLot :exec
-- Bounded by the grant for the same reason: a double settle cannot inflate a lot past
-- what it was ever given.
UPDATE credit_lots SET remaining = remaining + ? WHERE id = ? AND remaining + ? <= granted;

-- name: InsertAdmission :exec
INSERT INTO usage_admissions (user_id, kind, job_id, hold_credits, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: DeleteAdmissionForJob :exec
DELETE FROM usage_admissions WHERE job_id = ?;

-- name: InsertLotIfAbsent :exec
-- For a grant whose id is derived from what it is FOR rather than randomly: the signup
-- bonus. `adduser` is rerunnable to repair an account, and a repair must not mint a
-- second bonus.
INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING;

-- name: InsertHoldDebit :exec
INSERT INTO credit_hold_lots (job_id, lot_id, credits) VALUES (?, ?, ?);

-- name: HoldDebitsForJob :many
SELECT lot_id, credits FROM credit_hold_lots WHERE job_id = ? ORDER BY rowid;

-- name: OpenAdmissionForJob :one
-- Only an unsettled admission is returned, which is what makes settlement idempotent: a
-- terminal transition that runs twice finds nothing the second time.
SELECT user_id, kind, job_id, hold_credits, created_at
FROM usage_admissions
WHERE job_id = ? AND settled_at IS NULL;

-- name: MarkAdmissionSettled :exec
UPDATE usage_admissions SET settled_credits = ?, settled_at = ?
WHERE job_id = ? AND settled_at IS NULL;

-- name: UnsettledHoldJobs :many
SELECT job_id FROM usage_admissions WHERE settled_at IS NULL ORDER BY created_at;

-- name: InsertEvent :exec
INSERT INTO usage_events (
    user_id, kind, job_id, stage, model,
    prompt_tokens, completion_tokens, cost_microusd, cost_source, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: SumCostForJob :one
-- COALESCE keeps a job with no recorded call a 0 rather than a NULL the row mapper would
-- have to special-case; the CAST is what makes sqlc type the result int64.
SELECT CAST(COALESCE(SUM(cost_microusd), 0) AS INTEGER) FROM usage_events WHERE job_id = ?;
