-- Queries for the usage context. sqlc compiles these into internal/usage/store/sqlc;
-- internal/usage/store maps the generated rows to domain types.

-- name: LotsInConsumptionOrder :many
-- The one ordering the balance is ever read in, and the only reader of the product rule
-- behind it (QUOTA-12): KIND first (monthly, then bonus, then purchased) and only then
-- soonest expiry, non-expiring last, oldest first.
--
-- Kind leads because a purchased credit was paid for and must be the last to burn. Expiry
-- order alone used to produce that by accident, resting on the signup bonus happening to be
-- the older of two never-expiring lots; a lot bought before a bonus was granted would have
-- inverted it. The rank is spelled here rather than passed in from Go because this query is
-- the rule's only reader.
--
-- Keep every comment in this file ASCII: sqlc slices the emitted query text by byte offset,
-- so one multi-byte character shifts it and generates SQL that will not parse.
SELECT id, user_id, kind, granted, remaining, expires_at, created_at
FROM credit_lots
WHERE user_id = ?
  AND remaining > 0
  AND (expires_at IS NULL OR expires_at > ?)
ORDER BY CASE kind WHEN 'monthly' THEN 0 WHEN 'bonus' THEN 1 ELSE 2 END,
         expires_at IS NULL, expires_at, created_at, id;

-- name: ActiveMonthlyLot :one
SELECT id, user_id, kind, granted, remaining, expires_at, created_at
FROM credit_lots
WHERE user_id = ? AND kind = 'monthly' AND expires_at IS NOT NULL AND expires_at > ?
ORDER BY expires_at DESC
LIMIT 1;

-- name: InsertLot :exec
INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: RaiseLot :exec
-- Grows a lot that already exists, on both sides at once so the granted/remaining CHECK
-- holds however much of it has been spent. It is the upgrade top-up (QUOTA-35): the one
-- write that edits a lot the account was already given.
UPDATE credit_lots SET granted = granted + ?, remaining = remaining + ? WHERE id = ?;

-- name: VoidUntouchedLot :execrows
UPDATE credit_lots SET remaining = 0 WHERE id = ? AND remaining = granted;

-- name: LotUntouched :one
SELECT EXISTS(
    SELECT 1 FROM credit_lots
    WHERE id = ? AND kind = 'purchased' AND granted > 0 AND remaining = granted
);

-- name: UntouchedPurchasedLots :many
-- Which of these purchased lots are still whole, in one statement. A billing screen asks
-- about every purchase it is about to render, and the answer only decides whether a button
-- appears, so this one reads on the read pool rather than on the single writer that the
-- balance reads deliberately use.
--
-- sqlc.slice keeps the variable IN list a prepared statement rather than concatenated SQL.
SELECT id FROM credit_lots
WHERE id IN (sqlc.slice('ids'))
  AND kind = 'purchased'
  AND granted > 0
  AND remaining = granted;

-- name: RestoreLot :execrows
UPDATE credit_lots SET remaining = remaining + ? WHERE id = ? AND remaining + ? <= granted;

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

-- name: InsertLotIfAbsent :execrows
-- For a grant whose id is derived from what it is FOR rather than randomly: the signup
-- monthly window today and the payment-method bonus later. Re-running the operation must
-- not mint a second lot.
INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING;

-- name: UpsertLot :exec
-- The window write behind a subscription's first charge (QUOTA-42). Unlike
-- InsertLotIfAbsent it overwrites whatever the window's id already held, because the
-- values come from the tier and the window rather than from the row: an account that signs
-- up and subscribes on the same anchor date derives the SAME id for its free window and for
-- the one it just paid for, and keeping the free grant there would silently discard the
-- tier's. Overwriting is also what makes the operation idempotent under a provider retry.
INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    granted = excluded.granted,
    remaining = excluded.remaining,
    expires_at = excluded.expires_at;

-- name: ExpireMonthlyLotsExcept :exec
-- Closes the monthly window an account is running at the instant a new one opens, keeping
-- the row for history the way a lapsed window is kept (QUOTA-12). The excepted id is the
-- window being opened: without it a re-run would expire the lot the upsert had just
-- written, and the pair would stop being idempotent.
UPDATE credit_lots SET expires_at = ?
WHERE user_id = ?
  AND kind = 'monthly'
  AND id <> ?
  AND expires_at IS NOT NULL
  AND expires_at > ?;

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
    prompt_tokens, completion_tokens, reasoning_tokens, reasoning_truncated,
    cost_microusd, cost_source, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- Where one model's completion budget went at one stage, over a recent window. It is the
-- evidence an operator needs to see that a model is ignoring its reasoning effort BEFORE it
-- fails somebody's generation: `supported_parameters` says a model accepts reasoning_effort,
-- never which values it honors, so only measurement can tell.
--
-- Per (model, stage) because the effort is per (model, purpose): an aggregate over the model
-- alone would average an observation stage that is fine together with a writing stage that
-- is not.
-- name: ReasoningSpendByStage :many
SELECT model,
       CAST(COUNT(*) AS INTEGER) AS calls,
       CAST(COALESCE(SUM(reasoning_tokens), 0) AS INTEGER) AS reasoning_tokens,
       CAST(COALESCE(SUM(completion_tokens), 0) AS INTEGER) AS completion_tokens,
       CAST(COALESCE(SUM(reasoning_truncated), 0) AS INTEGER) AS reasoning_truncations
FROM usage_events
WHERE stage = ? AND created_at >= ?
GROUP BY model;

-- name: CostForJob :one
-- COALESCE keeps a job with no recorded call a 0 rather than a NULL the row mapper would
-- have to special-case; the CAST is what makes sqlc type the result int64.
SELECT CAST(COALESCE(SUM(cost_microusd), 0) AS INTEGER) AS total_microusd,
       CAST(COALESCE(SUM(CASE
           WHEN cost_source IN ('reported', 'estimated') AND cost_microusd > 0
           THEN cost_microusd ELSE 0 END), 0) AS INTEGER) AS confirmed_microusd
FROM usage_events WHERE job_id = ?;
