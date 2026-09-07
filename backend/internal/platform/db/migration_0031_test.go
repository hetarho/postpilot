package db

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0031AddsAndRollsBackScheduledChangeEvent(t *testing.T) {
	handle, err := Open(filepath.Join(t.TempDir(), "postpilot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-08T00:00:00Z"
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO users(id,password_hash,plan,created_at) VALUES('alice','hash','free',?)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO billing_events(user_id,kind,tier,term,created_at) VALUES('alice','change_scheduled','basic','annual',?)`, at); err != nil {
		t.Fatalf("new CHECK refused change_scheduled: %v", err)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 30); err != nil {
		t.Fatalf("down to 30: %v", err)
	}
	var kind string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT kind FROM billing_events WHERE user_id='alice'`).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "cancel_scheduled" {
		t.Fatalf("rolled-back kind = %q", kind)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO billing_events(user_id,kind,created_at) VALUES('alice','change_scheduled',?)`, at); err == nil {
		t.Fatal("rolled-back CHECK accepted change_scheduled")
	}
}
