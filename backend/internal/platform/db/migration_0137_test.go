package db

import (
	"github.com/pressly/goose/v3"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestMigration0137PreservesExistingClipsWithoutInventingSpeech(t *testing.T) {
	h, err := Open(filepath.Join(t.TempDir(), "pre-clip-speech.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(t.Context(), 136); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Writer.Exec("INSERT INTO users(id,password_hash,created_at,plan) VALUES ('owner','hash','created','free')"); err != nil {
		t.Fatal(err)
	}
	old := `{"Version":6,"Composition":{"Elements":[{"Scope":"narration"}]}}`
	if _, err := h.Writer.Exec("INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at,edit_plan_json) VALUES ('project','owner','old clip','vertical',15000,'created','updated',?)", old); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	var saved string
	if err := h.Reader.QueryRow("SELECT edit_plan_json FROM clip_projects WHERE id='project'").Scan(&saved); err != nil || saved != old {
		t.Fatal("legacy plan rewritten", saved, err)
	}
	for _, table := range []string{"clip_speech_assets", "clip_speech_segments"} {
		var count int
		if err := h.Reader.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal(table, count, err)
		}
	}
	rows, err := h.Reader.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation")
	}
}
