package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/platform/db"
)

func TestRetirementSnapshotSupportsAnEmptyInstallation(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	snapshot, err := New(handle.Writer, handle.Reader).RetirementSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CutoffAt == "" || len(snapshot.Agents) != 0 || len(snapshot.Jobs) != 0 || snapshot.Pairings != 0 || snapshot.Reservations != 0 || snapshot.Assets != 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}
