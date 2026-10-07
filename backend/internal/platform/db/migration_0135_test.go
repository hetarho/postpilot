package db

import (
	"github.com/pressly/goose/v3"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestMigration0135LeavesWritingVoicesAndProjectionsIntact(t *testing.T) {
	h, err := Open(filepath.Join(t.TempDir(), "pre-spoken.db"))
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
	if _, err := p.UpTo(t.Context(), 134); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Writer.Exec("INSERT INTO users(id,password_hash,created_at,plan) VALUES ('owner','hash','2026-10-04T00:00:00Z','free')"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Writer.Exec("INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES ('writing','owner','Writing style',1,'created','updated')"); err != nil {
		t.Fatal(err)
	}
	// Pin every existing voice table/trigger/index definition, including training
	// and post projections. The new migration cannot rewrite any of them.
	before := map[string]string{}
	rows, err := h.Reader.Query("SELECT name,sql FROM sqlite_master WHERE name LIKE 'voice%' AND sql IS NOT NULL ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatal(err)
		}
		before[name] = sql
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(before) == 0 {
		t.Fatal("missing legacy voice schema")
	}
	if _, err := p.UpTo(t.Context(), 135); err != nil {
		t.Fatal(err)
	}
	for name, definition := range before {
		var after string
		if err := h.Reader.QueryRow("SELECT sql FROM sqlite_master WHERE name = ?", name).Scan(&after); err != nil || after != definition {
			t.Fatal(name, after, err)
		}
	}
	var name, created, updated string
	var isDefault int
	if err := h.Reader.QueryRow("SELECT name,is_default,created_at,updated_at FROM voices WHERE id='writing' AND user_id='owner'").Scan(&name, &isDefault, &created, &updated); err != nil || name != "Writing style" || isDefault != 1 || created != "created" || updated != "updated" {
		t.Fatal(name, isDefault, created, updated, err)
	}
	var count int
	if err := h.Reader.QueryRow("SELECT COUNT(*) FROM spoken_voices").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	violations, err := h.Reader.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer violations.Close()
	if violations.Next() {
		t.Fatal("foreign key violation")
	}
}
