package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// clipWriteTx coordinates context-owned operations on the single writer. No
// provider, media, storage or cleanup call belongs inside this callback.
func clipWriteTx(ctx context.Context, writer *sql.DB, fn func(*sql.Conn) error) error {
	conn, err := writer.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "ROLLBACK")
	}()
	if err = fn(conn); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return err
}

type clipGuard struct {
	writer    *sql.DB
	admission jobAdmission
}

func (a clipGuard) Reserve(ctx context.Context, start job.Start) error {
	return clipWriteTx(ctx, a.writer, func(conn *sql.Conn) error {
		j, err := jobstore.NewTx(conn).GetByID(ctx, start.JobID)
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
		admission.ledger = admission.ledger.WithStore(usagestore.NewTx(conn))
		return admission.Hold(ctx, start)
	})
}

func (a clipGuard) Authorize(ctx context.Context, user, id string) error {
	// The conditional writer statement serializes authorization with cancellation.
	return jobstore.New(a.writer, a.writer).AuthorizeClipDispatch(ctx, user, id)
}
