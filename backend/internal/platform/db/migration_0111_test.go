package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// VOICE r5 (T471): 111 creates voice_analyses, drops the versioned profile with its per-version
// samples and overrides, converts nothing — so every voice reads as not made — and clears every
// 기본, since only a made voice may be one.
func TestMigration0111StartsEveryVoiceOverAsNotMade(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 110); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-29T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,'` + at + `','` + at + `')`,
		`INSERT INTO voice_profiles(voice_id,user_id,current_version,updated_at) VALUES('voice-a','alice',1,'` + at + `')`,
		`INSERT INTO voice_profile_versions(id,user_id,voice_id,version,snapshot,origin,created_at) VALUES('v1','alice','voice-a',1,'{}','analysis','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if _, err := provider.UpTo(ctx, 111); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"voice_profiles", "voice_profile_versions", "voice_version_samples", "voice_manual_overrides"} {
		var n int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s survived 111: %d %v", table, n, err)
		}
	}
	var analyses, defaults int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM voice_analyses`).Scan(&analyses); err != nil || analyses != 0 {
		t.Fatalf("an old profile was converted: %d %v", analyses, err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM voices WHERE is_default = 1`).Scan(&defaults); err != nil || defaults != 0 {
		t.Fatalf("a 기본 survived: %d %v", defaults, err)
	}
	insert := func(slot string) error {
		_, err := handle.Writer.Exec(`INSERT INTO voice_analyses(voice_id,user_id,slot,snapshot,material_ids,analyze_model,created_at)
			VALUES('voice-a','alice',?,'{}','[]','stub/analyze',?)`, slot, at)
		return err
	}
	if err := insert("current"); err != nil {
		t.Fatalf("a current analysis: %v", err)
	}
	if err := insert("previous"); err != nil {
		t.Fatalf("a previous analysis: %v", err)
	}
	if err := insert("current"); err == nil {
		t.Fatal("a second current analysis was accepted")
	}
	if err := insert("older"); err == nil {
		t.Fatal("a third slot was accepted")
	}
}
