package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

const at0091 = "2026-09-27T00:00:00Z"

// GEN-31: after 91 a second active seed of one voice is refused by the database itself, as
// every other voice-owned kind already was; a seed of another voice and a finished one are
// not; and 91's Down restores the guard that let the second seed through.
func TestMigration0091GuardsConcurrentSeedsOfOneVoice(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 91); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at0091 + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,'` + at0091 + `','` + at0091 + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-b','alice','리뷰',0,'` + at0091 + `','` + at0091 + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	seed := func(id, voice, status string) error {
		_, err := handle.Writer.Exec(`INSERT INTO generation_jobs(id,user_id,voice_id,kind,status,created_at,updated_at)
			VALUES(?,'alice',?,'seed_voice',?,?,?)`, id, voice, status, at0091, at0091)
		return err
	}
	if err := seed("seed-1", "voice-a", "queued"); err != nil {
		t.Fatal(err)
	}
	if err := seed("seed-2", "voice-a", "running"); err == nil || !strings.Contains(err.Error(), "active voice job already exists") {
		t.Fatalf("a second active seed of one voice was admitted: %v", err)
	}
	if err := seed("seed-3", "voice-b", "queued"); err != nil {
		t.Fatalf("a seed of another voice was refused: %v", err)
	}
	if err := seed("seed-4", "voice-a", "done"); err != nil {
		t.Fatalf("a finished seed was refused: %v", err)
	}
	var planned string
	if err := handle.Reader.QueryRow(`SELECT sql FROM sqlite_master WHERE name='generation_jobs_active_voice_kind_idx'`).Scan(&planned); err != nil || !strings.Contains(planned, "'seed_voice'") {
		t.Fatalf("index does not name seed_voice: %q %v", planned, err)
	}

	if _, err := handle.Writer.Exec(`DELETE FROM generation_jobs`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 90); err != nil {
		t.Fatal(err)
	}
	if err := seed("seed-5", "voice-a", "queued"); err != nil {
		t.Fatal(err)
	}
	if err := seed("seed-6", "voice-a", "queued"); err != nil {
		t.Fatalf("90's guard, restored by Down, never named seed_voice: %v", err)
	}
}
