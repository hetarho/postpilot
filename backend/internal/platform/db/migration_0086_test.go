package db

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// The shared defaults become intro A and outro B (CLIP-111), so a project that
// never chose is first given the presets it renders in today: an empty id
// becomes b or e, an explicit choice is left alone, and nothing is touched or
// made stale.
func TestMigration0086KeepsEveryExistingProjectsPresets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "presets.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Compare(entry.Name(), "0086_") >= 0 {
			continue
		}
		raw, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = &fstest.MapFile{Data: raw}
	}
	if err := migrate(t.Context(), d.Writer, before); err != nil {
		t.Fatal(err)
	}
	project := func(id, intro, outro string) string {
		return `INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,intro_preset,outro_preset,edit_plan_revision,created_at,updated_at) VALUES('` + id + `','owner','p','vertical',15000,'` + intro + `','` + outro + `',3,'2026-09-20T00:00:00Z','2026-09-20T00:00:00Z')`
	}
	for _, sql := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-20T00:00:00Z')`,
		project("never-chose", "", ""),
		project("chose-intro", "a", ""),
		project("chose-both", "a", "b"),
	} {
		if _, err := d.Writer.Exec(sql); err != nil {
			t.Fatal(sql, err)
		}
	}
	for range 2 {
		// Every boot runs Migrate: a second pass changes nothing more.
		if err := Migrate(t.Context(), d.Writer); err != nil {
			t.Fatal(err)
		}
		for _, want := range []struct{ id, intro, outro string }{
			{"never-chose", "b", "e"},
			{"chose-intro", "a", "e"},
			{"chose-both", "a", "b"},
		} {
			var intro, outro, updated string
			var revision int
			if err := d.Reader.QueryRow(`SELECT intro_preset,outro_preset,edit_plan_revision,updated_at FROM clip_projects WHERE id=?`, want.id).Scan(&intro, &outro, &revision, &updated); err != nil {
				t.Fatal(err)
			}
			if intro != want.intro || outro != want.outro || revision != 3 || updated != "2026-09-20T00:00:00Z" {
				t.Fatalf("%s took %q/%q rev %d at %s, wanted %q/%q untouched", want.id, intro, outro, revision, updated, want.intro, want.outro)
			}
		}
	}
}
