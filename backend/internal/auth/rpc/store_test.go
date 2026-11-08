package rpc_test

import (
	"testing"

	"github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/platform/db/dbtest"
)

// newStore opens a throwaway SQLite database, applies the embedded migrations, and
// returns the real store. t.TempDir is removed automatically, so each test gets a
// clean schema built by the same migrations that run in production.
func newStore(t *testing.T) *store.Store {
	t.Helper()

	handle := dbtest.Open(t)

	return store.New(handle.Writer, handle.Reader)
}
