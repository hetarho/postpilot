package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// CLIP-166: 102 gives every video template a design selection at the shared defaults.
func TestMigration0102GivesTemplatesADesignSelection(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 101); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO video_templates(id,user_id,name,composition_body,created_at,updated_at) VALUES('tpl','alice','여행','<clip version="1"/>','` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if _, err := provider.UpTo(ctx, 102); err != nil {
		t.Fatal(err)
	}
	var intro, outro, styles string
	if err := handle.Reader.QueryRow(`SELECT intro_preset, outro_preset, allowed_caption_styles FROM video_templates WHERE id='tpl'`).Scan(&intro, &outro, &styles); err != nil || intro != "" || outro != "" || styles != "[]" {
		t.Fatalf("an existing template reads %q %q %q (%v), want the shared defaults", intro, outro, styles, err)
	}
	if _, err := provider.DownTo(ctx, 101); err != nil {
		t.Fatalf("down: %v", err)
	}
}
