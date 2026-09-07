package db

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// 0027 rebuilds credit_lots to widen its kind CHECK, and a rebuild is exactly the migration
// whose rollback nobody exercises until they need it. The Down half must land the credits
// somewhere the two-kind world can hold them and leave the foreign-key graph intact.
func TestMigration0027RollsBackPurchasedLotsToBonus(t *testing.T) {
	handle, err := Open(filepath.Join(t.TempDir(), "postpilot.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO users (id, password_hash, plan, created_at)
		 VALUES ('alice','hash','free','2026-09-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, lot := range []struct{ id, kind string }{{"m", "monthly"}, {"b", "bonus"}, {"p", "purchased"}} {
		if _, err := handle.Writer.ExecContext(ctx,
			`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
			 VALUES (?, 'alice', ?, 100, 100, NULL, '2026-09-01T00:00:00Z')`, lot.id, lot.kind); err != nil {
			t.Fatalf("seed %s lot: %v", lot.kind, err)
		}
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 26); err != nil {
		t.Fatalf("down to 26: %v", err)
	}

	// The paid credits survive as a bonus rather than being dropped, and every lot is
	// still there.
	for _, tc := range []struct{ id, want string }{{"m", "monthly"}, {"b", "bonus"}, {"p", "bonus"}} {
		var kind string
		if err := handle.Reader.QueryRow("SELECT kind FROM credit_lots WHERE id = ?", tc.id).Scan(&kind); err != nil {
			t.Fatalf("read lot %s: %v", tc.id, err)
		}
		if kind != tc.want {
			t.Errorf("lot %s kind = %q, want %q", tc.id, kind, tc.want)
		}
	}

	// Back to the two-kind CHECK.
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
		 VALUES ('p2', 'alice', 'purchased', 1, 1, NULL, '2026-09-01T00:00:00Z')`); err == nil {
		t.Error("the rolled-back schema accepted a purchased lot")
	}

	rows, err := handle.Reader.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Error("rollback left a foreign-key violation")
	}
}
