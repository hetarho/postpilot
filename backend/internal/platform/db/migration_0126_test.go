package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// TMPL-63: 126 lets an owner cancel a template request, so both cancellation CHECKs name
// `template_request`. The widened CHECKs rebuild generation_jobs, whose children cascade from
// it, so the rebuild must keep every job and its checkpoint; the way back drops only the
// requests that were asked to stop.
func TestMigration0126AdmitsTemplateRequestCancellationWithoutLosingJobs(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 125); err != nil {
		t.Fatal(err)
	}
	const at = "2026-10-01T00:00:00Z"
	exec := func(statement string) error {
		_, err := handle.Writer.Exec(statement)
		return err
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('clip','alice','성수','vertical',30000,'` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('render','alice','clip','render_clip','running','` + at + `','` + at + `')`,
		`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','alice','render','{"Version":1}')`,
		`INSERT INTO generation_jobs(id,user_id,kind,status,payload,cancellation_policy_version,created_at,updated_at) VALUES('kept','alice','template_request','done','{}',1,'` + at + `','` + at + `')`,
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	// What the old table refused.
	if err := exec(`INSERT INTO generation_jobs(id,user_id,kind,status,cancellation_policy_version,cancel_requested_at,created_at,updated_at) VALUES('early','alice','template_request','running',1,'` + at + `','` + at + `','` + at + `')`); err == nil {
		t.Fatal("the pre-126 table already accepted a cancelled template request")
	}
	if _, err := provider.UpTo(ctx, 126); err != nil {
		t.Fatal(err)
	}
	for table, where := range map[string]string{
		"generation_jobs":          "id IN ('render','kept')",
		"clip_attempt_checkpoints": "job_id='render'",
	} {
		var kept int
		want := 1
		if table == "generation_jobs" {
			want = 2
		}
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM ` + table + ` WHERE ` + where).Scan(&kept); err != nil || kept != want {
			t.Fatalf("the rebuild lost rows of %s: %d %v", table, kept, err)
		}
	}
	for _, statement := range []string{
		`INSERT INTO generation_jobs(id,user_id,kind,status,cancellation_policy_version,created_at,updated_at) VALUES('asked','alice','template_request','running',1,'` + at + `','` + at + `')`,
		`UPDATE generation_jobs SET cancel_requested_at='` + at + `' WHERE id='asked'`,
		`UPDATE generation_jobs SET status='cancelled', finished_at='` + at + `' WHERE id='asked'`,
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("after 126 %q: %v", statement, err)
		}
	}
	// Only under the cancellation policy, as every charged kind.
	if err := exec(`INSERT INTO generation_jobs(id,user_id,kind,status,cancel_requested_at,created_at,updated_at) VALUES('unversioned','alice','template_request','running','` + at + `','` + at + `','` + at + `')`); err == nil {
		t.Fatal("a template request without the cancellation policy accepted a stop")
	}
	// Every trigger came back: one active clip job per project still holds.
	if err := exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('second','alice','clip','render_clip','queued','` + at + `','` + at + `')`); err == nil {
		t.Fatal("a second active clip job was accepted after the rebuild")
	}

	if _, err := provider.DownTo(ctx, 125); err != nil {
		t.Fatalf("down: %v", err)
	}
	var asked, kept int
	if err := handle.Reader.QueryRow(`SELECT (SELECT count(*) FROM generation_jobs WHERE id='asked'), (SELECT count(*) FROM generation_jobs WHERE id IN ('render','kept'))`).Scan(&asked, &kept); err != nil || asked != 0 || kept != 2 {
		t.Fatalf("the way back kept %d stopped requests and %d other jobs: %v", asked, kept, err)
	}
}
