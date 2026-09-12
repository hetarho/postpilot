package db

import (
	"context"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

func TestMigration0048PreservesJobGuardsAndHistoricalSettlement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cancellation.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	prior := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Compare(e.Name(), "0048_") >= 0 {
			continue
		}
		raw, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		prior[e.Name()] = &fstest.MapFile{Data: raw}
	}
	ctx := context.Background()
	if err := migrate(ctx, d.Writer, prior); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-01T00:00:00Z')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('clip','owner','old','square',15000,'2026-09-01T00:00:00Z','2026-09-01T00:00:00Z')`,
		`INSERT INTO generation_jobs(id,user_id,kind,status,payload,created_at,updated_at) VALUES('queued','owner','generate','queued','payload-q','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z')`,
		`INSERT INTO generation_jobs(id,user_id,kind,status,payload,created_at,updated_at) VALUES('running','owner','analyze_voice','running','payload-r','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z')`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,payload,created_at,updated_at,finished_at) VALUES('done','owner','clip','generate_clip','done','payload-d','2026-09-01T00:00:00Z','2026-09-01T00:00:01Z','2026-09-01T00:00:01Z')`,
		`INSERT INTO usage_admissions(user_id,kind,job_id,hold_credits,approved_max_credits,settled_credits,created_at,settled_at) VALUES('owner','generate_clip','done',5,5,2,'2026-09-01T00:00:00Z','2026-09-01T00:00:01Z')`,
	} {
		if _, err := d.Writer.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	guards := func() map[string]string {
		rows, err := d.Reader.Query(`SELECT name,sql FROM sqlite_master WHERE type IN ('index','trigger') AND sql LIKE '%generation_jobs%' ORDER BY name`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var name, sql string
			if err := rows.Scan(&name, &sql); err != nil {
				t.Fatal(err)
			}
			out[name] = sql
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := guards()
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, guards()) {
		t.Fatal("job ownership guards or active-work indexes changed")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM generation_jobs WHERE cancellation_policy_version=0 AND cancel_requested_at IS NULL AND ((id='queued' AND status='queued' AND payload='payload-q') OR (id='running' AND status='running' AND payload='payload-r') OR (id='done' AND status='done' AND payload='payload-d' AND finished_at='2026-09-01T00:00:01Z'))`).Scan(&n); err != nil || n != 3 {
		t.Fatal("job history changed", n, err)
	}
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM usage_admissions WHERE job_id='done' AND hold_credits=5 AND settled_credits=2 AND settled_at='2026-09-01T00:00:01Z' AND settlement_reason IS NULL AND confirmed_charge_credits IS NULL AND cancellation_fee_credits IS NULL`).Scan(&n); err != nil || n != 1 {
		t.Fatal("historical settlement reconstructed", n, err)
	}
	for _, q := range []string{
		`UPDATE generation_jobs SET status='cancelled' WHERE id='running'`,
		`UPDATE generation_jobs SET cancel_requested_at='2026-09-01T00:00:02Z' WHERE id='running'`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,cancellation_policy_version,created_at,updated_at) VALUES('foreign','owner','missing','generate_clip','queued',1,'2026-09-01T00:00:00Z','2026-09-01T00:00:00Z')`,
	} {
		if _, err := d.Writer.Exec(q); err == nil {
			t.Fatal("lost database guard", q)
		}
	}
	if _, err := d.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,cancellation_policy_version,cancel_requested_at,created_at,updated_at,finished_at) VALUES('cancelled','owner','clip','generate_clip','cancelled',1,'2026-09-01T00:00:01Z','2026-09-01T00:00:00Z','2026-09-01T00:00:02Z','2026-09-01T00:00:02Z')`); err != nil {
		t.Fatal(err)
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 47); err == nil {
		t.Fatal("rollback discarded cancellation history")
	}
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM generation_jobs WHERE id='cancelled' AND status='cancelled'`).Scan(&n); err != nil || n != 1 {
		t.Fatal("failed rollback changed history", n, err)
	}
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&n); err != nil || n != 0 {
		t.Fatal("migration broke foreign keys", n, err)
	}
}
