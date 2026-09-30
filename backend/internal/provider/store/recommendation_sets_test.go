package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
)

func openRecommendationStore(t *testing.T) (*providerstore.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "provider.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	return providerstore.New(handle.Writer, handle.Reader), handle
}

func storedSet(id, label, model string) provider.RecommendationSet {
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: model}
	other := llm.ModelRef{ProviderID: "openrouter", ModelID: model + "-b"}
	return provider.RecommendationSet{ID: id, Label: label, Selections: []provider.RecommendationStageSelection{
		{Stage: provider.StageObserve, Active: ref, CandidateA: ref, CandidateB: other},
		{Stage: provider.StageAnalyze, Active: ref},
		{Stage: provider.StageWrite, Active: other, CandidateA: other, CandidateB: ref},
	}}
}

func setIDs(t *testing.T, store *providerstore.Store) []string {
	t.Helper()
	sets, err := store.ListRecommendationSets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(sets))
	for _, set := range sets {
		ids = append(ids, set.ID)
	}
	return ids
}

// The seeded set comes first, a created one is appended after it with all seven slots, and a
// replace rewrites the label and every slot of that set alone.
func TestRecommendationSetsRoundTripInOrder(t *testing.T) {
	store, _ := openRecommendationStore(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

	if err := store.CreateRecommendationSet(ctx, storedSet("mine", "Mine", "m"), provider.MaxRecommendationSets, at); err != nil {
		t.Fatal(err)
	}
	sets, err := store.ListRecommendationSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].ID != "balanced-2026-08" || sets[1].ID != "mine" {
		t.Fatalf("sets = %+v", sets)
	}
	if got, want := sets[1], storedSet("mine", "Mine", "m"); len(got.Selections) != 3 ||
		got.Selections[0] != want.Selections[0] || got.Selections[1] != want.Selections[1] || got.Selections[2] != want.Selections[2] {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}

	if err := store.ReplaceRecommendationSet(ctx, storedSet("mine", "Renamed", "n"), at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	sets, err = store.ListRecommendationSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sets[1].Label != "Renamed" || sets[1].Selections[1].Active.ModelID != "n" {
		t.Fatalf("replaced = %+v", sets[1])
	}
	if sets[0].Selections[1].Active.ModelID != "openai/gpt-5.6-luna" {
		t.Fatalf("replace reached another set: %+v", sets[0])
	}
	if err := store.ReplaceRecommendationSet(ctx, storedSet("nobody", "Ghost", "g"), at); !errors.Is(err, provider.ErrRecommendationNotFound) {
		t.Fatalf("replace unknown = %v", err)
	}
}

// The limit is counted inside the write, the seeded set included.
func TestCreateRecommendationSetStopsAtTheLimit(t *testing.T) {
	store, _ := openRecommendationStore(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	if err := store.CreateRecommendationSet(ctx, storedSet("second", "Second", "s"), 2, at); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRecommendationSet(ctx, storedSet("third", "Third", "t"), 2, at); !errors.Is(err, provider.ErrRecommendationLimit) {
		t.Fatalf("third of two = %v", err)
	}
	if ids := setIDs(t, store); len(ids) != 2 {
		t.Fatalf("ids = %v", ids)
	}
}

// A move swaps neighbours and renumbers; at either end it changes nothing. Delete takes the
// slots with the set. None of it touches model_selections (MODEL-71).
func TestMoveAndDeleteRecommendationSets(t *testing.T) {
	store, handle := openRecommendationStore(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	kept := provider.Selection{Stage: provider.StageWrite, Slot: provider.SlotActive, Ref: llm.ModelRef{ProviderID: "openrouter", ModelID: "kept"}, UpdatedAt: at}
	if err := store.UpsertSelection(ctx, "alice", kept); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"b", "c"} {
		if err := store.CreateRecommendationSet(ctx, storedSet(id, id, id), provider.MaxRecommendationSets, at); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.MoveRecommendationSet(ctx, "c", true); err != nil {
		t.Fatal(err)
	}
	if err := store.MoveRecommendationSet(ctx, "c", true); err != nil {
		t.Fatal(err)
	}
	if err := store.MoveRecommendationSet(ctx, "c", true); err != nil {
		t.Fatalf("moving the top set up: %v", err)
	}
	if err := store.MoveRecommendationSet(ctx, "b", false); err != nil {
		t.Fatalf("moving the bottom set down: %v", err)
	}
	if ids := setIDs(t, store); len(ids) != 3 || ids[0] != "c" || ids[1] != "balanced-2026-08" || ids[2] != "b" {
		t.Fatalf("order = %v, want c balanced b", ids)
	}
	if err := store.MoveRecommendationSet(ctx, "nobody", true); !errors.Is(err, provider.ErrRecommendationNotFound) {
		t.Fatalf("move unknown = %v", err)
	}

	if err := store.DeleteRecommendationSet(ctx, "c"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRecommendationSet(ctx, "c"); !errors.Is(err, provider.ErrRecommendationNotFound) {
		t.Fatalf("delete twice = %v", err)
	}
	var slots int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM recommendation_set_slots WHERE set_id = 'c'`).Scan(&slots); err != nil || slots != 0 {
		t.Fatalf("slots left = %d, %v", slots, err)
	}

	selections, err := store.ListSelectionSlots(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(selections) != 1 || selections[0].Ref != kept.Ref {
		t.Fatalf("selections = %+v", selections)
	}
}

// MODEL-73: the document replaces the whole list inside its own transaction. A listed stored
// id keeps its row, an unlisted one is deleted, a new id is inserted, and positions follow the
// list.
func TestReplaceRecommendationSetsInsideACallersTransaction(t *testing.T) {
	store, handle := openRecommendationStore(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	if err := store.CreateRecommendationSet(ctx, storedSet("gone", "Gone", "g"), provider.MaxRecommendationSets, at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	kept := provider.Selection{Stage: provider.StageObserve, Slot: provider.SlotActive, Ref: llm.ModelRef{ProviderID: "openrouter", ModelID: "google/gemini-3.7-flash"}, UpdatedAt: at}
	if err := store.UpsertSelection(ctx, "alice", kept); err != nil {
		t.Fatal(err)
	}

	tx, err := handle.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows := providerstore.NewTx(tx)
	seeded, err := rows.ListRecommendationSets(ctx)
	if err != nil || len(seeded) != 2 {
		t.Fatalf("list in tx = %+v, %v", seeded, err)
	}
	renamed := seeded[0]
	renamed.Label = "Balanced, retuned"
	renamed.Selections[1].Active.ModelID = "vendor/other"
	if err := rows.ReplaceRecommendationSets(ctx, []provider.RecommendationSet{storedSet("fresh", "Fresh", "f"), renamed}, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	sets, err := store.ListRecommendationSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].ID != "fresh" || sets[1].ID != "balanced-2026-08" ||
		sets[1].Label != "Balanced, retuned" || sets[1].Selections[1].Active.ModelID != "vendor/other" {
		t.Fatalf("sets = %+v", sets)
	}
	var created string
	if err := handle.Reader.QueryRow(`SELECT created_at FROM recommendation_sets WHERE id = 'balanced-2026-08'`).Scan(&created); err != nil || created != "2026-09-30T00:00:00.000000000Z" {
		t.Fatalf("a kept set lost its creation time: %q, %v", created, err)
	}
	selections, err := store.ListSelectionSlots(ctx, "alice")
	if err != nil || len(selections) != 1 || selections[0].Ref != kept.Ref {
		t.Fatalf("a set replace touched selections: %+v, %v", selections, err)
	}
}
