package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
)

func TestSaveSelectionsIsAtomicAndActiveReadsStayCompatible(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	store := providerstore.New(handle.Writer, handle.Reader)
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	original := provider.Selection{Stage: provider.StageWrite, Slot: provider.SlotActive, Ref: llm.ModelRef{ProviderID: "p", ModelID: "original"}, UpdatedAt: now}
	if err := store.UpsertSelection(ctx, "alice", original); err != nil {
		t.Fatal(err)
	}
	badBatch := []provider.Selection{
		{Stage: provider.StageWrite, Slot: provider.SlotCandidateA, Ref: llm.ModelRef{ProviderID: "p", ModelID: "a"}, UpdatedAt: now},
		{Stage: provider.StageWrite, Slot: provider.SelectionSlot("invalid"), Ref: llm.ModelRef{ProviderID: "p", ModelID: "b"}, UpdatedAt: now},
	}
	if err := store.SaveSelections(ctx, "alice", badBatch); err == nil {
		t.Fatal("invalid second row did not fail")
	}
	rows, err := store.ListSelectionSlots(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Slot != provider.SlotActive || rows[0].Ref != original.Ref {
		t.Fatalf("partial batch or active regression: %+v", rows)
	}

	good := []provider.Selection{
		{Stage: provider.StageWrite, Slot: provider.SlotCandidateA, Ref: llm.ModelRef{ProviderID: "p", ModelID: "a"}, UpdatedAt: now},
		{Stage: provider.StageWrite, Slot: provider.SlotCandidateB, Ref: llm.ModelRef{ProviderID: "p", ModelID: "b"}, UpdatedAt: now},
	}
	if err := store.SaveSelections(ctx, "alice", good); err != nil {
		t.Fatal(err)
	}
	active, err := store.ListSelections(ctx, "alice")
	if err != nil || len(active) != 1 || active[0].Ref != original.Ref {
		t.Fatalf("active compatibility = %+v err=%v", active, err)
	}
	rows, err = store.ListSelectionSlots(ctx, "alice")
	if err != nil || len(rows) != 3 {
		t.Fatalf("slot reload = %+v err=%v", rows, err)
	}
}
