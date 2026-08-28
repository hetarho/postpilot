package rpc_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/platform/db"
)

// newStore opens a throwaway SQLite database, applies the embedded migrations, and
// returns the real store. t.TempDir is removed automatically, so each test gets a
// clean schema built by the same migrations that run in production.
func newStore(t *testing.T) *store.Store {
	t.Helper()

	handle, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })

	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(handle.Writer, handle.Reader)
}
