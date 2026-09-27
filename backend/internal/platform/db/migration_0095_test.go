package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// GUIDE-43: 95 holds one row per switched-off 기본 지침, keyed by account, kind and key; a kind
// outside post/clip is refused, a repeat of the key is the same row, and the rows leave with the
// account.
func TestMigration0095StoresSwitchedOffDefaults(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 95); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO guideline_defaults_off(user_id,kind,default_key,created_at) VALUES('alice',?,?,?)`
	if _, err := handle.Writer.Exec(insert, "post", "tags", at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(insert, "clip", "tags", at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(insert, "post", "tags", at); err == nil {
		t.Fatal("the same switch was stored twice")
	}
	if _, err := handle.Writer.Exec(insert, "video", "tags", at); err == nil {
		t.Fatal("a kind outside post/clip was stored")
	}
	if _, err := handle.Writer.Exec(`DELETE FROM users WHERE id='alice'`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM guideline_defaults_off`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows after the account left = %d (%v)", n, err)
	}
}
