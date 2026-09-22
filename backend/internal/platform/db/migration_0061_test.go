package db

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// 0061 repairs projects an earlier TouchCompositionRevision pushed out of the draft state by
// raising edit_plan_revision for inputs saved before any generation existed. The repair has to
// reach exactly those rows: a generated project's revision is what its rendered result is
// compared against, so moving one would stale a render that is perfectly current.
func TestMigration0061ReturnsUngeneratedProjectsToRevisionZero(t *testing.T) {
	handle, err := Open(filepath.Join(t.TempDir(), "postpilot.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := migrateBeforePublishingRemoval(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 60); err != nil {
		t.Fatalf("down to 60: %v", err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO users (id, password_hash, plan, created_at)
		 VALUES ('alice','hash','free','2026-09-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	seed := []struct {
		id       string
		plan     any
		revision int
		rendered int
	}{
		// Inputs typed in step 1, never generated: what the bug stranded at step 2.
		{"stranded", nil, 2, 0},
		{"untouched", nil, 0, 0},
		// Generated and rendered: its revision is the render's identity and must not move.
		{"generated", "plan", 4, 3},
	}
	for _, row := range seed {
		if _, err := handle.Writer.ExecContext(ctx,
			`INSERT INTO clip_projects (id, user_id, title, ratio, target_duration_ms, edit_plan_json,
			 edit_plan_revision, rendered_plan_revision, created_at, updated_at)
			 VALUES (?, 'alice', 'clip', 'vertical', 15000, ?, ?, ?, '2026-09-01T00:00:00Z', '2026-09-01T00:00:00Z')`,
			row.id, row.plan, row.revision, row.rendered); err != nil {
			t.Fatalf("seed %s: %v", row.id, err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	for _, tc := range []struct{ id, want string }{{"stranded", "0"}, {"untouched", "0"}, {"generated", "4"}} {
		var revision string
		if err := handle.Reader.QueryRow("SELECT edit_plan_revision FROM clip_projects WHERE id = ?", tc.id).Scan(&revision); err != nil {
			t.Fatalf("read %s: %v", tc.id, err)
		}
		if revision != tc.want {
			t.Errorf("%s revision = %s, want %s", tc.id, revision, tc.want)
		}
	}
	// updated_at is untouched: the repair is not an edit the owner made.
	var stamp string
	if err := handle.Reader.QueryRow("SELECT updated_at FROM clip_projects WHERE id = 'stranded'").Scan(&stamp); err != nil {
		t.Fatal(err)
	}
	if stamp != "2026-09-01T00:00:00Z" {
		t.Errorf("repair moved updated_at to %q", stamp)
	}
}
