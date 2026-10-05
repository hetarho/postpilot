-- +goose Up
-- The ledger prices through the FX path alone (QUOTA-14, QUOTA-59), so the rows only the
-- pre-FX regime understood are closed here once instead of on every read.
--
-- An admission still open with credits held and no frozen rate was written before FX
-- settlement. Its unfinished work is charged nothing: every credit the hold took returns to
-- the lot it came from (never past what that lot granted, as RefundToLot guards), and the
-- admission goes with its hold and eligible-lot rows, as Release drops a start that never
-- happened. A job of such an admission that still runs finds no admission and makes no paid
-- call. Usage events stay: they are the provider's evidence, not a charge.
UPDATE credit_lots
SET remaining = MIN(granted, remaining + (
    SELECT SUM(h.credits) FROM credit_hold_lots h
    JOIN usage_admissions a ON a.job_id = h.job_id
    WHERE h.lot_id = credit_lots.id
      AND a.settled_at IS NULL AND a.hold_credits > 0 AND a.fx_applied_e4 IS NULL))
WHERE id IN (
    SELECT h.lot_id FROM credit_hold_lots h
    JOIN usage_admissions a ON a.job_id = h.job_id
    WHERE a.settled_at IS NULL AND a.hold_credits > 0 AND a.fx_applied_e4 IS NULL);

DELETE FROM usage_admission_eligible_lots WHERE job_id IN (
    SELECT job_id FROM usage_admissions
    WHERE settled_at IS NULL AND hold_credits > 0 AND fx_applied_e4 IS NULL);
DELETE FROM credit_hold_lots WHERE job_id IN (
    SELECT job_id FROM usage_admissions
    WHERE settled_at IS NULL AND hold_credits > 0 AND fx_applied_e4 IS NULL);
DELETE FROM usage_admissions
WHERE settled_at IS NULL AND hold_credits > 0 AND fx_applied_e4 IS NULL;

-- A monthly lot with no coverage is a window from before benefit windows. Renewal used to
-- expire one on every read; every one still open ends now instead, kept for history like
-- any lapsed window. The stamp is the store's own layout, so it compares as stored text.
UPDATE credit_lots
SET expires_at = strftime('%Y-%m-%dT%H:%M:%S', 'now') || substr(strftime('%f', 'now'), 3) || '000000Z'
WHERE kind = 'monthly' AND coverage_id IS NULL
  AND expires_at > strftime('%Y-%m-%dT%H:%M:%S', 'now') || substr(strftime('%f', 'now'), 3) || '000000Z';

-- +goose Down
-- Returned credits, dropped admissions and ended windows are not restored: rolling the
-- binary back leaves a ledger the pre-FX code still reads, with nothing left open for it.
SELECT 1;
