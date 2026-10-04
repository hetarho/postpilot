package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/usage"
)

// fundingPredicate selects the credit lots a refunded payment funded from the fields of the
// funding value billing computed, and from nothing else: the pack's lot, the lots correlated to
// the order, and the coverage's daily and monthly grants whose window starts in [Start, End) —
// every one but upgrade bonuses, or only the window cause's. The grant trigger
// credit_refund_block_future_grant (migration 0130) applies this same predicate — less the lot
// clause, as an issued lot is never issued again — to a lot about to be issued, over the guard
// row GuardRefundFunding writes from the same fields.
const fundingPredicate = `l.user_id=? AND ((?<>'' AND l.id=?) OR (?<>'' AND l.correlation_id=?)
	OR (?<>'' AND l.coverage_id=? AND l.kind IN ('daily','monthly')
		AND COALESCE(l.issuance_cause,'')<>'upgrade' AND (?='' OR l.issuance_cause=?)
		AND l.window_start>=? AND l.window_start<?))`

func fundingArgs(f usage.RefundFunding) []any {
	return []any{f.UserID, f.LotID, f.LotID, f.Correlation, f.Correlation, f.CoverageID, f.CoverageID,
		f.WindowCause, f.WindowCause, f.Start.UTC().Format(writeLayout), f.End.UTC().Format(writeLayout)}
}

func (s *Store) RefundFundingEvidence(ctx context.Context, f usage.RefundFunding, at time.Time) (usage.RefundFundingEvidence, error) {
	var e usage.RefundFundingEvidence
	args := fundingArgs(f)
	var used, remaining sql.NullInt64
	err := s.raw.QueryRowContext(ctx, `SELECT SUM(l.granted-l.remaining),
		SUM(CASE WHEN l.expires_at IS NULL OR l.expires_at>? THEN l.remaining ELSE 0 END)
		FROM credit_lots l WHERE `+fundingPredicate,
		append([]any{at.UTC().Format(writeLayout)}, args...)...).Scan(&used, &remaining)
	if err != nil {
		return e, err
	}
	e.CreditsUsed, e.CreditsRemaining = int(used.Int64), int(remaining.Int64)
	var reserved, jobs sql.NullInt64
	err = s.raw.QueryRowContext(ctx, `SELECT SUM(h.credits),COUNT(DISTINCT a.job_id)
		FROM credit_hold_lots h JOIN usage_admissions a ON a.job_id=h.job_id
		JOIN credit_lots l ON l.id=h.lot_id
		WHERE a.settled_at IS NULL AND `+fundingPredicate, args...).Scan(&reserved, &jobs)
	if err != nil {
		return e, err
	}
	e.CreditsReserved = int(reserved.Int64)
	err = s.raw.QueryRowContext(ctx, `SELECT COUNT(DISTINCT a.job_id)
		FROM credit_hold_lots h JOIN usage_admissions a ON a.job_id=h.job_id
		JOIN credit_lots l ON l.id=h.lot_id WHERE `+fundingPredicate,
		args...).Scan(&jobs)
	if err != nil {
		return e, err
	}
	e.PaidJobs = int(jobs.Int64)
	// Reservations debit remaining at admission, but no confirmed consumption
	// has occurred yet. Keep those amounts separate in the operator's evidence.
	e.CreditsUsed = max(0, e.CreditsUsed-e.CreditsReserved)
	return e, nil
}

// GuardRefundFunding freezes the lots the payment funded and, when it funded grants still to
// come (a coverage window or a correlation), records the guard row the grant trigger reads.
func (s *Store) GuardRefundFunding(ctx context.Context, f usage.RefundFunding, requestID string) error {
	if f.CoverageID != "" || f.Correlation != "" {
		_, err := s.raw.ExecContext(ctx, `INSERT INTO credit_refund_funding_guards
			(request_id,user_id,order_id,kind,coverage_id,starts_at,ends_at,correlation_id,window_cause)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			requestID, f.UserID, f.OrderID, f.Kind, f.CoverageID,
			f.Start.UTC().Format(writeLayout), f.End.UTC().Format(writeLayout), f.Correlation, f.WindowCause)
		if err != nil {
			return err
		}
	}
	_, err := s.raw.ExecContext(ctx, `UPDATE credit_lots SET refund_request_id=?
		WHERE id IN (SELECT l.id FROM credit_lots l WHERE `+fundingPredicate+`)
		AND refund_request_id IS NULL`, append([]any{requestID}, fundingArgs(f)...)...)
	return err
}

func (s *Store) ReleaseRefundFunding(ctx context.Context, requestID string) error {
	if _, err := s.raw.ExecContext(ctx, `DELETE FROM credit_refund_funding_guards WHERE request_id=?`, requestID); err != nil {
		return err
	}
	_, err := s.raw.ExecContext(ctx, `UPDATE credit_lots SET refund_request_id=NULL WHERE refund_request_id=?`, requestID)
	return err
}

func (s *Store) ConfirmRefundFunding(ctx context.Context, requestID string, at time.Time) error {
	// An expiry keeps grant/spend history and ensures a late hold return cannot
	// make these credits spendable again. Purchased lots have no expiry, so zero
	// their unspent remainder after active holds have been ruled out.
	_, err := s.raw.ExecContext(ctx, `UPDATE credit_lots SET
		remaining=CASE WHEN kind='purchased' THEN 0 ELSE remaining END,
		expires_at=CASE WHEN kind='purchased' THEN expires_at ELSE ? END
		WHERE refund_request_id=?`, at.UTC().Format(writeLayout), requestID)
	if err != nil {
		return err
	}
	_, err = s.raw.ExecContext(ctx, `DELETE FROM credit_refund_funding_guards
		WHERE request_id=? AND kind='upgrade'`, requestID)
	return err
}
