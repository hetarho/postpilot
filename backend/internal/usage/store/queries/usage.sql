-- Queries for the usage context. sqlc compiles these into internal/usage/store/sqlc;
-- internal/usage/store maps the generated rows to domain types.
--
-- Every window is a half-open [start, end) range so a boundary instant belongs to
-- exactly one day and one month, whichever way the clock is read.

-- name: InsertAdmission :exec
INSERT INTO usage_admissions (user_id, kind, job_id, created_at) VALUES (?, ?, ?, ?);

-- name: DeleteAdmissionForJob :exec
DELETE FROM usage_admissions WHERE job_id = ?;

-- name: CountAdmissionsInWindow :one
SELECT COUNT(*) FROM usage_admissions
WHERE user_id = ? AND created_at >= ? AND created_at < ?;

-- name: InsertEvent :exec
INSERT INTO usage_events (
    user_id, kind, job_id, stage, model,
    prompt_tokens, completion_tokens, cost_microusd, cost_source, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: SumCostInWindow :one
-- COALESCE keeps the empty window a 0 rather than a NULL the row mapper would have to
-- special-case; the CAST is what makes sqlc type the result int64 instead of interface{}.
SELECT CAST(COALESCE(SUM(cost_microusd), 0) AS INTEGER) FROM usage_events
WHERE user_id = ? AND created_at >= ? AND created_at < ?;
