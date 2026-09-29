package clip

import (
	"context"
	"errors"
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

var ErrExportAllowance = errors.New("server export allowance exhausted")

// ExportAllowanceError carries the origin benefit month's exact balance and renewal.
type ExportAllowanceError struct{ Window ExportWindow }

func (e *ExportAllowanceError) Error() string { return ErrExportAllowance.Error() }
func (e *ExportAllowanceError) Unwrap() error { return ErrExportAllowance }

// ExportReservations is owned by clip. A durable id is allocated before a render
// job is queued; the origin window stays attached across renewals and retries.
type ExportReservations interface {
	ReserveExport(context.Context, string, string, int, string, time.Time) error
	BindExport(context.Context, string, string) error
	CommitExport(context.Context, AttemptResult) error
	ReleaseExport(context.Context, string) error
	RecoverExports(context.Context) error
}
