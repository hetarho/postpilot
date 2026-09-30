package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/platform/db"
	providerstore "github.com/postpilot/backend/internal/provider/store"
)

// The document names models by id alone; the adapter fills the registry's provider on the way
// in, drops it on the way out, and gives a new set its identity (MODEL-72).
func TestCatalogRecommendationsRoundTripsTheProviderRows(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	tx, err := handle.Writer.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	rows := catalogRecommendations{rows: providerstore.NewTx(tx), providerID: "openrouter"}

	seeded, err := rows.List(ctx)
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seeded = %+v, %v", seeded, err)
	}
	want := modelcatalog.StoredSet{
		ID: "balanced-2026-08", Label: "Balanced · August 2026",
		Observe: [3]string{"google/gemini-3.7-flash", "google/gemini-3.7-flash", "qwen/qwen3.8-flash"},
		Analyze: "openai/gpt-5.6-luna",
		Write:   [3]string{"anthropic/claude-sonnet-5", "anthropic/claude-sonnet-5", "x-ai/grok-4.6"},
	}
	if seeded[0] != want {
		t.Fatalf("seeded = %+v, want %+v", seeded[0], want)
	}

	fresh := want
	fresh.ID, fresh.Label = "", "Fresh"
	if err := rows.Replace(ctx, []modelcatalog.StoredSet{seeded[0], fresh}, time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	after, err := rows.List(ctx)
	if err != nil || len(after) != 2 || after[0].ID != "balanced-2026-08" || after[1].ID == "" || after[1].Label != "Fresh" {
		t.Fatalf("after = %+v, %v", after, err)
	}
	var providers int
	if err := tx.QueryRow(`SELECT count(*) FROM recommendation_set_slots WHERE provider_id = 'openrouter'`).Scan(&providers); err != nil || providers != 14 {
		t.Fatalf("slots carrying the registry's provider = %d, %v", providers, err)
	}
}
