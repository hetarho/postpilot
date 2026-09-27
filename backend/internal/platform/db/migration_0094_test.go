package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// TMPL-18: 94 removes every `<note>…</note>` from a stored post-template body — one, two, one alone
// on its line — leaves a body without one, and touches no updated_at.
func TestMigration0094DropsNotesFromTemplateBodies(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 93); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at); err != nil {
		t.Fatal(err)
	}
	bodies := map[string][2]string{
		"one":  {"<write>인트로</write><note>존댓말</note>", "<write>인트로</write>"},
		"two":  {"<note>a</note><write>인트로</write><note>b</note>", "<write>인트로</write>"},
		"line": {"<write>인트로</write>\n<note>광고처럼 들리지 않게</note>\n끝", "<write>인트로</write>\n\n끝"},
		"none": {"<write>인트로</write>", "<write>인트로</write>"},
	}
	for id, body := range bodies {
		if _, err := handle.Writer.Exec(`INSERT INTO templates(id,user_id,name,body,created_at,updated_at) VALUES(?,'alice',?,?,?,?)`, id, id, body[0], at, at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := provider.UpTo(ctx, 94); err != nil {
		t.Fatal(err)
	}
	for id, body := range bodies {
		var got, updated string
		if err := handle.Reader.QueryRow(`SELECT body, updated_at FROM templates WHERE id=?`, id).Scan(&got, &updated); err != nil {
			t.Fatal(err)
		}
		if got != body[1] || updated != at {
			t.Errorf("%s = %q (updated %s), want %q", id, got, updated, body[1])
		}
	}
}
