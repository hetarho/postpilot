package db

import (
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// MODEL r39 (T891): 170 gives every writing test an output kind that existing rows read as a
// post, and adds the photos a photo observation test owns.
func TestMigration0170AddsTestOutputAndPhotos(t *testing.T) {
	h := openTemp(t)
	ctx := t.Context()
	if err := migrate(ctx, h.Writer, migrationsBefore(t, "0170_")); err != nil {
		t.Fatal(err)
	}
	exec := func(statement string, args ...any) error {
		_, err := h.Writer.Exec(statement, args...)
		return err
	}
	const at = "2026-11-17T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','h','` + at + `'),('bob','h','` + at + `')`,
		`INSERT INTO writing_tests(id,user_id,operation_key,fingerprint,kind,factor,model_stage,count,status,context,common_hash,prompt_version,created_at,updated_at) VALUES('old-test','alice','old-request','fp','ab','model','write',2,'review','{"private":"retained"}','hash','v1','` + at + `','` + at + `')`,
	} {
		if err := exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, h.Writer); err != nil {
		t.Fatal(err)
	}

	var output, private string
	if err := h.Reader.QueryRow(`SELECT output,context FROM writing_tests WHERE id='old-test'`).Scan(&output, &private); err != nil || output != "post" || private != `{"private":"retained"}` {
		t.Fatalf("an existing test reads output %q context %q: %v", output, private, err)
	}
	if err := exec(`UPDATE writing_tests SET output='observation' WHERE id='old-test'`); err != nil {
		t.Fatalf("observation refused: %v", err)
	}
	if err := exec(`UPDATE writing_tests SET output='video' WHERE id='old-test'`); err == nil {
		t.Fatal("an unknown output kind was accepted")
	}

	photo := func(id, user, key, status string, test any) error {
		return exec(`INSERT INTO writing_test_photos(id,user_id,test_id,filename,object_key,status,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?)`, id, user, test, id+".jpg", key, status, at, at)
	}
	if err := photo("waiting", "alice", "tests/alice/waiting", "pending", nil); err != nil {
		t.Fatalf("an unattached photo refused: %v", err)
	}
	if err := photo("held", "alice", "tests/alice/held", "ready", "old-test"); err != nil {
		t.Fatalf("a photo held by its owner's test refused: %v", err)
	}
	if err := photo("stolen", "bob", "tests/bob/stolen", "ready", "old-test"); err == nil {
		t.Fatal("a photo was held by another owner's test")
	}
	if err := photo("dupe", "alice", "tests/alice/held", "pending", nil); err == nil {
		t.Fatal("an object key was claimed twice")
	}
	if err := photo("odd", "alice", "tests/alice/odd", "uploaded", nil); err == nil {
		t.Fatal("an unknown photo status was accepted")
	}
	if err := exec(`UPDATE writing_test_photos SET bytes=-1 WHERE id='held'`); err == nil {
		t.Fatal("a negative size was accepted")
	}

	// Deleting a test removes the photos it holds and not the owner's unattached ones.
	if err := exec(`DELETE FROM writing_tests WHERE id='old-test'`); err != nil {
		t.Fatal(err)
	}
	var remaining string
	if err := h.Reader.QueryRow(`SELECT group_concat(id) FROM writing_test_photos`).Scan(&remaining); err != nil || remaining != "waiting" {
		t.Fatalf("photos after the test went: %q %v", remaining, err)
	}
	var violations int
	if err := h.Reader.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign key violations %d: %v", violations, err)
	}
}

// 170's Down removes the table and the column and keeps the tests themselves.
func TestMigration0170DownKeepsWritingTests(t *testing.T) {
	h := openTemp(t)
	ctx := t.Context()
	if err := Migrate(ctx, h.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-11-17T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','h','` + at + `')`,
		`INSERT INTO writing_tests(id,user_id,operation_key,fingerprint,kind,factor,model_stage,count,status,context,common_hash,prompt_version,created_at,updated_at) VALUES('kept','alice','request','fp','ab','model','observe',2,'review','{}','hash','v1','` + at + `','` + at + `')`,
	} {
		if _, err := h.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, h.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(ctx, 169); err != nil {
		t.Fatal(err)
	}
	var table, column int
	if err = h.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='writing_test_photos'`).Scan(&table); err != nil || table != 0 {
		t.Fatalf("the photo table survived Down: %d %v", table, err)
	}
	if err = h.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('writing_tests') WHERE name='output'`).Scan(&column); err != nil || column != 0 {
		t.Fatalf("the output column survived Down: %d %v", column, err)
	}
	var kept int
	if err = h.Reader.QueryRow(`SELECT count(*) FROM writing_tests WHERE id='kept'`).Scan(&kept); err != nil || kept != 1 {
		t.Fatalf("Down lost the test: %d %v", kept, err)
	}
}
