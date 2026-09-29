package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

var retiredVoiceTables0106 = []string{
	"voice_rule_evidence", "voice_rule_confirmations", "voice_rule_comparison_candidates",
	"voice_rule_comparisons", "voice_profile_validation_items", "voice_profile_validations",
	"voice_sentence_feedback", "voice_authored_sources", "voice_contrast_rules", "voice_learning_events",
}

// VOICE r5: 106 drops everything learned from finalized posts while each of those tables
// holds a row, fails the queued and running jobs of the three retired kinds and keeps their
// finished ones, and rebuilds the voice guards without them: a voice with a running job
// still refuses deletion and one with none deletes.
func TestMigration0106DropsLearningAndKeepsTheVoiceGuards(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 105); err != nil {
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
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-b','alice','리뷰',0,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-c','alice','일상',0,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('post','alice','voice-b','` + at + `','` + at + `')`,
		`INSERT INTO voice_learning_events(id,user_id,voice_id,post_slug,baseline_revision,input_hash,baseline_content,final_content,model_ref,status,created_at) VALUES('event','alice','voice-b','post',1,'hash','{}','{}','p/m','done','` + at + `')`,
		`INSERT INTO voice_authored_sources(id,user_id,voice_id,post_slug,learning_event_id,title,tags,body,excerpt,created_at) VALUES('source','alice','voice-b','post','event','제목','[]','본문','발췌','` + at + `')`,
		`INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('rule','alice','voice-b','~요로 끝낸다','key','endings',3,'active','diff','` + at + `','` + at + `')`,
		`INSERT INTO voice_rule_evidence(id,user_id,voice_id,rule_id,event_id,origin,payload_ref,created_at) VALUES('evidence','alice','voice-b','rule','event','diff','ref','` + at + `')`,
		`INSERT INTO voice_rule_confirmations(id,user_id,voice_id,rule_id,proposed_statement,event_id,status,created_at) VALUES('confirmation','alice','voice-b','rule','~다로 끝낸다','event','pending','` + at + `')`,
		`INSERT INTO voice_rule_comparisons(id,user_id,voice_id,rule_id,source_id,profile_version,model_ref,target_length,input_snapshot,rule_on_side,status,created_at) VALUES('comparison','alice','voice-b','rule','source',1,'p/m',1000,'{}','left','review','` + at + `')`,
		`INSERT INTO voice_rule_comparison_candidates(id,comparison_id,display_side,output,status) VALUES('candidate','comparison','left','글','succeeded')`,
		`INSERT INTO voice_profile_validations(id,user_id,voice_id,profile_version,analyze_model_ref,write_model_ref,judge_enabled,status,created_at) VALUES('validation','alice','voice-b',1,'p/m','p/m',0,'partial','` + at + `')`,
		`INSERT INTO voice_profile_validation_items(id,validation_id,source_id,voice_id,user_id,position,status) VALUES('item','validation','source','voice-b','alice',0,'pending')`,
		`INSERT INTO voice_sentence_feedback(id,user_id,voice_id,post_slug,sentence_ref,kind,payload_ref,processing_state,created_at) VALUES('feedback','alice','voice-b','post','s1','thumbs','ref','pending','` + at + `')`,
		job("learn-queued", "voice-b", "learn_voice", "queued"),
		job("compare-running", "voice-b", "compare_voice_rule", "running"),
		job("validate-done", "voice-b", "validate_voice_profile", "done"),
		job("analyze-running", "voice-c", "analyze_voice", "running"),
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	// Before 106 the undecided comparison and validation alone keep voice-b alive.
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-b'`, at); err == nil || !strings.Contains(err.Error(), "voice has publishable work") {
		t.Fatalf("the pre-106 guard let a voice with undecided work go: %v", err)
	}

	if _, err := provider.UpTo(ctx, 106); err != nil {
		t.Fatal(err)
	}
	for _, table := range retiredVoiceTables0106 {
		var n int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=?`, table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s survived 106: %d %v", table, n, err)
		}
	}
	for id, want := range map[string]string{
		"learn-queued": "failed", "compare-running": "failed", "validate-done": "done", "analyze-running": "running",
	} {
		var status string
		var reason, finished *string
		if err := handle.Reader.QueryRow(`SELECT status, error_reason, finished_at FROM generation_jobs WHERE id=?`, id).Scan(&status, &reason, &finished); err != nil {
			t.Fatal(err)
		}
		if status != want {
			t.Fatalf("%s is %s after 106, want %s", id, status, want)
		}
		if want == "failed" && (reason == nil || *reason != "JOB_INTERRUPTED" || finished == nil) {
			t.Fatalf("%s failed without an interruption: %v %v", id, reason, finished)
		}
	}
	for _, object := range []string{"generation_jobs_active_voice_kind_idx", "generation_jobs_refuse_duplicate_voice_work", "voices_refuse_publishable_work_on_delete"} {
		var sql string
		if err := handle.Reader.QueryRow(`SELECT sql FROM sqlite_master WHERE name=?`, object).Scan(&sql); err != nil {
			t.Fatalf("%s is missing after 106: %v", object, err)
		}
		for _, gone := range []string{"learn_voice", "compare_voice_rule", "validate_voice_profile", "voice_rule_comparisons", "voice_profile_validations"} {
			if strings.Contains(sql, gone) {
				t.Fatalf("%s still names %s: %s", object, gone, sql)
			}
		}
	}

	// voice-c's running analysis still refuses its deletion and a second analysis of it.
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-c'`, at); err == nil || !strings.Contains(err.Error(), "voice has publishable work") {
		t.Fatalf("a voice with a running job was deleted: %v", err)
	}
	if err := exec(job("analyze-again", "voice-c", "analyze_voice", "queued")); err == nil || !strings.Contains(err.Error(), "active voice job already exists") {
		t.Fatalf("a second active analysis of one voice was admitted: %v", err)
	}
	// voice-b has nothing running any more and deletes.
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-b'`, at); err != nil {
		t.Fatalf("a voice with no running work was refused: %v", err)
	}
	var violations int
	rows, err := handle.Reader.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		violations++
	}
	rows.Close()
	if violations != 0 {
		t.Fatalf("106 left %d foreign key violations", violations)
	}
}
