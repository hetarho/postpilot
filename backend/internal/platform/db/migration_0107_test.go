package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// MODEL r20: 107 deletes every analyze comparison with its candidates and badges, fails the
// job still running one, drops the analyze A/B pair rows while keeping the analyze active
// selection and every other stage's pair, and lets a voice go while a write comparison
// names it; a voice with a running job still refuses.
func TestMigration0107RetiresTheAnalyzeComparison(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 106); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-29T00:00:00Z"
	exec := func(statement string, args ...any) error {
		_, err := handle.Writer.Exec(statement, args...)
		return err
	}
	experimentRow := func(id, voice, post, stage, status string) string {
		postValue := "NULL"
		if post != "" {
			postValue = "'" + post + "'"
		}
		return `INSERT INTO model_experiments(id,user_id,post_slug,voice_id,stage,status,input_hash,prompt_version,created_at) VALUES('` +
			id + `','alice',` + postValue + `,'` + voice + `','` + stage + `','` + status + `','hash','v1','` + at + `')`
	}
	candidate := func(id, experiment, side string) string {
		return `INSERT INTO model_experiment_candidates(id,experiment_id,model_provider_id,model_id,model_label,display_side,status) VALUES('` +
			id + `','` + experiment + `','p','m-` + side + `','M','` + side + `','succeeded')`
	}
	selection := func(stage, slot string) string {
		return `INSERT INTO model_selections(user_id,stage,slot,provider_id,model_id,updated_at) VALUES('alice','` + stage + `','` + slot + `','p','m-` + slot + `','` + at + `')`
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-a','alice','기본',1,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-b','alice','리뷰',0,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-c','alice','일상',0,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,created_at,updated_at) VALUES('post','alice','voice-b','` + at + `','` + at + `')`,
		experimentRow("analyze-running", "voice-a", "", "analyze", "running"),
		experimentRow("analyze-decided", "voice-a", "", "analyze", "decided"),
		experimentRow("write-review", "voice-b", "post", "write", "review"),
		candidate("analyze-left", "analyze-decided", "left"),
		candidate("analyze-right", "analyze-decided", "right"),
		candidate("write-left", "write-review", "left"),
		candidate("write-right", "write-review", "right"),
		`INSERT INTO model_experiment_badges(experiment_id,candidate_id,badge) VALUES('analyze-decided','analyze-left','in_voice')`,
		`INSERT INTO generation_jobs(id,user_id,voice_id,kind,status,payload,created_at,updated_at) VALUES('analyze-job','alice','voice-a','model_experiment','running','analyze-running','` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,user_id,voice_id,kind,status,created_at,updated_at) VALUES('voice-job','alice','voice-c','analyze_voice','running','` + at + `','` + at + `')`,
		selection("analyze", "active"), selection("analyze", "candidate_a"), selection("analyze", "candidate_b"),
		selection("observe", "active"), selection("observe", "candidate_a"), selection("observe", "candidate_b"),
		selection("write", "candidate_a"), selection("write", "candidate_b"),
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	// Before 107 an undecided write comparison keeps its voice alive.
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-b'`, at); err == nil || !strings.Contains(err.Error(), "voice has publishable work") {
		t.Fatalf("the pre-107 guard let a voice with an undecided comparison go: %v", err)
	}

	if _, err := provider.UpTo(ctx, 107); err != nil {
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
	if n := count(`SELECT count(*) FROM model_experiments WHERE stage='analyze'`); n != 0 {
		t.Fatalf("%d analyze comparisons survived 107", n)
	}
	if n := count(`SELECT count(*) FROM model_experiment_candidates WHERE experiment_id LIKE 'analyze-%'`); n != 0 {
		t.Fatalf("%d analyze candidates survived 107", n)
	}
	if n := count(`SELECT count(*) FROM model_experiment_badges`); n != 0 {
		t.Fatalf("%d analyze badges survived 107", n)
	}
	if n := count(`SELECT count(*) FROM model_experiment_candidates WHERE experiment_id='write-review'`); n != 2 {
		t.Fatalf("the write comparison lost candidates: %d", n)
	}
	var status, reason string
	if err := handle.Reader.QueryRow(`SELECT status, error_reason FROM generation_jobs WHERE id='analyze-job'`).Scan(&status, &reason); err != nil || status != "failed" || reason != "JOB_INTERRUPTED" {
		t.Fatalf("the analyze comparison's job = %s %s %v", status, reason, err)
	}
	for stage, want := range map[string][]string{"analyze": {"active"}, "observe": {"active", "candidate_a", "candidate_b"}, "write": {"candidate_a", "candidate_b"}} {
		rows, err := handle.Reader.Query(`SELECT slot FROM model_selections WHERE stage=? ORDER BY slot`, stage)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var slot string
			if err := rows.Scan(&slot); err != nil {
				t.Fatal(err)
			}
			got = append(got, slot)
		}
		rows.Close()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s selections after 107 = %v, want %v", stage, got, want)
		}
	}
	var trigger string
	if err := handle.Reader.QueryRow(`SELECT sql FROM sqlite_master WHERE name='voices_refuse_publishable_work_on_delete'`).Scan(&trigger); err != nil || strings.Contains(trigger, "model_experiments") {
		t.Fatalf("the delete guard still reads experiments: %q %v", trigger, err)
	}
	// voice-c's running analysis still refuses its deletion; voice-b, named by an undecided
	// write comparison, deletes.
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-c'`, at); err == nil || !strings.Contains(err.Error(), "voice has publishable work") {
		t.Fatalf("a voice with a running job was deleted: %v", err)
	}
	if err := exec(`UPDATE voices SET deleted_at=? WHERE id='voice-b'`, at); err != nil {
		t.Fatalf("a voice named by a write comparison was refused: %v", err)
	}
}
