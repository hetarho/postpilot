package db

import (
	"context"
	"encoding/json"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// Change 16 drops voice_profiles.styleguide. A voice analysed before change 02 and never
// re-analysed since has its WHOLE profile in that text and no published structured version, so
// dropping the column would silently reduce a trained voice to an empty one. 0017 publishes the
// version such a voice never had, and leaves every already-versioned voice alone.
func TestMigration0017RescuesAPreStructuredStyleguide(t *testing.T) {
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
	// Stop at 16: the rescue has to be exercised on a database that still HAS the column, and
	// 0017's own Down blanks it, so rolling back and forward again would test nothing.
	if _, err := provider.UpTo(ctx, 16); err != nil {
		t.Fatal(err)
	}
	at := "2026-08-29T00:00:00Z"
	if _, err := handle.Writer.Exec(
		"INSERT INTO users (id, password_hash, created_at) VALUES ('alice','hash',?)", at,
	); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		voiceID, styleguide string
		currentVersion      int
	}{
		{"legacy", "## 1. 종결어미 분포\n해요체", 0},
		{"versioned", "이미 구조화된 프로필이 있는 말투", 4},
		{"blank", "", 0},
	} {
		if _, err := handle.Writer.Exec(
			`INSERT INTO voices(id,user_id,name,source_language,is_default,created_at,updated_at)
			 VALUES(?,'alice',?,'ko',0,?,?)`, row.voiceID, row.voiceID, at, at,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := handle.Writer.Exec(
			`INSERT INTO voice_profiles(voice_id,user_id,styleguide,rules,corpus_version,current_version,updated_at)
			 VALUES(?,'alice',?,'',0,?,?)`, row.voiceID, row.styleguide, row.currentVersion, at,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}

	var snapshot string
	var version int
	if err := handle.Reader.QueryRow(
		`SELECT v.snapshot, p.current_version FROM voice_profile_versions v
		  JOIN voice_profiles p ON p.voice_id = v.voice_id
		 WHERE v.voice_id='legacy'`).Scan(&snapshot, &version); err != nil {
		t.Fatalf("legacy voice lost its analysis: %v", err)
	}
	if version != 1 {
		t.Fatalf("rescued voice head = %d", version)
	}
	var decoded struct {
		Empty   bool
		Lexical struct {
			Description struct{ Value, Source string }
		}
	}
	if err := json.Unmarshal([]byte(snapshot), &decoded); err != nil {
		t.Fatalf("rescued snapshot is not a profile: %v", err)
	}
	if decoded.Empty || decoded.Lexical.Description.Value != "## 1. 종결어미 분포\n해요체" ||
		decoded.Lexical.Description.Source != "analyzed" {
		t.Fatalf("rescued snapshot = %+v", decoded)
	}
	// A voice that already had a version, and one that never had an analysis, are untouched.
	for _, row := range []struct {
		voiceID string
		want    int
	}{{"versioned", 4}, {"blank", 0}} {
		var head, versions int
		if err := handle.Reader.QueryRow(
			`SELECT current_version, (SELECT count(*) FROM voice_profile_versions v WHERE v.voice_id=?)
			   FROM voice_profiles WHERE voice_id=?`, row.voiceID, row.voiceID).Scan(&head, &versions); err != nil {
			t.Fatal(err)
		}
		if head != row.want || versions != 0 {
			t.Fatalf("%s head=%d versions=%d want head=%d", row.voiceID, head, versions, row.want)
		}
	}
}
