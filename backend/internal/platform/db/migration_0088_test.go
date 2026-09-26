package db

import (
	"context"
	"io/fs"
	"reflect"
	"testing"

	"github.com/pressly/goose/v3"
)

const at0088 = "2026-09-26T00:00:00Z"

// presetTables0088 are the two tables 0088 drops, the fields first because they hang off the row.
var presetTables0088 = []string{"guideline_preset_fields", "guideline_presets"}

// provider0088 takes a fresh database to 87 and seeds alice's preset switched on for two 분야
// beside a fields guideline and its link, so each test moves across 88 itself.
func provider0088(t *testing.T) (*DB, *goose.Provider) {
	t.Helper()
	handle := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(context.Background(), 87); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?),('bob','hash',?)`, []any{at0088, at0088}},
		{`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('fields','alice','메뉴 가격은 쓰지 않기','fields',?,?)`, []any{at0088, at0088}},
		{`INSERT INTO guideline_fields(guideline_id,field,user_id) VALUES('fields','cafe','alice')`, nil},
		{`INSERT INTO guideline_presets(user_id,enabled,updated_at) VALUES('alice',1,?)`, []any{at0088}},
		{`INSERT INTO guideline_preset_fields(user_id,field) VALUES('alice','cafe'),('alice','restaurant')`, nil},
	} {
		if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %q: %v", statement.sql, err)
		}
	}
	return handle, provider
}

// guidelineState0088 is every guideline row and field link the drop must leave, one string each.
func guidelineState0088(t *testing.T, handle *DB) []string {
	t.Helper()
	rows, err := handle.Reader.Query(`SELECT 'g:' || id || '|' || user_id || '|' || text || '|' || scope || '|' || created_at || '|' || updated_at FROM guidelines
		UNION ALL SELECT 'f:' || guideline_id || '>' || field || '@' || user_id FROM guideline_fields ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// GUIDE-14: a post's guidelines are the owner's texts alone and nothing reads the preset, so 0088
// drops both preset tables and leaves the guidelines, their links and the graph's integrity.
func TestMigration0088DropsTheGuidelinePresets(t *testing.T) {
	ctx := context.Background()
	handle, provider := provider0088(t)
	before := guidelineState0088(t, handle)
	definitions := map[string]string{}
	for _, table := range presetTables0088 {
		definition, ok := schemaObject0077(t, handle, "table", table)
		if !ok {
			t.Fatalf("%s is missing at 87", table)
		}
		definitions[table] = definition
	}

	if _, err := provider.UpTo(ctx, 88); err != nil {
		t.Fatal(err)
	}
	for _, table := range presetTables0088 {
		if _, ok := schemaObject0077(t, handle, "table", table); ok {
			t.Errorf("%s survived 0088", table)
		}
	}
	if after := guidelineState0088(t, handle); !reflect.DeepEqual(after, before) {
		t.Fatalf("the drop changed the guidelines: %q, want %q", after, before)
	}
	if n := foreignKeyViolations0078(t, handle); n != 0 {
		t.Fatalf("%d foreign-key violations after the drop", n)
	}
	var integrity string
	if err := handle.Reader.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, %v", integrity, err)
	}

	// Down brings both back empty, with 0078's DDL, for a rolled-back binary that reads them.
	if _, err := provider.DownTo(ctx, 87); err != nil {
		t.Fatal(err)
	}
	for _, table := range presetTables0088 {
		definition, ok := schemaObject0077(t, handle, "table", table)
		if !ok || definition != definitions[table] {
			t.Errorf("restored %s = %q, want 0078's %q", table, definition, definitions[table])
		}
		if n := count0078(t, handle, `SELECT count(*) FROM `+table); n != 0 {
			t.Errorf("restored %s holds %d rows, want none", table, n)
		}
	}
	field := `INSERT INTO guideline_preset_fields(user_id,field) VALUES(?,?)`
	if err := exec0078(t, handle, field, "alice", "cafe"); err == nil {
		t.Error("the restored fields accepted a 분야 without its account's preset row")
	}
	preset := `INSERT INTO guideline_presets(user_id,enabled,updated_at) VALUES(?,?,?)`
	if err := exec0078(t, handle, preset, "alice", 2, at0088); err == nil {
		t.Error("the restored preset accepted an enabled value other than 0 or 1")
	}
	if err := exec0078(t, handle, preset, "bob", 1, at0088); err != nil {
		t.Fatal(err)
	}
	if err := exec0078(t, handle, field, "bob", "cafe"); err != nil {
		t.Fatal(err)
	}
	if err := exec0078(t, handle, `DELETE FROM users WHERE id = 'bob'`); err != nil {
		t.Fatal(err)
	}
	if n := count0078(t, handle, `SELECT count(*) FROM guideline_presets`) + count0078(t, handle, `SELECT count(*) FROM guideline_preset_fields`); n != 0 {
		t.Errorf("deleting the account left %d restored preset rows", n)
	}
	if after := guidelineState0088(t, handle); !reflect.DeepEqual(after, before) {
		t.Fatalf("the rollback changed the guidelines: %q, want %q", after, before)
	}
}
