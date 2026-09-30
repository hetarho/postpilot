package db

import (
	"context"
	"testing"
)

// The seed is the set providers.yaml used to ship (MODEL-26), slot for slot, and every model it
// names is a row the catalog seed inserts, so a fresh installation can register it straight away.
func TestMigration0122SeedsTheShippedRecommendationSet(t *testing.T) {
	ctx := context.Background()
	handle := openTemp(t)
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}

	var id, label string
	var position int
	if err := handle.Reader.QueryRow("SELECT id, label, position FROM recommendation_sets").Scan(&id, &label, &position); err != nil {
		t.Fatal(err)
	}
	if id != "balanced-2026-08" || label != "Balanced · August 2026" || position != 1 {
		t.Fatalf("seeded set = %q %q %d", id, label, position)
	}

	rows, err := handle.Reader.Query("SELECT stage, slot, provider_id, model_id FROM recommendation_set_slots WHERE set_id = ? ORDER BY stage, slot", id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var stage, slot, providerID, modelID string
		if err := rows.Scan(&stage, &slot, &providerID, &modelID); err != nil {
			t.Fatal(err)
		}
		if providerID != "openrouter" {
			t.Errorf("%s/%s provider = %q", stage, slot, providerID)
		}
		got[stage+"/"+slot] = modelID
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"observe/active":      "google/gemini-3.7-flash",
		"observe/candidate_a": "google/gemini-3.7-flash",
		"observe/candidate_b": "qwen/qwen3.8-flash",
		"analyze/active":      "openai/gpt-5.6-luna",
		"write/active":        "anthropic/claude-sonnet-5",
		"write/candidate_a":   "anthropic/claude-sonnet-5",
		"write/candidate_b":   "x-ai/grok-4.6",
	}
	if len(got) != len(want) {
		t.Fatalf("slots = %v", got)
	}
	for key, model := range want {
		if got[key] != model {
			t.Errorf("%s = %q, want %q", key, got[key], model)
		}
		var seeded int
		if err := handle.Reader.QueryRow("SELECT count(*) FROM catalog_models WHERE model_id = ?", model).Scan(&seeded); err != nil || seeded != 1 {
			t.Errorf("%s is not a seeded catalog row: %d, %v", model, seeded, err)
		}
	}
}

// Analyze keeps its active slot alone, and deleting a set takes its slots with it.
func TestMigration0122SlotShapeAndCascade(t *testing.T) {
	ctx := context.Background()
	handle := openTemp(t)
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec("INSERT INTO recommendation_set_slots(set_id,stage,slot,provider_id,model_id) VALUES('balanced-2026-08','analyze','candidate_a','openrouter','m')"); err == nil {
		t.Fatal("an analyze candidate slot was accepted")
	}
	mustExec(t, handle.Writer, "DELETE FROM recommendation_sets WHERE id = 'balanced-2026-08'")
	var left int
	if err := handle.Reader.QueryRow("SELECT count(*) FROM recommendation_set_slots").Scan(&left); err != nil || left != 0 {
		t.Fatalf("slots left after delete = %d, %v", left, err)
	}
}
