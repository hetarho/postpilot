package db

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// 0043 gives a template a target length and a tag count without inventing either: a template
// written before this migration had no opinion about them, and NULL is exactly how that
// reads afterwards (TEMPLATE-47). A backfilled 1000 would make every existing template start
// overwriting the posts it is assigned to.
func TestMigration0043LeavesExistingTemplatesWithoutNumbers(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 42); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-11T00:00:00Z"
	if _, err := handle.Writer.Exec(
		"INSERT INTO users (id, password_hash, created_at) VALUES ('alice','hash',?)", at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(
		`INSERT INTO templates(id,user_id,name,description,body,created_at,updated_at)
		 VALUES('legacy','alice','방문 리뷰','','<write>본문</write>',?,?)`, at, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}

	var length, tags sql.NullInt64
	if err := handle.Reader.QueryRow(
		`SELECT target_length, tag_count FROM templates WHERE id='legacy'`,
	).Scan(&length, &tags); err != nil {
		t.Fatal(err)
	}
	if length.Valid || tags.Valid {
		t.Fatalf("a number was invented: target_length=%v tag_count=%v", length, tags)
	}

	// A writer that names neither column gets NULL rather than a zero the domain would then
	// have to read as "0 characters, 0 tags".
	if _, err := handle.Writer.Exec(
		`INSERT INTO templates(id,user_id,name,description,body,created_at,updated_at)
		 VALUES('next','alice','짧은 소식','','<write>본문</write>',?,?)`, at, at,
	); err != nil {
		t.Fatal(err)
	}
	var set int
	if err := handle.Reader.QueryRow(
		`SELECT count(*) FROM templates WHERE target_length IS NOT NULL OR tag_count IS NOT NULL`,
	).Scan(&set); err != nil {
		t.Fatal(err)
	}
	if set != 0 {
		t.Fatalf("%d templates carry a number nobody authored", set)
	}
}
