package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// exportFundingPredicate selects the export windows a refunded payment funded from the fields
// of the funding value billing computed, and from nothing else: the coverage's windows starting
// in [Start, End), and the window carrying the export adjustment correlated to the order. The
// window trigger server_export_refund_block_future_window (migration 0130) applies the window
// half to a window about to open, over the guard row GuardRefundFunding writes.
const exportFundingPredicate = `w.user_id=? AND ?<>'' AND w.coverage_id=? AND
	((?<>'' AND EXISTS (SELECT 1 FROM server_export_adjustments a
		WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id AND a.window_start=w.window_start
		AND a.correlation_id=? AND a.refunded_at IS NULL))
	OR (w.window_start>=? AND w.window_start<?))`

func exportFundingArgs(f clip.RefundFunding) []any {
	return []any{f.UserID, f.CoverageID, f.CoverageID, f.Correlation, f.Correlation, stamp(f.Start), stamp(f.End)}
}

// fundedWindowColumns reads what the funded arithmetic needs of one window: its counters, every
// live upgrade adjustment, this funding's own adjustment and the adjustments made before it.
// It takes the funding's correlation twice.
const fundedWindowColumns = `w.user_id,w.coverage_id,w.window_start,w.allowance,w.used,w.reserved,
	COALESCE((SELECT SUM(a.allowance_delta) FROM server_export_adjustments a
		WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id
		AND a.window_start=w.window_start AND a.refunded_at IS NULL),0),
	COALESCE((SELECT a.allowance_delta FROM server_export_adjustments a
		WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id
		AND a.window_start=w.window_start AND a.correlation_id=? AND a.refunded_at IS NULL),0),
	COALESCE((SELECT SUM(b.allowance_delta) FROM server_export_adjustments b
		WHERE b.user_id=w.user_id AND b.coverage_id=w.coverage_id
		AND b.window_start=w.window_start AND b.refunded_at IS NULL
		AND b.rowid<(SELECT rowid FROM server_export_adjustments c WHERE c.correlation_id=?)),0)`

type fundedWindow struct {
	user, coverage, start                                              string
	allowance, used, reserved, allUpgrades, thisUpgrade, priorUpgrades int
}

// funding is the one funded arithmetic of a window: a window carrying the payment's own
// adjustment funded that adjustment, on top of the base allowance and the upgrades before it;
// any other funded window funded its base allowance, every upgrade on it excluded.
func (w fundedWindow) funding() (funded, prior int) {
	if w.thisUpgrade > 0 {
		return w.thisUpgrade, w.allowance - w.allUpgrades + w.priorUpgrades
	}
	return max(0, w.allowance-w.allUpgrades), 0
}

// evidence splits the funded exports into used, reserved and still remaining, counting the
// window's use against what it held before the payment first.
func (w fundedWindow) evidence() (used, reserved, remaining int) {
	funded, prior := w.funding()
	used = min(funded, max(0, w.used-prior))
	reserved = min(max(0, funded-used), max(0, w.used+w.reserved-prior-used))
	return used, reserved, max(0, funded-used-reserved)
}

// voidedAllowance is the window's allowance once the funded exports nobody used or reserved
// are taken back.
func (w fundedWindow) voidedAllowance() int {
	funded, _ := w.funding()
	return max(w.used+w.reserved, w.allowance-funded)
}

func (s *Store) fundedWindows(ctx context.Context, f clip.RefundFunding, where string, args ...any) ([]fundedWindow, error) {
	rows, err := s.raw.QueryContext(ctx, `SELECT `+fundedWindowColumns+` FROM server_export_windows w WHERE `+where,
		append([]any{f.Correlation, f.Correlation}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var windows []fundedWindow
	for rows.Next() {
		var w fundedWindow
		if err := rows.Scan(&w.user, &w.coverage, &w.start, &w.allowance, &w.used, &w.reserved,
			&w.allUpgrades, &w.thisUpgrade, &w.priorUpgrades); err != nil {
			return nil, err
		}
		windows = append(windows, w)
	}
	return windows, rows.Err()
}

func (s *Store) RefundFundingEvidence(ctx context.Context, f clip.RefundFunding) (clip.RefundFundingEvidence, error) {
	windows, err := s.fundedWindows(ctx, f, exportFundingPredicate, exportFundingArgs(f)...)
	if err != nil {
		return clip.RefundFundingEvidence{}, err
	}
	var evidence clip.RefundFundingEvidence
	for _, w := range windows {
		used, reserved, remaining := w.evidence()
		evidence.ExportsUsed += used
		evidence.ExportsReserved += reserved
		evidence.ExportsRemaining += remaining
	}
	return evidence, nil
}

// GuardRefundFunding freezes the windows the payment funded and records the guard row the
// window trigger reads; a payment that funded no coverage window (a pack) guards nothing.
func (s *Store) GuardRefundFunding(ctx context.Context, f clip.RefundFunding, requestID string) error {
	if f.CoverageID == "" {
		return nil
	}
	_, err := s.raw.ExecContext(ctx, `INSERT INTO server_export_refund_guards
		(request_id,user_id,order_id,kind,coverage_id,starts_at,ends_at) VALUES (?,?,?,?,?,?,?)`,
		requestID, f.UserID, f.OrderID, f.Kind, f.CoverageID, stamp(f.Start), stamp(f.End))
	if err != nil {
		return err
	}
	_, err = s.raw.ExecContext(ctx, `UPDATE server_export_windows SET refund_request_id=?
		WHERE rowid IN (SELECT w.rowid FROM server_export_windows w WHERE `+exportFundingPredicate+`)
		AND refund_request_id IS NULL`, append([]any{requestID}, exportFundingArgs(f)...)...)
	return err
}

func (s *Store) ReleaseRefundFunding(ctx context.Context, requestID string) error {
	if _, err := s.raw.ExecContext(ctx, `DELETE FROM server_export_refund_guards WHERE request_id=?`, requestID); err != nil {
		return err
	}
	_, err := s.raw.ExecContext(ctx, `UPDATE server_export_windows SET refund_request_id=NULL WHERE refund_request_id=?`, requestID)
	return err
}

func (s *Store) ConfirmRefundFunding(ctx context.Context, f clip.RefundFunding, requestID string, at time.Time) error {
	windows, err := s.fundedWindows(ctx, f, `w.refund_request_id=?`, requestID)
	if err != nil {
		return err
	}
	for _, w := range windows {
		if _, err := s.raw.ExecContext(ctx, `UPDATE server_export_windows SET allowance=?,refund_request_id=NULL
			WHERE user_id=? AND coverage_id=? AND window_start=? AND refund_request_id=?`,
			w.voidedAllowance(), w.user, w.coverage, w.start, requestID); err != nil {
			return err
		}
	}
	if f.Correlation != "" {
		if _, err := s.raw.ExecContext(ctx, `UPDATE server_export_adjustments SET refunded_at=?
			WHERE correlation_id=? AND refunded_at IS NULL`, stamp(at), f.Correlation); err != nil {
			return err
		}
	}
	if f.Kind == "upgrade" {
		_, err = s.raw.ExecContext(ctx, `DELETE FROM server_export_refund_guards WHERE request_id=?`, requestID)
	}
	return err
}
