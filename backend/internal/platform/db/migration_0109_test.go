package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// VOICE r5 (T468): 109 fails the queued and running seeds, drops the voice's language, keeps
// the one-active-job guard for analyze_voice alone, and lets the 기본 and the last voice go
// while a voice with a running job still refuses.
func TestMigration0109MakesTheDefaultOptionalAndVoicesLanguageless(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 108); err != nil {
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
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO users(id,password_hash,created_at) VALUES('bob','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,source_language,created_at,updated_at) VALUES('voice-a','alice','기본',1,'ko','` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-b','alice','리뷰',0,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-bob','bob','하나',1,'` + at + `','` + at + `')`,
		job("seed-queued", "voice-b", "seed_voice", "queued"),
		job("seed-running", "voice-a", "seed_voice", "running"),
		job("seed-done", "voice-a", "seed_voice", "done"),
		job("analysis-running", "voice-b", "analyze_voice", "running"),
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	// Before 109 the 기본 and the last voice refuse.
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-bob'`, at); err == nil || !strings.Contains(err.Error(), "default voice cannot be deleted") {
		t.Fatalf("the pre-109 guard let the 기본 go: %v", err)
	}

	if _, err := provider.UpTo(ctx, 109); err != nil {
		t.Fatal(err)
	}
	count := func(query string) int {
		t.Helper()
		var n int
		if err := handle.Reader.QueryRow(query).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for id, want := range map[string]string{"seed-queued": "failed", "seed-running": "failed", "seed-done": "done", "analysis-running": "running"} {
		var status, reason string
		if err := handle.Reader.QueryRow(`SELECT status, coalesce(error_reason,'') FROM generation_jobs WHERE id=?`, id).Scan(&status, &reason); err != nil {
			t.Fatal(err)
		}
		if status != want || (want == "failed" && reason != "JOB_INTERRUPTED") {
			t.Fatalf("%s after 109 = %s %s, want %s", id, status, reason, want)
		}
	}
	if n := count(`SELECT count(*) FROM pragma_table_info('voices') WHERE name='source_language'`); n != 0 {
		t.Fatal("voices still carries source_language")
	}
	for _, name := range []string{"generation_jobs_active_voice_kind_idx", "generation_jobs_refuse_duplicate_voice_work"} {
		var sql string
		if err := handle.Reader.QueryRow(`SELECT sql FROM sqlite_master WHERE name=?`, name).Scan(&sql); err != nil || strings.Contains(sql, "seed_voice") || !strings.Contains(sql, "'analyze_voice'") {
			t.Fatalf("%s = %q %v", name, sql, err)
		}
	}
	// A second active analysis of one voice is still refused.
	if err := exec(job("analysis-again", "voice-b", "analyze_voice", "queued")); err == nil || !strings.Contains(err.Error(), "active voice job already exists") {
		t.Fatalf("a second active analysis was accepted: %v", err)
	}
	// voice-b has a running analysis, so it stays; the 기본 of alice and bob's only voice go.
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-b'`, at); err == nil || !strings.Contains(err.Error(), "voice has publishable work") {
		t.Fatalf("a voice with a running job was deleted: %v", err)
	}
	if err := exec(`UPDATE generation_jobs SET status='failed' WHERE id='analysis-running'`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`UPDATE voices SET deleted_at=?, is_default=0 WHERE id='voice-a'`, at); err != nil {
		t.Fatalf("the 기본 was refused: %v", err)
	}
	if err := exec(`UPDATE voices SET deleted_at=?, is_default=0 WHERE id='voice-bob'`, at); err != nil {
		t.Fatalf("the last voice was refused: %v", err)
	}
	if n := count(`SELECT count(*) FROM voices WHERE user_id='bob' AND deleted_at IS NULL`); n != 0 {
		t.Fatalf("bob still has %d active voices", n)
	}
	// At most one 기본 per account still holds.
	if err := exec(`UPDATE voices SET deleted_at=NULL WHERE id='voice-a'`); err != nil {
		t.Fatal(err)
	}
	if err := exec(`UPDATE voices SET is_default=1 WHERE id IN ('voice-a','voice-b')`); err == nil {
		t.Fatal("two active defaults were accepted")
	}
}
