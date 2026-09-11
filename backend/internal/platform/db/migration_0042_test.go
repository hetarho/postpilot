package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// 0042 adds the preset, the disclosure and the CTA without guessing any of them:
// a template nobody categorised stays uncategorised, and a clip nobody declared
// a campaign type for stays undeclared — which is what the generation gate then
// refuses, rather than rendering a clip with an invented disclosure.
func TestMigration0042LeavesExistingRowsUndeclared(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 41); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-11T00:00:00Z"
	if _, err := handle.Writer.Exec(
		"INSERT INTO users (id, password_hash, created_at) VALUES ('alice','hash',?)", at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(
		`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,accent,created_at,updated_at)
		 VALUES('legacy','alice','여행','[]','','["clean"]','teal',?,?)`, at, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(
		`INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,created_at,updated_at)
		 VALUES('project','alice','제주','legacy','vertical',20000,?,?)`, at, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}

	var preset, disclosure, cta string
	if err := handle.Reader.QueryRow(`SELECT preset FROM video_templates WHERE id='legacy'`).Scan(&preset); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow(`SELECT disclosure, cta FROM clip_projects WHERE id='project'`).Scan(&disclosure, &cta); err != nil {
		t.Fatal(err)
	}
	if preset != "" || disclosure != "" || cta != "" {
		t.Fatalf("a value was invented: preset=%q disclosure=%q cta=%q", preset, disclosure, cta)
	}
	// The columns are NOT NULL, so a writer that omits them gets the empty
	// string rather than a null the domain would have to handle twice.
	if _, err := handle.Writer.Exec(
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at)
		 VALUES('next','alice','새 클립','square',15000,?,?)`, at, at,
	); err != nil {
		t.Fatal(err)
	}
	var nulls int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM clip_projects WHERE disclosure IS NULL OR cta IS NULL`).Scan(&nulls); err != nil {
		t.Fatal(err)
	}
	if nulls != 0 {
		t.Fatalf("%d null disclosure/cta rows", nulls)
	}
}
