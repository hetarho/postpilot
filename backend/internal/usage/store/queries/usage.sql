-- Queries for the usage context. sqlc compiles these into internal/usage/store/sqlc;
-- internal/usage/store maps the generated rows to domain types.

-- name: LotsInConsumptionOrder :many
-- The one ordering the balance is ever read in, and the only reader of the product rule
-- behind it (QUOTA-12): every lot that carries an expiry first (monthly, voucher and an
-- expiring bonus alike) by soonest expiry, then the never-expiring bonus lots, then the
-- purchased ones; ties go to the oldest.
--
-- Expiry leads because a credit that can lapse must burn before one that cannot: a voucher
-- spent after a never-expiring bonus would run out its clock unspent. Purchased still comes
-- last because a paid credit must be the last to burn, and it never expires anyway. The
-- rank is spelled here rather than passed in from Go because this query is the rule's only
-- reader.
--
-- Keep every comment in this file ASCII: sqlc slices the emitted query text by byte offset,
-- so one multi-byte character shifts it and generates SQL that will not parse.
SELECT id, user_id, kind, granted, remaining, expires_at, created_at,
       coverage_id, window_start, issuance_cause, correlation_id
FROM credit_lots
WHERE user_id = ?
  AND remaining > 0
  AND refund_request_id IS NULL
  AND (expires_at IS NULL OR expires_at > ?)
ORDER BY CASE WHEN kind = 'purchased' THEN 2 WHEN expires_at IS NULL THEN 1 ELSE 0 END,
         expires_at, created_at, id;

-- name: ActiveMonthlyLot :one
SELECT id, user_id, kind, granted, remaining, expires_at, created_at,
       coverage_id, window_start, issuance_cause, correlation_id
FROM credit_lots
WHERE user_id = ? AND kind = 'monthly' AND expires_at IS NOT NULL AND expires_at > ?
ORDER BY expires_at DESC
LIMIT 1;

-- name: InsertLot :exec
INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at,
                         coverage_id, window_start, issuance_cause, correlation_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: RaiseLot :exec
-- Grows a lot that already exists, on both sides at once so the granted/remaining CHECK
-- holds however much of it has been spent. It is the upgrade top-up (QUOTA-35): the one
-- write that edits a lot the account was already given.
UPDATE credit_lots SET granted = granted + ?, remaining = remaining + ? WHERE id = ?;

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

-- name: ExpireVoucherLot :execrows
-- A revoked voucher's lot (QUOTA-58). Voiding moves the expiry to the revocation instant
-- instead of zeroing `remaining`: RefundToLot adds back into any lot up to its grant, so a
-- zeroed lot would take back the unused part of a hold that was open when the voucher was
-- revoked. Expiry is read-time, so a refund landing after it counts toward nothing, and the
-- remainder stays on the row as history.
UPDATE credit_lots SET expires_at = ?
WHERE id = ? AND kind = 'voucher' AND expires_at > ?;

-- name: VoucherLots :many
-- Where each of these voucher lots stands, in one statement on the read pool. The operator's
-- voucher list is its only reader, and the answer decides nothing but what a row shows.
SELECT id, user_id, kind, granted, remaining, expires_at, created_at,
       coverage_id, window_start, issuance_cause, correlation_id
FROM credit_lots
WHERE id IN (sqlc.slice('ids'))
  AND kind = 'voucher';

-- name: SpendFromLot :exec
-- The `remaining >= ?` guard is in the statement rather than in a read before it: two
-- writers that each read the same lot must not both pass their own arithmetic.
UPDATE credit_lots SET remaining = remaining - ? WHERE id = ? AND remaining >= ? AND refund_request_id IS NULL;

-- name: RefundToLot :exec
-- Bounded by the grant for the same reason: a double settle cannot inflate a lot past
-- what it was ever given.
UPDATE credit_lots SET remaining = remaining + ? WHERE id = ? AND remaining + ? <= granted;

