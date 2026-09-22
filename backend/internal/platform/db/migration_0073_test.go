package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// Every comparison that predates the origin column was started where its verdict behaved one
// particular way, and the migration has to leave those rows behaving as they did: a write
// comparison came from a surface whose verdict applied, and every decided write verdict owed
// that application whether it completed, failed, or was interrupted before either.
func TestMigration0073BackfillsOriginAndOwedApplications(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 72); err != nil {
		t.Fatal(err)
	}

	at := "2026-09-22T00:00:00Z"
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','voice',1,?,?)`, at, at); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		id, stage, status string
		applied           any
		wantOrigin        string
		wantRequested     int
	}{
		{"write-awaiting-apply", "write", "decided", nil, "editor", 1},
		{"write-applied", "write", "decided", at, "editor", 1},
		{"write-in-review", "write", "review", nil, "editor", 0},
		{"write-dismissed", "write", "dismissed", nil, "editor", 0},
		{"observe-decided", "observe", "decided", nil, "lab", 0},
		{"observe-applied", "observe", "decided", at, "lab", 1},
		{"analyze-in-review", "analyze", "review", nil, "lab", 0},
	}
	for _, row := range rows {
		var language any
		if row.stage == "write" {
			language = "ko"
		}
		// One post each: the pre-0073 unresolved-write index would otherwise refuse the
		// second unresolved write comparison on the same slug.
		slug := row.id + "-post"
		if _, err := handle.Writer.ExecContext(ctx,
			`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES(?,'alice','voice',?,?)`, slug, at, at); err != nil {
			t.Fatal(err)
		}
		if _, err := handle.Writer.ExecContext(ctx,
			`INSERT INTO model_experiments(id,user_id,post_slug,stage,status,input_hash,prompt_version,created_at,applied_at,target_language)
			 VALUES(?,'alice',?,?,?,'hash','v1',?,?,?)`,
			row.id, slug, row.stage, row.status, at, row.applied, language,
		); err != nil {
			t.Fatalf("seed %s: %v", row.id, err)
		}
	}

	if _, err := provider.UpTo(ctx, 73); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		var origin string
		var requested int
		if err := handle.Reader.QueryRowContext(ctx,
			`SELECT origin, apply_requested FROM model_experiments WHERE id = ?`, row.id).Scan(&origin, &requested); err != nil {
			t.Fatalf("read %s: %v", row.id, err)
		}
		if origin != row.wantOrigin || requested != row.wantRequested {
			t.Errorf("%s = origin %q, apply_requested %d; want %q, %d", row.id, origin, requested, row.wantOrigin, row.wantRequested)
		}
	}

	// The rebuilt guard reads the backfilled columns: a write verdict still owing its
	// application holds its post, and one that owes nothing releases it.
	var held int
	if err := handle.Reader.QueryRowContext(ctx,
		`SELECT count(*) FROM model_experiments
		 WHERE stage = 'write' AND post_slug IS NOT NULL
		   AND status = 'decided'
		   AND ((apply_requested = 1 AND applied_at IS NULL) OR (adoption_requested = 1 AND adopted_at IS NULL))`,
	).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != 1 {
		t.Fatalf("decided write comparisons holding their post = %d, want only the unapplied one", held)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO model_experiments(id,user_id,post_slug,stage,status,origin,input_hash,prompt_version,created_at,target_language)
		 VALUES('next','alice','write-applied-post','write','queued','lab','hash','v1',?,'ko')`, at); err != nil {
		t.Fatalf("a comparison after a completed application: %v", err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO model_experiments(id,user_id,post_slug,stage,status,origin,input_hash,prompt_version,created_at,target_language)
		 VALUES('blocked','alice','write-awaiting-apply-post','write','queued','lab','hash','v1',?,'ko')`, at); err == nil {
		t.Fatal("the rebuilt guard accepted a second comparison while an application was still owed")
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO model_experiments(id,user_id,post_slug,stage,status,origin,input_hash,prompt_version,created_at,target_language)
		 VALUES('bad-origin','alice',NULL,'observe','queued','editor-lab','hash','v1',?,NULL)`, at); err == nil {
		t.Fatal("the origin column accepted a value outside its vocabulary")
	}
}
