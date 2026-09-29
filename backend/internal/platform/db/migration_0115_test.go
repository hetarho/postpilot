package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0115PreservesExistingLotsHoldsAndPaidCoverage(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 114); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-30T00:00:00.000000000Z"
	end := "2026-10-30T00:00:00.000000000Z"
	mustExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at,plan) VALUES('alice','hash',?,'basic')", at)
	mustExec(t, handle.Writer, `INSERT INTO credit_lots(id,user_id,kind,granted,remaining,expires_at,created_at)
VALUES('old','alice','monthly',330,300,?,?)`, end, at)
	mustExec(t, handle.Writer, "INSERT INTO credit_hold_lots(job_id,lot_id,credits) VALUES('job','old',30)")
	mustExec(t, handle.Writer, `INSERT INTO subscriptions(user_id,tier,term,anchor_at,term_start,term_end,next_grant_at,status,created_at,updated_at)
VALUES('alice','basic','monthly',?,?,? ,?,'active',?,?)`, at, at, end, end, at, at)
	if _, err := provider.UpTo(ctx, 115); err != nil {
		t.Fatal(err)
	}
	var granted, remaining, holds int
	if err := handle.Reader.QueryRow("SELECT granted,remaining FROM credit_lots WHERE id='old'").Scan(&granted, &remaining); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow("SELECT count(*) FROM credit_hold_lots WHERE job_id='job' AND lot_id='old'").Scan(&holds); err != nil {
		t.Fatal(err)
	}
	if granted != 330 || remaining != 300 || holds != 1 {
		t.Fatalf("migration changed old grant/hold: %d/%d holds=%d", remaining, granted, holds)
	}
	var coverageID string
	if err := handle.Reader.QueryRow("SELECT coverage_id FROM subscriptions WHERE user_id='alice'").Scan(&coverageID); err != nil || coverageID == "" {
		t.Fatalf("coverage id=%q err=%v", coverageID, err)
	}
	mustExec(t, handle.Writer, "UPDATE subscriptions SET tier='light' WHERE user_id='alice'")
	mustExec(t, handle.Writer, `INSERT INTO credit_lots(id,user_id,kind,granted,remaining,expires_at,created_at,coverage_id,window_start,issuance_cause)
VALUES('new-daily','alice','daily',15,15,?,?,'coverage',?,'coverage')`, end, at, at)
	if _, err := provider.DownTo(ctx, 114); err != nil {
		t.Fatal(err)
	}
	var tier, kind string
	if err := handle.Reader.QueryRow("SELECT tier FROM subscriptions WHERE user_id='alice'").Scan(&tier); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow("SELECT kind FROM credit_lots WHERE id='new-daily'").Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow("SELECT count(*) FROM credit_hold_lots WHERE job_id='job' AND lot_id='old'").Scan(&holds); err != nil {
		t.Fatal(err)
	}
	if tier != "basic" || kind != "bonus" || holds != 1 {
		t.Fatalf("rollback tier=%s daily=%s holds=%d", tier, kind, holds)
	}
}