-- name: InsertAdmission :exec
INSERT INTO usage_admissions (user_id, kind, job_id, hold_credits, created_at, approved_max_credits, cancellation_policy_version,
                              coverage_id,daily_window_start,benefit_window_start,
                              fx_source,fx_publication_date,fx_reference_e4,fx_applied_e4,fx_temporary,
                              admitted_plan,admitted_models_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteAdmissionForJob :exec
DELETE FROM usage_admissions WHERE job_id = ?;

-- name: InsertLotIfAbsent :execrows
-- Window grants never refill. A successful term renewal may extend the expiry of
-- an already opened day or month that crossed the old paid term boundary.
INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at,
                         coverage_id, window_start, issuance_cause, correlation_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET expires_at = excluded.expires_at
WHERE credit_lots.kind IN ('daily', 'monthly')
  AND credit_lots.coverage_id = excluded.coverage_id
  AND credit_lots.expires_at < excluded.expires_at;

-- name: WindowLotExpiries :many
-- Where each of these window grants already ends, on the read pool. A balance read compares
-- them with the grants it would open: InsertLotIfAbsent writes only a missing grant or one
-- that ends sooner.
SELECT id, expires_at FROM credit_lots
WHERE id IN (sqlc.slice('ids'));

-- name: InsertHoldDebit :exec
INSERT INTO credit_hold_lots (job_id, lot_id, credits, origin_coverage_id, origin_window_start)
VALUES (?, ?, ?, ?, ?);

-- name: InsertEligibleLot :exec
INSERT INTO usage_admission_eligible_lots(job_id,lot_id) VALUES (?, ?)
ON CONFLICT(job_id,lot_id) DO NOTHING;

-- name: EligibleLotsForJob :many
SELECT l.id,l.user_id,l.kind,l.granted,l.remaining,l.expires_at,l.created_at,
       l.coverage_id,l.window_start,l.issuance_cause,l.correlation_id
FROM usage_admission_eligible_lots e JOIN credit_lots l ON l.id=e.lot_id
WHERE e.job_id=? AND l.remaining>0 AND l.refund_request_id IS NULL
ORDER BY CASE WHEN l.kind='purchased' THEN 2 WHEN l.expires_at IS NULL THEN 1 ELSE 0 END,
         l.expires_at,l.created_at,l.id;

-- name: DeleteEligibleLotsForJob :exec
DELETE FROM usage_admission_eligible_lots WHERE job_id=?;

-- name: HoldDebitsForJob :many
SELECT lot_id, credits FROM credit_hold_lots WHERE job_id = ? ORDER BY rowid;

-- name: OpenAdmissionForJob :one
-- Only an unsettled admission is returned, which is what makes settlement idempotent: a
-- terminal transition that runs twice finds nothing the second time.
SELECT user_id, kind, job_id, hold_credits, created_at, approved_max_credits, cancellation_policy_version,
       coverage_id,daily_window_start,benefit_window_start,
       fx_source,fx_publication_date,fx_reference_e4,fx_applied_e4,fx_temporary,
       admitted_plan,admitted_models_json
FROM usage_admissions
WHERE job_id = ? AND settled_at IS NULL;

-- The kinds that settle against an approved ceiling are passed in, not named here. The
-- filter stays on the kind rather than on `approved_max_credits IS NOT NULL` so that
-- admissions written before the ceiling column existed are still projected.
-- name: AccountingForJob :one
SELECT a.approved_max_credits, a.hold_credits, a.settled_credits, a.settled_at, a.cancellation_policy_version, a.settlement_reason, a.confirmed_charge_credits, a.cancellation_fee_credits,
       a.settlement_cause,a.compensation_credits,a.compensation_lot_id,a.compensation_expires_at,
       a.fx_source,a.fx_publication_date,a.fx_reference_e4,a.fx_applied_e4,a.fx_temporary,
       CAST(COALESCE((SELECT SUM(h.credits) FROM credit_hold_lots h WHERE h.job_id = a.job_id), 0) AS INTEGER) AS debited_credits
FROM usage_admissions a
WHERE a.user_id = sqlc.arg(user_id) AND a.job_id = sqlc.arg(job_id)
  AND (a.kind IN (SELECT value FROM json_each(sqlc.arg(kinds)))
       OR EXISTS (SELECT 1 FROM usage_unit_admissions u WHERE u.admission_id = a.id));

-- name: MarkAdmissionSettled :exec
UPDATE usage_admissions SET settled_credits = ?, settled_at = ?, settlement_reason = ?, confirmed_charge_credits = ?, cancellation_fee_credits = ?,
       settlement_cause=?,compensation_credits=?,compensation_lot_id=?,compensation_expires_at=?
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
FROM usage_events WHERE job_id = ?
AND NOT EXISTS (SELECT 1 FROM usage_unit_events WHERE event_id = usage_events.id);

-- One row per successfully settled post generation and stage: the provider cost that stage's
-- calls recorded and the rate the job was admitted at, which is what the per-post credit
-- figure is computed from (QUOTA-64). A group holding any call whose cost is unavailable is
-- left out rather than counted low, and a job admitted before rates were frozen has no rate
-- to convert with, so it is left out too.
-- name: RecentPostStageCosts :many
SELECT a.job_id,
       a.user_id,
       e.stage,
       e.model,
       CAST(COALESCE(SUM(e.cost_microusd), 0) AS INTEGER) AS cost_microusd,
       a.fx_source,
       a.fx_publication_date,
       a.fx_reference_e4,
       a.fx_applied_e4
FROM usage_admissions a
JOIN usage_events e ON e.job_id = a.job_id
WHERE a.kind = 'generate'
  AND a.settlement_reason = 'succeeded'
  AND a.settled_at >= ?
  AND a.fx_applied_e4 IS NOT NULL
GROUP BY a.job_id, a.user_id, e.stage, e.model,
         a.fx_source, a.fx_publication_date, a.fx_reference_e4, a.fx_applied_e4
HAVING SUM(CASE WHEN e.cost_source = 'unavailable' THEN 1 ELSE 0 END) = 0;
