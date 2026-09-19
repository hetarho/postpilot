package app

import (
	"context"
	"database/sql"
	"errors"
)

// WriteTx runs fn over the ports of one writer transaction. No provider,
// media, storage or cleanup call belongs inside fn (ARCH-10).
func WriteTx(ctx context.Context, writer *sql.DB, bind Binder, fn func(Ports) error) error {
	if writer == nil || bind == nil {
		return errors.New("clip app: writer and binder are required")
	}
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// A no-op once Commit has landed, and the undo when it has not. database/sql
	// also rolls back on its own when ctx is cancelled, which is the part a
	// hand-written ROLLBACK on that same cancelled context could never do.
	defer func() { _ = tx.Rollback() }()
	if err = fn(bind(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
