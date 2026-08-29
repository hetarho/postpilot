package db

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

func openTemp(t *testing.T) *DB {
	t.Helper()
	// A nested directory proves Open creates the parent — on a fresh volume the
	// mount point exists but the db's directory does not.
	handle, err := Open(filepath.Join(t.TempDir(), "nested", "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	return handle
}

// TestOpenAppliesPragmas is job 01 A11: WAL and exactly one write connection.
func TestOpenAppliesPragmas(t *testing.T) {
	handle := openTemp(t)

	var journalMode string
	if err := handle.Writer.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	// The pragmas ride the DSN, so they must hold on the reader pool too.
	var foreignKeys int
	if err := handle.Reader.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Error("foreign_keys is off on the reader — a cascade delete would silently not cascade")
	}

	if got := handle.Writer.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("writer MaxOpenConnections = %d, want 1 (the single serialized writer)", got)
	}
}

func TestMigrateAppliesSchemaAndIsIdempotent(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()

	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Every boot runs Migrate, so a second call must be a no-op, not a failure.
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("Migrate (second run): %v", err)
	}

	for _, table := range []string{"users", "sessions", "model_experiments", "model_experiment_candidates"} {
		var name string
		err := handle.Reader.QueryRow(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing after migration: %v", table, err)
		}
	}
}

func TestMigration0006UpgradesActiveSelectionsAndRollsBack(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	firstThree := fstest.MapFS{}
	for _, name := range []string{"0001_users_sessions.sql", "0002_posts_images_uploads.sql", "0003_model_selections.sql"} {
		data, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		firstThree[name] = &fstest.MapFile{Data: data}
	}
	if err := migrate(ctx, handle.Writer, firstThree); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO model_selections(user_id,stage,provider_id,model_id,updated_at) VALUES('alice','write','p','m','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	var slot, model string
	if err := handle.Reader.QueryRow(`SELECT slot,model_id FROM model_selections WHERE user_id='alice' AND stage='write'`).Scan(&slot, &model); err != nil {
		t.Fatal(err)
	}
	if slot != "active" || model != "m" {
		t.Fatalf("upgraded selection = %s/%s", slot, model)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	var columns int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('model_selections') WHERE name='slot'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("migration down retained slot column")
	}
	if err := handle.Reader.QueryRow(`SELECT model_id FROM model_selections WHERE user_id='alice' AND stage='write'`).Scan(&model); err != nil || model != "m" {
		t.Fatalf("down lost active selection: model=%q err=%v", model, err)
	}
}

// TestMigrateFailurePropagates is plan 01 AC8's mechanism: a broken migration must
// return an error so cmd/api can exit non-zero before the listener starts, which is
// what lets the deploy's /health gate roll back ([I7]).
func TestMigrateFailurePropagates(t *testing.T) {
	handle := openTemp(t)

	broken := fstest.MapFS{
		"0001_broken.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nCREATE TABLE ( this is not sql;\n"),
		},
	}

	if err := migrate(context.Background(), handle.Writer, broken); err == nil {
		t.Fatal("a broken migration reported success")
	}
}

func TestOpenRejectsUnusablePath(t *testing.T) {
	// A file where the directory should be: MkdirAll fails, and Open must say so at
	// boot rather than at the first query.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := Open(filepath.Join(blocker, "db.sqlite")); err == nil {
		t.Error("Open succeeded on a path whose parent is a regular file")
	}
}
