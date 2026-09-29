package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// VOICE r5 (T473): 112 creates voice_checks and guards 검증 like the analysis — one queued or
// running check_voice per voice, beside at most one analysis — and a voice with a running 검증
// cannot be deleted.
func TestMigration0112GuardsOneCheckPerVoice(t *testing.T) {
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
	exec := func(statement string, args ...any) error {
		_, err := handle.Writer.Exec(statement, args...)
		return err
	}
	job := func(id, voice, kind, status string) string {
		return `INSERT INTO generation_jobs(id,user_id,voice_id,kind,status,created_at,updated_at) VALUES('` +
			id + `','alice','` + voice + `','` + kind + `','` + status + `','` + at + `','` + at + `')`
	}
	check := func(id, status string) string {
		return `INSERT INTO voice_checks(id,user_id,voice_id,prompt_key,material_id,analysis_created_at,projection,write_model,status,created_at,updated_at)
			VALUES('` + id + `','alice','voice-a','opening_greeting','m1','` + at + `','[말투]','stub/write','` + status + `','` + at + `','` + at + `')`
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','리뷰',0,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-b','alice','일상',0,'` + at + `','` + at + `')`,
		check("check-1", "done"),
		job("analysis", "voice-a", "analyze_voice", "running"),
		job("check", "voice-a", "check_voice", "queued"),
		job("other-check", "voice-b", "check_voice", "running"),
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if err := exec(check("check-2", "waiting")); err == nil {
		t.Fatal("an unknown check status was accepted")
	}
	if err := exec(job("check-again", "voice-a", "check_voice", "queued")); err == nil || !strings.Contains(err.Error(), "active voice job already exists") {
		t.Fatalf("a second active 검증 of one voice was accepted: %v", err)
	}
	if err := exec(job("analysis-again", "voice-a", "analyze_voice", "queued")); err == nil {
		t.Fatal("a second active analysis was accepted")
	}
	var plan string
	if err := handle.Reader.QueryRow(`SELECT sql FROM sqlite_master WHERE name='generation_jobs_active_voice_kind_idx'`).Scan(&plan); err != nil || !strings.Contains(plan, "'check_voice'") {
		t.Fatalf("the voice-kind index = %q %v", plan, err)
	}
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-b'`, at); err == nil || !strings.Contains(err.Error(), "voice has publishable work") {
		t.Fatalf("a voice with a running 검증 was deleted: %v", err)
	}
	if err := exec(`UPDATE generation_jobs SET status='done' WHERE id='other-check'`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-b'`, at); err != nil {
		t.Fatalf("a voice with a finished 검증 was refused: %v", err)
	}
}
