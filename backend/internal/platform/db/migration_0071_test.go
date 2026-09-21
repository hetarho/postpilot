package db

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0071AlignsEstimatorCombosWithRegistrationLevels(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	selected := migrationsThrough(t,
		"0018_catalog_models.sql",
		"0020_catalog_model_purposes.sql",
		"0028_estimator_combos.sql",
		"0040_registration_level.sql",
		"0071_estimator_combo_levels.sql",
	)
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, selected, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 40); err != nil {
		t.Fatal(err)
	}

	at := "2026-09-21T00:00:00Z"
	registrations := []struct {
		modelID, purpose, level string
	}{
		{"openai/gpt-5.6-luna", "photo-analysis", "top"},
		{"anthropic/claude-sonnet-5", "writing", "top"},
		{"google/gemini-3.7-flash", "photo-analysis", "premium"},
		{"x-ai/grok-4.6", "writing", "premium"},
		{"z-ai/glm-5.3-flash", "photo-analysis", "balanced"},
		{"qwen/qwen3.8-flash", "writing", "top"},
		{"openrouter/free", "photo-analysis", ""},
		{"deepseek/deepseek-v4-flash-0731", "writing", "value"},
	}
	for _, registration := range registrations {
		var level any
		if registration.level != "" {
			level = registration.level
		}
		if _, err := handle.Writer.Exec(
			`INSERT INTO catalog_model_purposes(model_id,purpose,created_at,level) VALUES(?,?,?,?)`,
			registration.modelID, registration.purpose, at, level,
		); err != nil {
			t.Fatalf("register %s for %s: %v", registration.modelID, registration.purpose, err)
		}
	}
	assignments := []struct {
		combo, observe, write string
	}{
		{"quality", "openai/gpt-5.6-luna", "anthropic/claude-sonnet-5"},
		{"balanced", "google/gemini-3.7-flash", "x-ai/grok-4.6"},
		{"value", "z-ai/glm-5.3-flash", "qwen/qwen3.8-flash"},
		{"cheapest", "openrouter/free", "deepseek/deepseek-v4-flash-0731"},
	}
	for _, assignment := range assignments {
		if _, err := handle.Writer.Exec(
			`INSERT INTO estimator_combos(combo,observe_model_id,write_model_id,updated_at) VALUES(?,?,?,?)`,
			assignment.combo, assignment.observe, assignment.write, at,
		); err != nil {
			t.Fatalf("assign %s: %v", assignment.combo, err)
		}
	}

	if _, err := provider.UpTo(ctx, 71); err != nil {
		t.Fatal(err)
	}
	rows, err := handle.Reader.Query(`SELECT combo,observe_model_id,write_model_id FROM estimator_combos ORDER BY combo`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string][2]string{}
	for rows.Next() {
		var combo, observe, write string
		if err := rows.Scan(&combo, &observe, &write); err != nil {
			t.Fatal(err)
		}
		got[combo] = [2]string{observe, write}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("migrated assignments = %v, want only the two level-matched rows", got)
	}
	if got["top"] != [2]string{"openai/gpt-5.6-luna", "anthropic/claude-sonnet-5"} {
		t.Errorf("top = %v", got["top"])
	}
	if got["premium"] != [2]string{"google/gemini-3.7-flash", "x-ai/grok-4.6"} {
		t.Errorf("premium = %v", got["premium"])
	}
	if _, err := handle.Writer.Exec(
		`INSERT INTO estimator_combos(combo,observe_model_id,write_model_id,updated_at) VALUES('quality',?,?,?)`,
		"openai/gpt-5.6-luna", "anthropic/claude-sonnet-5", at,
	); err == nil {
		t.Error("the rebuilt constraint accepted retired combo quality")
	}
}
