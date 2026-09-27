package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// MEM-15: 92 empties every finished extraction's payload and leaves an unfinished one, which
// still needs its frozen source, and every other kind of job alone.
func TestMigration0092ClearsFinishedExtractionPayloads(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 91); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-27T00:00:00Z"
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, kind, status, payload string }{
		{"done", "extract_memory", "done", `{"candidates":["private"]}`},
		{"running", "extract_memory", "running", `{"source":"frozen"}`},
		{"other", "analyze_voice", "done", `{"kept":true}`},
	} {
		if _, err := handle.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,kind,status,payload,created_at,updated_at) VALUES(?,'alice',?,?,?,?,?)`,
			row.id, row.kind, row.status, row.payload, at, at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := provider.UpTo(ctx, 92); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"done": "", "running": `{"source":"frozen"}`, "other": `{"kept":true}`} {
		var got string
		if err := handle.Reader.QueryRow(`SELECT payload FROM generation_jobs WHERE id=?`, id).Scan(&got); err != nil || got != want {
			t.Fatalf("%s payload = %q, want %q (%v)", id, got, want, err)
		}
	}
}
