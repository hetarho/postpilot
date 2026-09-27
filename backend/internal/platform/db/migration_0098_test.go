package db

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// CLIP-178: 98 gives every clip project a storyline column that reads as none until a flow call
// stores one, and refuses a value that is not JSON.
func TestMigration0098AddsAClipStorylineThatStartsAsNone(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 97); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('before','alice','성수','vertical',30000,'` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if _, err := provider.UpTo(ctx, 98); err != nil {
		t.Fatal(err)
	}
	var storyline sql.NullString
	if err := handle.Reader.QueryRow(`SELECT storyline_json FROM clip_projects WHERE id='before'`).Scan(&storyline); err != nil || storyline.Valid {
		t.Fatalf("a clip from before 98 reads storyline %v (%v), want none", storyline, err)
	}
	stored := `{"Paragraphs":[{"Text":"가게 앞","ObservationIDs":["source/0"]}],"MadeWithSources":["source"]}`
	if _, err := handle.Writer.Exec(`UPDATE clip_projects SET storyline_json=? WHERE id='before'`, stored); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`UPDATE clip_projects SET storyline_json='not json' WHERE id='before'`); err == nil {
		t.Fatal("a storyline that is not JSON was stored")
	}
	if _, err := provider.DownTo(ctx, 97); err != nil {
		t.Fatalf("down: %v", err)
	}
}
