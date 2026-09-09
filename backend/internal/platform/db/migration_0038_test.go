package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestMigration0038PreservesExistingJobsAndGuards(t *testing.T) {
	d := openTemp(t)
	prior := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "0038_") {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		prior[e.Name()] = &fstest.MapFile{Data: body}
	}
	ctx := context.Background()
	if err := migrate(ctx, d.Writer, prior); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-10T00:00:00Z"
	if _, err := d.Writer.Exec("INSERT INTO users(id,password_hash,plan,created_at) VALUES('owner','hash','free',?)", at); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.Exec("INSERT INTO generation_jobs(id,user_id,kind,status,payload,created_at,updated_at) VALUES('old','owner','legacy','queued','{}',?,?)", at, at); err != nil {
		t.Fatal(err)
	}
	guards := map[string]string{}
	rows, err := d.Reader.Query("SELECT name,sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL AND tbl_name='generation_jobs' AND name!='generation_jobs_active_user_kind_idx'")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		guards[name] = ddl
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	var ready int
	var payload, status string
	if err := d.Reader.QueryRow("SELECT dispatch_ready,payload,status FROM generation_jobs WHERE id='old' AND clip_project_id IS NULL").Scan(&ready, &payload, &status); err != nil {
		t.Fatal(err)
	}
	if ready != 1 || payload != "{}" || status != "queued" {
		t.Fatal(ready, payload, status)
	}
	for name, want := range guards {
		var got string
		if err := d.Reader.QueryRow("SELECT sql FROM sqlite_master WHERE name=?", name).Scan(&got); err != nil || got != want {
			t.Fatal(name, got, err)
		}
	}
	if _, err := d.Writer.Exec("INSERT INTO generation_jobs(id,user_id,kind,status,payload,created_at,updated_at) VALUES('duplicate','owner','legacy','queued','{}',?,?)", at, at); err == nil {
		t.Fatal("old account-kind guard lost")
	}
}
