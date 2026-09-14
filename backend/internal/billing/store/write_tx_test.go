package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/billing"
	billingstore "github.com/postpilot/backend/internal/billing/store"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
	usagestore "github.com/postpilot/backend/internal/usage/store"
)

// The billing writer carries the same guarantee as the usage one: a caller whose context dies
// before the COMMIT must not hand the pool's single connection back mid-transaction, or every
// later write in the process fails until a restart.
func TestInWriteTxSurvivesACancelledCommit(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "billing-write-tx.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}

	store := billingstore.New(handle.Writer, handle.Reader)
	store.SetCreditsForTx(func(tx *sql.Tx) billing.Credits {
		return usage.NewService(usagestore.NewTx(tx), nil, 0)
	})
	store.SetPlansForTx(func(tx *sql.Tx) billing.Plans {
		return auth.NewService(authstore.NewTx(tx), time.Hour)
	})

	cancelled, cancel := context.WithCancel(ctx)
	noop := func(billing.Store, billing.Credits, billing.Plans) error { return nil }
	if err := store.InWriteTx(cancelled, func(billing.Store, billing.Credits, billing.Plans) error {
		cancel()
		return nil
	}); err == nil {
		t.Fatal("a commit on a cancelled context reported success")
	}

	if err := store.InWriteTx(ctx, noop); err != nil {
		t.Fatalf("the next billing transaction inherited the abandoned one: %v", err)
	}
}
