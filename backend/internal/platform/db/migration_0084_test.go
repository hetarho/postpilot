package db

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// 0084 admits the voucher lot kind (QUOTA-58). The rebuild must keep every existing lot as it
// was, keep the hold debits that point at them, and fold a voucher lot back into an expiring
// bonus on the way down.
func TestMigration0084AdmitsVoucherLotsKeepingEveryRow(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 83); err != nil {
		t.Fatal(err)
	}

	at := "2026-09-25T00:00:00.000000000Z"
	expires := "2026-10-25T00:00:00.000000000Z"
	mustExec(t, handle.Writer,
		`INSERT INTO users (id, password_hash, plan, created_at) VALUES ('alice','hash','free',?)`, at)
	for _, lot := range []struct {
		id, kind string
		expires  any
	}{
		{"monthly", "monthly", expires},
		{"bonus", "bonus", nil},
		{"purchased", "purchased", nil},
	} {
		mustExec(t, handle.Writer,
			`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
			 VALUES (?, 'alice', ?, 10, 7, ?, ?)`, lot.id, lot.kind, lot.expires, at)
	}
	mustExec(t, handle.Writer, `INSERT INTO credit_hold_lots (job_id, lot_id, credits) VALUES ('job-1', 'monthly', 3)`)
	if _, err := handle.Writer.Exec(
		`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
		 VALUES ('early', 'alice', 'voucher', 1, 1, ?, ?)`, expires, at); err == nil {
		t.Fatal("0083 already accepted a voucher lot")
	}

	if _, err := provider.UpTo(ctx, 84); err != nil {
		t.Fatal(err)
	}
	for _, lot := range []struct {
		id, kind string
		expires  sql.NullString
	}{
		{"monthly", "monthly", sql.NullString{String: expires, Valid: true}},
		{"bonus", "bonus", sql.NullString{}},
		{"purchased", "purchased", sql.NullString{}},
	} {
		var kind string
		var granted, remaining int
		var gotExpires sql.NullString
		if err := handle.Writer.QueryRow(
			`SELECT kind, granted, remaining, expires_at FROM credit_lots WHERE id = ?`, lot.id,
		).Scan(&kind, &granted, &remaining, &gotExpires); err != nil {
			t.Fatalf("read %s: %v", lot.id, err)
		}
		if kind != lot.kind || granted != 10 || remaining != 7 || gotExpires != lot.expires {
			t.Fatalf("%s = %s %d/%d %v, want %s 10/7 %v", lot.id, kind, remaining, granted, gotExpires, lot.kind, lot.expires)
		}
	}
	var debits int
	if err := handle.Writer.QueryRow(`SELECT count(*) FROM credit_hold_lots WHERE lot_id = 'monthly'`).Scan(&debits); err != nil || debits != 1 {
		t.Fatalf("hold debits after rebuild = %d, %v; want 1", debits, err)
	}
	mustExec(t, handle.Writer,
		`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
		 VALUES ('voucher', 'alice', 'voucher', 5, 4, ?, ?)`, expires, at)
	if _, err := handle.Writer.Exec(
		`INSERT INTO credit_lots (id, user_id, kind, granted, remaining, expires_at, created_at)
		 VALUES ('gift', 'alice', 'gift', 1, 1, NULL, ?)`, at); err == nil {
		t.Fatal("a fifth lot kind was accepted")
	}

	if _, err := provider.DownTo(ctx, 83); err != nil {
		t.Fatal(err)
	}
	var kind string
	var remaining int
	var gotExpires sql.NullString
	if err := handle.Writer.QueryRow(
		`SELECT kind, remaining, expires_at FROM credit_lots WHERE id = 'voucher'`,
	).Scan(&kind, &remaining, &gotExpires); err != nil {
		t.Fatal(err)
	}
	if kind != "bonus" || remaining != 4 || gotExpires.String != expires {
		t.Fatalf("voucher after rollback = %s %d %v, want bonus 4 %s", kind, remaining, gotExpires, expires)
	}
}

func mustExec(t *testing.T, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}
