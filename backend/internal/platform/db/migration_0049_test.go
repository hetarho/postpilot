package db

import (
	"github.com/pressly/goose/v3"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestMigration0049KeepsOldRendersUnfinalizedAndConfirmationIrreversible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "finalization.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Compare(entry.Name(), "0049_") >= 0 {
			continue
		}
		raw, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = &fstest.MapFile{Data: raw}
	}
	if err := migrate(t.Context(), d.Writer, before); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-13T00:00:00Z')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,edit_plan_json,edit_plan_revision,rendered_plan_revision,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,created_at,updated_at) VALUES('legacy','owner','legacy','square',15000,'legacy-plan',7,7,'clip-results/private.mp4','video/mp4',100,15000,'2026-09-13T00:00:00Z','2026-09-13T00:00:00Z','2026-09-13T00:00:00Z')`,
	} {
		if _, err := d.Writer.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(t.Context(), d.Writer); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), d.Writer); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM clip_projects WHERE id='legacy' AND finalized_at IS NULL AND finalized_plan_revision IS NULL AND finalized_result_key IS NULL AND result_id='legacy-legacy' AND edit_plan_json='legacy-plan' AND edit_plan_revision=7 AND rendered_plan_revision=7 AND source_access_revoked_at IS NULL AND result_key='clip-results/private.mp4'`).Scan(&n); err != nil || n != 1 {
		t.Fatal("migration inferred consent or changed result", n, err)
	}
	for _, sql := range []string{
		`UPDATE clip_projects SET finalized_at='2026-09-13T00:01:00Z' WHERE id='legacy'`,
		`UPDATE clip_projects SET finalized_at='2026-09-13T00:01:00Z',finalized_plan_revision=6,finalized_result_key=result_key,source_access_revoked_at='2026-09-13T00:01:00Z' WHERE id='legacy'`,
	} {
		if _, err := d.Writer.Exec(sql); err == nil {
			t.Fatal("partial confirmation accepted", sql)
		}
	}
	if _, err := d.Writer.Exec(`UPDATE clip_projects SET finalized_at='2026-09-13T00:01:00Z',finalized_plan_revision=7,finalized_result_key=result_key,source_access_revoked_at='2026-09-13T00:01:00Z' WHERE id='legacy'`); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE clip_projects SET finalized_at=NULL,finalized_plan_revision=NULL,finalized_result_key=NULL WHERE id='legacy'`,
		`UPDATE clip_projects SET result_key='clip-results/replaced.mp4' WHERE id='legacy'`,
		`UPDATE clip_projects SET edit_plan_json='changed' WHERE id='legacy'`,
		`UPDATE clip_projects SET source_access_revoked_at=NULL WHERE id='legacy'`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('job','owner','legacy','render_clip','queued','2026-09-13T00:01:00Z','2026-09-13T00:01:00Z')`,
	} {
		if _, err := d.Writer.Exec(sql); err == nil {
			t.Fatal("finalization guard bypassed", sql)
		}
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(t.Context(), 48); err == nil {
		t.Fatal("rollback discarded confirmation")
	}
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM clip_projects WHERE id='legacy' AND finalized_at IS NOT NULL AND result_key='clip-results/private.mp4'`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}
