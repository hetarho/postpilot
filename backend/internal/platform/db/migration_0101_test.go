package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// CLIP-177, CLIP-181: 101 lets the storyline call and the storyline request be cancelled like a
// generation and consume a quote, keeps every job from before, and rolls back.
func TestMigration0101MakesTheStorylineJobsCancellableAndQuoteConsuming(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 100); err != nil {
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
	exec(`INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at) VALUES('before','alice','generate_clip','done',?,?)`, at, at)
	cancelled := `INSERT INTO generation_jobs(id,user_id,kind,status,cancel_requested_at,cancellation_policy_version,created_at,updated_at) VALUES(?,'alice',?,'cancelled',?,1,?,?)`
	if _, err := handle.Writer.Exec(cancelled, "early", "storyline_clip", at, at, at); err == nil {
		t.Fatal("a storyline job was cancelled before 101")
	}

	if _, err := provider.UpTo(ctx, 101); err != nil {
		t.Fatal(err)
	}
	exec(cancelled, "storyline", "storyline_clip", at, at, at)
	exec(cancelled, "request", "revise_storyline_clip", at, at, at)
	var n int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM generation_jobs`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("jobs after 101 = %d (%v), want the old one and both storyline jobs", n, err)
	}
	if _, err := handle.Writer.Exec(cancelled, "post", "storyline", at, at, at); err == nil {
		t.Fatal("a post's storyline job was cancelled as a clip job")
	}

	if _, err := provider.DownTo(ctx, 100); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM generation_jobs`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("jobs after the rollback = %d (%v), want the old one", n, err)
	}
}
