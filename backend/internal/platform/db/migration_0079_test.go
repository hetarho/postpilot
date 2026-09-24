package db

import (
	"context"
	"io/fs"
	"reflect"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0079PreservesExistingWorkAndAccounting(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 78); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('media-owner','hash','2026-09-25T00:00:00Z')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at,result_key,result_id,result_content_type,result_bytes,result_duration_ms,result_created_at,edit_plan_revision,rendered_plan_revision,source_retention_expires_at) VALUES('media-project','media-owner','retained','vertical',15000,'2026-09-25T00:00:00Z','2026-09-25T00:00:00Z','private/result.mp4','result-id','video/mp4',1234,15000,'2026-09-25T00:00:00Z',1,1,'2026-09-26T00:00:00Z')`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,payload,created_at,updated_at) VALUES('existing-render','media-owner','media-project','render_clip','running','frozen','2026-09-25T00:00:00Z','2026-09-25T00:00:00Z')`,
		`INSERT INTO credit_lots(id,user_id,kind,granted,remaining,created_at) VALUES('media-credit','media-owner','purchased',100,75,'2026-09-25T00:00:00Z')`,
		`INSERT INTO clip_source_batches(id,user_id,project_id,state,created_at,expires_at) VALUES('legacy-batch','media-owner','media-project','ready','2026-09-25T00:00:00Z','2026-09-26T00:00:00Z')`,
		`INSERT INTO clip_generation_quotes(id,user_id,project_id,batch_id,input_digest,pricing_json,max_credits,expires_at) VALUES('legacy-quote','media-owner','media-project','legacy-batch','frozen-digest','{}',25,'2026-09-26T00:00:00Z')`,
	} {
		if _, err := d.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	queries := []string{
		`SELECT id,user_id,title,result_key,result_id,result_bytes,edit_plan_revision,rendered_plan_revision,source_retention_expires_at FROM clip_projects WHERE id='media-project'`,
		`SELECT id,user_id,clip_project_id,kind,status,payload,created_at,updated_at FROM generation_jobs WHERE id='existing-render'`,
		`SELECT id,user_id,kind,granted,remaining,created_at FROM credit_lots WHERE id='media-credit'`,
		`SELECT id,user_id,project_id,state,created_at,expires_at FROM clip_source_batches WHERE id='legacy-batch'`,
		`SELECT id,user_id,project_id,batch_id,input_digest,pricing_json,max_credits,expires_at,consumed_job_id FROM clip_generation_quotes WHERE id='legacy-quote'`,
	}
	snapshot := func() [][]any {
		t.Helper()
		out := make([][]any, len(queries))
		for i, query := range queries {
			rows, err := d.Reader.QueryContext(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			cols, err := rows.Columns()
			if err != nil {
				t.Fatal(err)
			}
			values := make([]any, len(cols))
			dest := make([]any, len(cols))
			for j := range values {
				dest[j] = &values[j]
			}
			if !rows.Next() {
				t.Fatal("seed disappeared")
			}
			if err := rows.Scan(dest...); err != nil {
				t.Fatal(err)
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			out[i] = values
		}
		return out
	}
	before := snapshot()
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("upgrade changed data: %v -> %v", before, after)
	}
	for _, table := range []string{"clip_media_stages", "clip_media_attempts", "clip_media_artifacts"} {
		var count int
		if err := d.Reader.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("new table %s: %d %v", table, count, err)
		}
	}
	var violations int
	if err := d.Reader.QueryRowContext(ctx, `SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign keys: %d %v", violations, err)
	}
}
