package db

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// POST-99: 96 gives every post a storyline column that reads as none until a write stores one,
// and refuses a value that is not JSON.
func TestMigration0096AddsAStorylineThatStartsAsNone(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 95); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,created_at,updated_at) VALUES('before','alice','voice-a','성수','` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if _, err := provider.UpTo(ctx, 96); err != nil {
		t.Fatal(err)
	}
	var storyline sql.NullString
	if err := handle.Reader.QueryRow(`SELECT storyline FROM posts WHERE slug='before'`).Scan(&storyline); err != nil || storyline.Valid {
		t.Fatalf("a post from before 96 reads storyline %v (%v), want none", storyline, err)
	}
	stored := `{"paragraphs":[{"text":"가게 앞","files":["a.jpg"]}],"edited_by_hand":false,"made_with":["a.jpg"]}`
	if _, err := handle.Writer.Exec(`UPDATE posts SET storyline=? WHERE slug='before'`, stored); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`UPDATE posts SET storyline='not json' WHERE slug='before'`); err == nil {
		t.Fatal("a storyline that is not JSON was stored")
	}
	if _, err := provider.DownTo(ctx, 95); err != nil {
		t.Fatalf("down: %v", err)
	}
}
