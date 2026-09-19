package app

import (
	"context"
	"database/sql"
	"errors"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// Guard takes the credit hold for a charged clip job in the same transaction that proves
// the job is still the running, unreserved `prepare`-stage job the approval described.
type Guard struct {
	writer     *sql.DB
	bind       Binder
	authorizer Authorizer
}

func NewGuard(writer *sql.DB, bind Binder, authorizer Authorizer) Guard {
	if writer == nil || bind == nil || authorizer == nil {
		panic("clip app: guard needs writer, binder and authorizer")
	}
	return Guard{writer: writer, bind: bind, authorizer: authorizer}
}

// Reservable is the state guard: only a running charged clip job in its
// prepare stage, not yet asked to cancel, under the policy the start names,
// may take a hold.
func Reservable(j job.Job, hold Hold) bool {
	return j.UserID == hold.UserID && clip.ChargedJobKind(j.Kind) && j.Status == job.StatusRunning && j.Stage == "prepare" && j.CancelRequestedAt == nil && j.CancellationPolicyVersion == hold.Reservation.CancellationPolicyVersion
}

func (g Guard) Reserve(ctx context.Context, hold Hold) error {
	return WriteTx(ctx, g.writer, g.bind, func(p Ports) error {
		j, err := p.Jobs.GetByID(ctx, hold.JobID)
		if err != nil {
			return err
		}
		if !Reservable(j, hold) {
			return clip.ErrCreditAllowance
		}
		if p.Admission == nil {
			return errors.New("clip ledger unavailable")
		}
		return p.Admission.Hold(ctx, hold)
	})
}

// Authorize serializes dispatch authorization with cancellation through the
// job store's conditional writer statement.
func (g Guard) Authorize(ctx context.Context, user, id string) error {
	return g.authorizer.AuthorizeDispatch(ctx, user, id)
}
