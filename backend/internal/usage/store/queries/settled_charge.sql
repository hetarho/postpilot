-- name: SettledChargeForJob :one
SELECT CAST(CASE WHEN COALESCE((SELECT SUM(h.credits) FROM credit_hold_lots h WHERE h.job_id=a.job_id),0)=0
 THEN 0 ELSE a.settled_credits END AS INTEGER) AS charge
FROM usage_admissions a
WHERE a.user_id=? AND a.job_id=? AND a.settled_at IS NOT NULL AND a.settled_credits IS NOT NULL;
