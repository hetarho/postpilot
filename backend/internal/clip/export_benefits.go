package clip

import (
	"context"
	"time"
)

// ExportWindow is one anchored monthly allowance. Reservation and successful-use
// mutations join this aggregate in T482.
type ExportWindow struct {
	UserID, CoverageID        string
	Start, End                time.Time
	Allowance, Used, Reserved int
}

type ExportWindows interface {
	OpenExportWindow(ctx context.Context, window ExportWindow, correlationID string) error
	RaiseExportWindow(ctx context.Context, window ExportWindow, delta int, correlationID string) error
	CurrentExportWindow(ctx context.Context, userID string, at time.Time) (ExportWindow, bool, error)
}
