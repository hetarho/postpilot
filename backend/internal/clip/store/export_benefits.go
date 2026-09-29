package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func (s *Store) OpenExportWindow(ctx context.Context, window clip.ExportWindow, correlationID string) error {
	if window.UserID == "" || window.CoverageID == "" || !window.Start.Before(window.End) || window.Allowance < 0 {
		return errors.New("open export window: invalid entitlement")
	}
	_, err := s.write.InsertExportWindow(ctx, sqlc.InsertExportWindowParams{
		UserID: window.UserID, CoverageID: window.CoverageID,
		WindowStart: stamp(window.Start), WindowEnd: stamp(window.End),
		Allowance: int64(window.Allowance), CorrelationID: nullable(correlationID),
	})
	if err != nil {
		return fmt.Errorf("open export window: %w", err)
	}
	return nil
}

func (s *Store) RaiseExportWindow(ctx context.Context, window clip.ExportWindow, delta int, correlationID string) error {
	if delta < 0 || correlationID == "" {
		return errors.New("raise export window: invalid delta or correlation")
	}
	if err := s.OpenExportWindow(ctx, window, ""); err != nil {
		return err
	}
	if delta == 0 {
		return nil
	}
	created, err := s.write.InsertExportAdjustment(ctx, sqlc.InsertExportAdjustmentParams{
		CorrelationID: correlationID, UserID: window.UserID, CoverageID: window.CoverageID,
		WindowStart: stamp(window.Start), AllowanceDelta: int64(delta),
	})
	if err != nil {
		return fmt.Errorf("record export adjustment: %w", err)
	}
	if created == 0 {
		return nil
	}
	updated, err := s.write.RaiseExportAllowance(ctx, sqlc.RaiseExportAllowanceParams{
		Allowance: int64(delta), UserID: window.UserID, CoverageID: window.CoverageID,
		WindowStart: stamp(window.Start),
	})
	if err != nil {
		return fmt.Errorf("raise export allowance: %w", err)
	}
	if updated != 1 {
		return errors.New("raise export allowance: window missing")
	}
	return nil
}

func (s *Store) CurrentExportWindow(ctx context.Context, userID string, at time.Time) (clip.ExportWindow, bool, error) {
	row, err := s.read.CurrentExportWindow(ctx, sqlc.CurrentExportWindowParams{
		UserID: userID, WindowStart: stamp(at), WindowEnd: stamp(at),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return clip.ExportWindow{}, false, nil
	}
	if err != nil {
		return clip.ExportWindow{}, false, fmt.Errorf("read export window: %w", err)
	}
	start, err := time.Parse(timeLayout, row.WindowStart)
	if err != nil {
		return clip.ExportWindow{}, false, err
	}
	end, err := time.Parse(timeLayout, row.WindowEnd)
	if err != nil {
		return clip.ExportWindow{}, false, err
	}
	return clip.ExportWindow{UserID: row.UserID, CoverageID: row.CoverageID,
		Start: start, End: end, Allowance: int(row.Allowance), Used: int(row.Used),
		Reserved: int(row.Reserved)}, true, nil
}

var _ clip.ExportWindows = (*Store)(nil)
