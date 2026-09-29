package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0116RetainsActiveReservationsAndRollsBackCompensation(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 115); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-30T00:00:00.000000000Z"
	end := "2026-10-07T00:00:00.000000000Z"
	mustExec(t, handle.Writer, "INSERT INTO users(id,password_hash,created_at,plan) VALUES('alice','hash',?,'basic')", at)
	mustExec(t, handle.Writer, `INSERT INTO credit_lots(id,user_id,kind,granted,remaining,expires_at,created_at)
VALUES('old','alice','bonus',30,20,?,?)`, end, at)
	mustExec(t, handle.Writer, "INSERT INTO credit_hold_lots(job_id,lot_id,credits) VALUES('job','old',10)")
	if _, err := provider.UpTo(ctx, 116); err != nil {
		t.Fatal(err)
	}
	mustExec(t, handle.Writer, `INSERT INTO credit_lots(id,user_id,kind,granted,remaining,expires_at,created_at,issuance_cause)
VALUES('compensation:job','alice','compensation',4,4,?,?,'service_fault')`, end, at)
	mustExec(t, handle.Writer, `INSERT INTO fx_reference_days(publication_date,state,source,reference_e4,verified_at)
VALUES('2026-09-29','published','korea-eximbank',13600000,?)`, at)
	if _, err := provider.DownTo(ctx, 115); err != nil {
		t.Fatal(err)
	}
	var kind string
	var remaining, holds int
	if err := handle.Reader.QueryRow("SELECT kind,remaining FROM credit_lots WHERE id='compensation:job'").Scan(&kind, &remaining); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow("SELECT count(*) FROM credit_hold_lots WHERE job_id='job' AND lot_id='old'").Scan(&holds); err != nil {
		t.Fatal(err)
	}
	if kind != "bonus" || remaining != 4 || holds != 1 {
		t.Fatalf("rollback compensation=%s/%d old hold=%d", kind, remaining, holds)
	}
}
