package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/provider/store"
)

var testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// newStore opens a throwaway SQLite database, applies the embedded migrations (0003
// included), and seeds a user — selections reference users.
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
	users := authstore.New(handle.Writer, handle.Reader)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(context.Background(), auth.User{ID: id, PasswordHash: "hash", CreatedAt: testNow}); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}
	return store.New(handle.Writer, handle.Reader)
}

func TestSelectionsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	free := llm.ModelRef{ProviderID: "openrouter", ModelID: "openrouter/free"}
	paid := llm.ModelRef{ProviderID: "openrouter", ModelID: "anthropic/claude-sonnet-4.5"}

	if err := s.UpsertSelection(ctx, "alice", provider.Selection{Stage: provider.StageObserve, Ref: free, UpdatedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSelection(ctx, "alice", provider.Selection{Stage: provider.StageWrite, Ref: free, UpdatedAt: testNow}); err != nil {
		t.Fatal(err)
	}
	// AC4: the same account reads the same choice anywhere; a second save replaces.
	later := testNow.Add(time.Minute)
	if err := s.UpsertSelection(ctx, "alice", provider.Selection{Stage: provider.StageWrite, Ref: paid, UpdatedAt: later}); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListSelections(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("selections = %+v", got)
	}
	byStage := map[provider.Stage]provider.Selection{}
	for _, sel := range got {
		byStage[sel.Stage] = sel
	}
	if byStage[provider.StageObserve].Ref != free || byStage[provider.StageWrite].Ref != paid {
		t.Errorf("refs = %+v", byStage)
	}
	if !byStage[provider.StageWrite].UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt = %v, want %v", byStage[provider.StageWrite].UpdatedAt, later)
	}

	// Scoped by user.
	if other, _ := s.ListSelections(ctx, "bob"); len(other) != 0 {
		t.Errorf("bob sees alice's selections: %+v", other)
	}

	// The clear is conditional on the ref: a choice made after the read survives.
	if err := s.DeleteSelection(ctx, "alice", provider.Selection{Stage: provider.StageWrite, Ref: free}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListSelections(ctx, "alice")
	if len(got) != 2 {
		t.Fatalf("a clear for a stale ref removed the live row: %+v", got)
	}

	if err := s.DeleteSelection(ctx, "alice", provider.Selection{Stage: provider.StageWrite, Ref: paid}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListSelections(ctx, "alice")
	if len(got) != 1 || got[0].Stage != provider.StageObserve {
		t.Errorf("after delete = %+v", got)
	}
	// Deleting what is not there is not an error.
	if err := s.DeleteSelection(ctx, "alice", provider.Selection{Stage: provider.StageWrite, Ref: paid}); err != nil {
		t.Errorf("second delete = %v", err)
	}
}
