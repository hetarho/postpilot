package db

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

func TestMigration0047BackfillsBindingsAndPreservesOriginalIdentityAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retained.db")
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
		if strings.Compare(e.Name(), "0047_") >= 0 {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		prior[e.Name()] = &fstest.MapFile{Data: data}
	}
	ctx := context.Background()
	if err = migrate(ctx, d.Writer, prior); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-01T00:00:00Z')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,analysis_json,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,created_at,updated_at) VALUES('project','owner','saved','vertical',15000,'analysis','plan','result.mp4','video/mp4',123,15000,'2026-09-01T00:00:00Z','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z')`,
		`INSERT INTO clip_source_batches(id,user_id,project_id,state,job_id,created_at,expires_at) VALUES('batch','owner','project','consuming','job','2026-09-01T00:00:00Z','2026-09-01T06:00:00Z')`,
		`INSERT INTO clip_source_leases(id,batch_id,user_id,object_key,filename,content_type,fingerprint,declared_bytes,actual_bytes,duration_ms,width,height,state,ordinal) VALUES('source','batch','owner','clip-inputs/owner/original','source.mp4','video/mp4','fingerprint',123,123,15000,1080,1920,'ready',0)`,
		`INSERT INTO clip_generation_quotes(id,user_id,project_id,batch_id,input_digest,pricing_json,max_credits,expires_at) VALUES('quote','owner','project','batch','digest','{}',3,'2099-01-01T00:00:00Z')`,
	} {
		if _, err = d.Writer.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err = Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	var job, batch, canonical, key, expires, plan, result string
	err = d.Reader.QueryRow(`SELECT a.job_id,a.batch_id,l.canonical_id,l.object_key,l.retention_expires_at,p.edit_plan_json,p.result_key FROM clip_source_attempts a JOIN clip_source_leases l ON l.batch_id=a.batch_id JOIN clip_projects p ON p.id=a.project_id`).Scan(&job, &batch, &canonical, &key, &expires, &plan, &result)
	if err != nil || job != "job" || batch != "batch" || canonical != "source" || key != "clip-inputs/owner/original" || expires != "2026-09-01T06:00:00Z" || plan != "plan" || result != "result.mp4" {
		t.Fatal("migration changed retained identity or revived expired originals", err, job, batch, canonical, key, expires, plan, result)
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(ctx, 46); err != nil {
		t.Fatal("retention rollback failed", err)
	}
	if err = Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = d.Reader.QueryRow(`SELECT COUNT(*) FROM clip_source_attempts WHERE job_id='job'`).Scan(&n); err != nil || n != 1 {
		t.Fatal("round trip duplicated or lost binding", n, err)
	}
}
