package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

// F5: every metered provider call reads its job's admission. Outside a transaction that read
// goes to the read pool, so it returns while another write transaction holds the single
// writer connection instead of queueing behind it.
func TestHoldForJobOutsideATransactionDoesNotWaitOnTheWriter(t *testing.T) {
	_, handle := newServiceWithDB(t)
	store := usagestore.New(handle.Writer, handle.Reader)
	ctx := context.Background()
	if err := store.InsertAdmission(ctx, usage.Admission{UserID: "alice", Kind: "generate", JobID: "job", HoldCredits: 5, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	busy, err := handle.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Rollback() }()

	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	admission, _, found, err := store.HoldForJob(bounded, "job")
	if err != nil || !found || admission.HoldCredits != 5 {
		t.Fatalf("hold read while the writer is busy = %+v %v %v", admission, found, err)
	}
}

// The re-check that refuses a second hold runs inside the writer's transaction and must read
// what that transaction wrote: a transaction-bound store reads inside its transaction.
func TestHoldForJobInsideATransactionReadsItsOwnWrites(t *testing.T) {
	_, handle := newServiceWithDB(t)
	ctx := context.Background()
	errUndo := errors.New("undo")
	check := func(tx usage.Storage) error {
		if err := tx.InsertAdmission(ctx, usage.Admission{UserID: "alice", Kind: "generate", JobID: "job", HoldCredits: 5, CreatedAt: time.Now()}); err != nil {
			return err
		}
		if _, _, found, err := tx.HoldForJob(ctx, "job"); err != nil || !found {
			t.Errorf("the transaction did not see its own admission: %v %v", found, err)
		}
		return errUndo
	}
	if err := usagestore.New(handle.Writer, handle.Reader).InWriteTx(ctx, check); !errors.Is(err, errUndo) {
		t.Fatalf("InWriteTx: %v", err)
	}
	tx, err := handle.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := check(usagestore.NewTx(tx)); !errors.Is(err, errUndo) {
		t.Fatalf("NewTx: %v", err)
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
