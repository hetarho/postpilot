package db

import (
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// CLIP-192: a browser render's sampling is not one of the project's attempts, so inserting it
// keeps the checkpoint a failed generation's retry resumes from; the next real attempt, a
// generation or a render, still clears it.
func TestMigration0128KeepsTheCheckpointThroughABrowserRenderSampling(t *testing.T) {
	h := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.UpTo(t.Context(), 128); err != nil {
		t.Fatal(err)
	}
	const at = "2026-10-04T00:00:00Z"
	exec := func(stmt string) {
		t.Helper()
		if _, err := h.Writer.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	checkpoints := func() int {
		t.Helper()
		var n int
		if err := h.Reader.QueryRow(`SELECT count(*) FROM clip_attempt_checkpoints WHERE project_id='clip'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	job := func(id, kind string) {
		t.Helper()
		exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('` + id + `','alice','clip','` + kind + `','done','` + at + `','` + at + `')`)
	}
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`)
	exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('clip','alice','clip','vertical',30000,'` + at + `','` + at + `')`)
	exec(`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('failed','alice','clip','generate_clip','failed','` + at + `','` + at + `')`)
	exec(`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','alice','failed','{"Version":1}')`)

	job("sampling", "sample_browser_render")
	if checkpoints() != 1 {
		t.Fatal("a browser render's sampling dropped the failed generation's checkpoint")
	}
	for _, kind := range []string{"generate_clip", "render_clip"} {
		exec(`INSERT OR REPLACE INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','alice','failed','{"Version":1}')`)
		job("next-"+kind, kind)
		if checkpoints() != 0 {
			t.Fatalf("a new %s attempt kept the old checkpoint", kind)
		}
	}

	if _, err = p.DownTo(t.Context(), 127); err != nil {
		t.Fatalf("down: %v", err)
	}
	exec(`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','alice','failed','{"Version":1}')`)
	job("sampling-before", "sample_browser_render")
	if checkpoints() != 0 {
		t.Fatal("the way back kept the narrowed trigger")
	}
	var indexes int
	if err := h.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name IN ('generation_jobs_project_latest_idx','generation_jobs_post_idx','usage_admissions_job_idx','usage_events_job_idx')`).Scan(&indexes); err != nil || indexes != 0 {
		t.Fatalf("the way back left %d of the new indexes: %v", indexes, err)
	}
}
