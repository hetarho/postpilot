package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// MODEL r21 (T474): 113 marks every existing comparison as post-sourced and takes a
// voice-sourced one with its prompt and answer ids and no post.
func TestMigration0113AddsTheReflectionSource(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 112); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-29T00:00:00Z"
	insert := func(extra, values string) error {
		_, err := handle.Writer.Exec(`INSERT INTO model_experiments(id,user_id,stage,status,input_hash,prompt_version,created_at` + extra + `)
			VALUES(` + values + `)`)
		return err
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','리뷰',0,'` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := insert("", `'old','alice','write','review','h','v','`+at+`'`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 113); err != nil {
		t.Fatal(err)
	}
	var source string
	if err := handle.Reader.QueryRow(`SELECT source FROM model_experiments WHERE id='old'`).Scan(&source); err != nil || source != "post" {
		t.Fatalf("an existing comparison reads as %q: %v", source, err)
	}
	if err := insert(",source,voice_id,voice_prompt_key,voice_material_id", `'voiced','alice','write','queued','h','v','`+at+`','voice','voice-a','opening_greeting','m1'`); err != nil {
		t.Fatalf("a voice-sourced comparison: %v", err)
	}
	if err := insert(",source", `'bad','alice','write','queued','h','v','`+at+`','clip'`); err == nil {
		t.Fatal("an unknown source was accepted")
	}
}
