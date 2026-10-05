-- name: SaveUnitQuote :exec
INSERT INTO usage_unit_quotes (id, user_id, kind, digest, quote_json, expires_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: PurgeExpiredUnitQuotes :exec
DELETE FROM usage_unit_quotes WHERE expires_at <= ? AND consumed_job_id = '';

-- name: GetUnitQuote :one
SELECT quote_json, consumed_job_id FROM usage_unit_quotes WHERE id = ? AND user_id = ?;

-- name: ConsumeUnitQuote :execrows
UPDATE usage_unit_quotes SET consumed_job_id = ? WHERE id = ? AND consumed_job_id = '';

-- name: SaveUnitAdmission :exec
INSERT INTO usage_unit_admissions (job_id, admission_id, quote_id, budgets_json)
SELECT sqlc.arg(job_id), id, sqlc.arg(quote_id), sqlc.arg(budgets_json)
FROM usage_admissions WHERE job_id = sqlc.arg(job_id) AND settled_at IS NULL;

-- name: GetUnitAdmission :one
SELECT quote_id, budgets_json FROM usage_unit_admissions WHERE job_id = ?;

-- name: ClaimUnitCall :execrows
INSERT INTO usage_unit_calls (id, job_id, budget_fingerprint, created_at)
SELECT sqlc.arg(id), sqlc.arg(job_id), sqlc.arg(fingerprint), sqlc.arg(created_at)
WHERE (SELECT COUNT(*) FROM usage_unit_calls AS prior WHERE prior.job_id = sqlc.arg(job_id) AND prior.budget_fingerprint = sqlc.arg(fingerprint)) < CAST(sqlc.arg(maximum_calls) AS INTEGER);

-- name: GetUnitClaim :one
SELECT c.budget_fingerprint, a.user_id, a.kind, c.job_id FROM usage_unit_calls c
JOIN usage_admissions a ON a.job_id = c.job_id WHERE c.id = ?;

-- name: UnitEventRecorded :one
SELECT COUNT(*) FROM usage_unit_events WHERE claim_id = ?;

-- name: InsertUnitBaseEvent :one
INSERT INTO usage_events (user_id, kind, job_id, stage, model, prompt_tokens, completion_tokens, reasoning_tokens, reasoning_truncated, cost_microusd, cost_source, created_at)
VALUES (?, ?, ?, ?, ?, 0, 0, 0, 0, ?, ?, ?) RETURNING id;

-- name: InsertUnitEvidence :exec
INSERT INTO usage_unit_events (event_id, claim_id, provider_id, supplier_request_id, budget_fingerprint, evidence_json, reported_usd, exact_usd, cost_source, chargeable)
VALUES (sqlc.arg(event_id), sqlc.arg(claim_id), sqlc.arg(provider_id), sqlc.arg(request_id), sqlc.arg(fingerprint), sqlc.arg(evidence_json), sqlc.arg(reported_usd), sqlc.arg(exact_usd), sqlc.arg(cost_source),
    NOT EXISTS (SELECT 1 FROM usage_unit_events WHERE supplier_request_id <> '' AND provider_id = sqlc.arg(provider_id) AND supplier_request_id = sqlc.arg(request_id)));

-- name: UnitCostsForJob :many
SELECT u.exact_usd FROM usage_unit_events u JOIN usage_events e ON e.id = u.event_id
WHERE e.job_id = ? AND u.chargeable = 1 AND u.cost_source IN ('reported', 'estimated');

-- name: ClaimBoundedUnitCall :execrows
INSERT INTO usage_unit_calls(id,job_id,budget_fingerprint,created_at,input_characters,input_digest)
SELECT sqlc.arg(id),sqlc.arg(job),sqlc.arg(fingerprint),sqlc.arg(now),sqlc.arg(characters),sqlc.arg(input_digest)
WHERE (SELECT COUNT(*) FROM usage_unit_calls prior WHERE prior.job_id=sqlc.arg(job) AND prior.budget_fingerprint=sqlc.arg(fingerprint)) < CAST(sqlc.arg(max_calls) AS INTEGER)
AND (SELECT COALESCE(SUM(input_characters),0) FROM usage_unit_calls prior WHERE prior.job_id=sqlc.arg(job) AND prior.budget_fingerprint=sqlc.arg(fingerprint)) + CAST(sqlc.arg(characters) AS INTEGER) <= CAST(sqlc.arg(max_characters) AS INTEGER);
