package db

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestMigration0039LeavesHistoricalClipChargesAndUnknownApproval(t *testing.T) {
	d := openTemp(t)
	prior := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Compare(e.Name(), "0039_") >= 0 {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		prior[e.Name()] = &fstest.MapFile{Data: body}
	}
	ctx := context.Background()
	if err = migrate(ctx, d.Writer, prior); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec("INSERT INTO users(id,password_hash,plan,created_at) VALUES('owner','hash','free','2026-09-01T00:00:00Z')"); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec("INSERT INTO usage_admissions(user_id,kind,job_id,hold_credits,created_at,settled_at,settled_credits) VALUES('owner','generate_clip','old',5,'2026-09-01T00:00:00Z','2026-09-01T00:01:00Z',2)"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	var held, settled int
	var approved sql.NullInt64
	if err = d.Reader.QueryRow("SELECT hold_credits,settled_credits,approved_max_credits FROM usage_admissions WHERE job_id='old'").Scan(&held, &settled, &approved); err != nil || held != 5 || settled != 2 || approved.Valid {
		t.Fatal(held, settled, approved, err)
	}
	if _, err = d.Writer.Exec("UPDATE usage_admissions SET approved_max_credits=-1 WHERE job_id='old'"); err == nil {
		t.Fatal("negative approval accepted")
	}
}
