package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

const exportFundingPredicate = `w.user_id=? AND w.coverage_id=? AND
	((?='upgrade' AND EXISTS (SELECT 1 FROM server_export_adjustments a
		WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id AND a.window_start=w.window_start
		AND a.correlation_id=? AND a.refunded_at IS NULL))
	OR (? IN ('subscribe','renew') AND w.window_start>=? AND w.window_start<?))`

func exportFundingArgs(f clip.RefundFunding) []any {
	return []any{f.UserID, f.CoverageID, f.Kind, f.OrderID, f.Kind, stamp(f.Start), stamp(f.End)}
}

func (s *Store) RefundFundingEvidence(ctx context.Context, f clip.RefundFunding) (clip.RefundFundingEvidence, error) {
	rows, err := s.raw.QueryContext(ctx, `SELECT w.allowance,w.used,w.reserved,
		COALESCE((SELECT SUM(a.allowance_delta) FROM server_export_adjustments a
			WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id
			AND a.window_start=w.window_start AND a.refunded_at IS NULL),0),
		COALESCE((SELECT a.allowance_delta FROM server_export_adjustments a
			WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id
			AND a.window_start=w.window_start AND a.correlation_id=? AND a.refunded_at IS NULL),0),
		COALESCE((SELECT SUM(b.allowance_delta) FROM server_export_adjustments b
			WHERE b.user_id=w.user_id AND b.coverage_id=w.coverage_id
			AND b.window_start=w.window_start AND b.refunded_at IS NULL
			AND b.rowid<(SELECT rowid FROM server_export_adjustments c WHERE c.correlation_id=?)),0)
		FROM server_export_windows w WHERE `+exportFundingPredicate,
		append([]any{f.OrderID, f.OrderID}, exportFundingArgs(f)...)...)
	if err != nil {
		return clip.RefundFundingEvidence{}, err
	}
	defer rows.Close()
	var evidence clip.RefundFundingEvidence
	for rows.Next() {
		var allowance, used, reserved, allUpgrades, thisUpgrade, priorUpgrades int
		if err := rows.Scan(&allowance, &used, &reserved, &allUpgrades, &thisUpgrade, &priorUpgrades); err != nil {
			return evidence, err
		}
		funded := allowance - allUpgrades
		prior := 0
		if f.Kind == "upgrade" {
			funded = thisUpgrade
			prior = allowance - allUpgrades + priorUpgrades
		}
		if funded < 0 {
			funded = 0
		}
		fundedUsed := min(funded, max(0, used-prior))
		fundedReserved := min(max(0, funded-fundedUsed), max(0, used+reserved-prior-fundedUsed))
		evidence.ExportsUsed += fundedUsed
		evidence.ExportsReserved += fundedReserved
		evidence.ExportsRemaining += max(0, funded-fundedUsed-fundedReserved)
	}
	return evidence, rows.Err()
}

func (s *Store) GuardRefundFunding(ctx context.Context, f clip.RefundFunding, requestID string) error {
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
	rows, err := s.raw.QueryContext(ctx, `SELECT w.user_id,w.coverage_id,w.window_start,w.allowance,w.used,w.reserved,
		COALESCE((SELECT SUM(a.allowance_delta) FROM server_export_adjustments a
			WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id
			AND a.window_start=w.window_start AND a.refunded_at IS NULL),0),
		COALESCE((SELECT a.allowance_delta FROM server_export_adjustments a
			WHERE a.user_id=w.user_id AND a.coverage_id=w.coverage_id
			AND a.window_start=w.window_start AND a.correlation_id=? AND a.refunded_at IS NULL),0)
		FROM server_export_windows w WHERE w.refund_request_id=?`, f.OrderID, requestID)
	if err != nil {
		return err
	}
	type window struct {
		user, coverage, start                               string
		allowance, used, reserved, allUpgrades, thisUpgrade int
	}
	var windows []window
	for rows.Next() {
		var w window
		if err := rows.Scan(&w.user, &w.coverage, &w.start, &w.allowance, &w.used, &w.reserved, &w.allUpgrades, &w.thisUpgrade); err != nil {
			rows.Close()
			return err
		}
		windows = append(windows, w)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, w := range windows {
		funded := w.allowance - w.allUpgrades
		if f.Kind == "upgrade" {
			funded = w.thisUpgrade
		}
		remaining := max(w.used+w.reserved, w.allowance-max(0, funded))
		if _, err := s.raw.ExecContext(ctx, `UPDATE server_export_windows SET allowance=?,refund_request_id=NULL
			WHERE user_id=? AND coverage_id=? AND window_start=? AND refund_request_id=?`,
			remaining, w.user, w.coverage, w.start, requestID); err != nil {
			return err
		}
	}
	if f.Kind == "upgrade" {
		_, err = s.raw.ExecContext(ctx, `UPDATE server_export_adjustments SET refunded_at=?
			WHERE correlation_id=? AND refunded_at IS NULL`, stamp(at), f.OrderID)
		if err != nil {
			return err
		}
		_, err = s.raw.ExecContext(ctx, `DELETE FROM server_export_refund_guards WHERE request_id=?`, requestID)
	}
	return err
}
