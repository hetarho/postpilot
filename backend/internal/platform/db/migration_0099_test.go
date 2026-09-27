package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// CLIP-133, CLIP-181: 99 admits a storyline request's kind, keeps every request recorded before
// it, and 100 marks a plan written before it as the writer's own.
func TestMigrations0099And0100RecordStorylineRequestsAndTheWrittenPlan(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 98); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := handle.Writer.Exec(statement, args...); err != nil {
			t.Fatalf("%q: %v", statement, err)
		}
	}
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at)
	exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,edit_plan_json,edit_plan_revision,created_at,updated_at) VALUES('written','alice','성수','vertical',30000,'{}',3,?,?)`, at, at)
	exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('empty','alice','빈','vertical',30000,?,?)`, at, at)
	exec(`INSERT INTO clip_project_requests(id,project_id,user_id,kind,body,created_at) VALUES('r1','written','alice','revision:flow','빠르게',?)`, at)
	if _, err := handle.Writer.Exec(`INSERT INTO clip_project_requests(id,project_id,user_id,kind,body,created_at) VALUES('r0','written','alice','storyline','x',?)`, at); err == nil {
		t.Fatal("a storyline request was recorded before 99")
	}

	if _, err := provider.UpTo(ctx, 100); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO clip_project_requests(id,project_id,user_id,kind,body,created_at) VALUES('r2','written','alice','storyline','가게 소개를 먼저',?)`, at)
	var n int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM clip_project_requests`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("requests after 99 = %d (%v), want the old one and the storyline one", n, err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO clip_project_requests(id,project_id,user_id,kind,body,created_at) VALUES('r3','written','alice','story','x',?)`, at); err == nil {
		t.Fatal("a kind outside the set was recorded")
	}
	var written, empty int
	if err := handle.Reader.QueryRow(`SELECT generated_plan_revision FROM clip_projects WHERE id='written'`).Scan(&written); err != nil || written != 3 {
		t.Fatalf("a plan from before 100 reads generated revision %d (%v), want its own 3", written, err)
	}
	if err := handle.Reader.QueryRow(`SELECT generated_plan_revision FROM clip_projects WHERE id='empty'`).Scan(&empty); err != nil || empty != 0 {
		t.Fatalf("a clip with no plan reads generated revision %d (%v)", empty, err)
	}

	if _, err := provider.DownTo(ctx, 98); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM clip_project_requests`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("requests after the rollback = %d (%v), want the old one", n, err)
	}
}
