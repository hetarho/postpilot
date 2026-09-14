package main

import (
	"context"
	"database/sql"
	"errors"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// clipWriteTx coordinates context-owned operations on the single writer. No
// provider, media, storage or cleanup call belongs inside this callback.
func clipWriteTx(ctx context.Context, writer *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// A no-op once Commit has landed, and the undo when it has not. database/sql also rolls
	// back on its own when ctx is cancelled, which is the part a hand-written ROLLBACK on
	// that same cancelled context could never do.
	defer func() { _ = tx.Rollback() }()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type clipGuard struct {
	writer    *sql.DB
	admission jobAdmission
}

func (a clipGuard) Reserve(ctx context.Context, start job.Start) error {
	return clipWriteTx(ctx, a.writer, func(tx *sql.Tx) error {
		j, err := jobstore.NewTx(tx).GetByID(ctx, start.JobID)
		if err != nil {
			return err
		}
		if start.Clip == nil || j.UserID != start.UserID || j.Kind != job.KindGenerateClip || j.Status != job.StatusRunning || j.Stage != "prepare" || j.CancelRequestedAt != nil || j.CancellationPolicyVersion != start.Clip.CancellationPolicyVersion {
			return job.ErrCreditAllowance
		}
		admission := a.admission
		if admission.ledger == nil {
			return errors.New("clip ledger unavailable")
		}
		admission.ledger = admission.ledger.WithStore(usagestore.NewTx(tx))
		return admission.Hold(ctx, start)
	})
}

func (a clipGuard) Authorize(ctx context.Context, user, id string) error {
	// The conditional writer statement serializes authorization with cancellation.
	return jobstore.New(a.writer, a.writer).AuthorizeClipDispatch(ctx, user, id)
}
