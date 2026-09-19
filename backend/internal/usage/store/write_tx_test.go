package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// A cancelled caller must not strand the writer inside a transaction. The writer pool holds
// one connection, so the stranded one came back to the pool still open and every later write
// in the process failed with "cannot start a transaction within a transaction" — in prod that
// took out plan reads, draft saves and image uploads at once until the container restarted.
func TestInWriteTxSurvivesACancelledCaller(t *testing.T) {
	_, handle := newServiceWithDB(t)
	store := usagestore.New(handle.Writer, handle.Reader)

	ctx, cancel := context.WithCancel(context.Background())
	err := store.InWriteTx(ctx, func(usage.Storage) error {
		// What a disconnecting browser does: the request context dies mid-transaction, so
		// the rollback cannot be issued on it either.
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InWriteTx err = %v, want context.Canceled", err)
	}

	if err := store.InWriteTx(context.Background(), func(usage.Storage) error { return nil }); err != nil {
		t.Fatalf("the next write transaction inherited the abandoned one: %v", err)
	}
}

// The same guarantee when it is the COMMIT that the cancellation catches.
func TestInWriteTxSurvivesACancelledCommit(t *testing.T) {
	_, handle := newServiceWithDB(t)
	store := usagestore.New(handle.Writer, handle.Reader)

	ctx, cancel := context.WithCancel(context.Background())
	if err := store.InWriteTx(ctx, func(usage.Storage) error {
		cancel()
		return nil
	}); err == nil {
		t.Fatal("a commit on a cancelled context reported success")
	}

	if err := store.InWriteTx(context.Background(), func(usage.Storage) error { return nil }); err != nil {
		t.Fatalf("the next write transaction inherited the abandoned one: %v", err)
	}
}
