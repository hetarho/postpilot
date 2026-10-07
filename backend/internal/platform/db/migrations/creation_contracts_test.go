package migrations_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/pressly/goose/v3"
)

func TestCreationSchemaUpgradePreservesAcceptedUnknownSourcesAndPaidTables(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, os.DirFS("."), goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 147); err != nil {
		t.Fatal(err)
	}
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := handle.Writer.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"alice", "bob"} {
		exec("INSERT INTO users(id,password_hash,created_at) VALUES(?,'hash','now')", u)
		exec("INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES(?,?,?,0,'now','now')", "voice-"+u, u, u)
		exec("INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES(?,?,'now','now')", "post-"+u, u)
	}
	exec("INSERT INTO voice_samples(id,user_id,voice_id,kind,label,body,created_at) VALUES('material','alice','voice-alice','post','Label','changed live prose','now')")
	exec("INSERT INTO voice_analyses(voice_id,user_id,slot,snapshot,material_ids,analyze_model,created_at) VALUES('voice-alice','alice','current','{}','[\"material\"]','registry/model','now')")
	exec("INSERT INTO configuration_authoring_sessions(id,user_id,kind,request_id,revision,phase,snapshot,created_at,updated_at) VALUES('session','alice','post_template','create',1,'selected','{\"Selected\":{\"Name\":\"Draft\",\"Body\":\"unfinished\"}}','now','now')")
	exec("INSERT INTO voice_checks(id,user_id,voice_id,prompt_key,material_id,analysis_created_at,projection,write_model,status,piece,created_at,updated_at) VALUES('paid-check','alice','voice-alice','prompt','material','now','frozen-check','registry/model','done','paid output','now','now')")
	exec("INSERT INTO model_experiments(id,user_id,post_slug,stage,status,input_snapshot,input_hash,prompt_version,created_at) VALUES('paid-experiment','alice','post-alice','write','decided','paid snapshot','hash','v1','now')")

	if err = db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	var known int
	var accepted, snapshot string
	if err = handle.Reader.QueryRow("SELECT source_versions_known,accepted_sources,accepted_material_snapshot FROM voice_analyses").Scan(&known, &accepted, &snapshot); err != nil {
		t.Fatal(err)
	}
	if known != 0 || accepted != "[]" || snapshot != "[]" {
		t.Fatal("historical source freshness was fabricated")
	}
	var revision int
	if err = handle.Reader.QueryRow("SELECT content_revision FROM voice_samples").Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("sample revision: %d %v", revision, err)
	}
	var baseline *string
	var source string
	if err = handle.Reader.QueryRow("SELECT saved_baseline,working_source FROM configuration_authoring_sessions").Scan(&baseline, &source); err != nil || baseline != nil || !strings.Contains(source, "unfinished") {
		t.Fatalf("draft upgrade: %v %q %v", baseline, source, err)
	}
	for _, table := range []string{"model_experiments", "voice_checks"} {
		var name string
		if err = handle.Reader.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Fatalf("paid table %s lost: %v", table, err)
		}
	}
	var piece, paidSnapshot string
	if err = handle.Reader.QueryRow("SELECT piece FROM voice_checks WHERE id='paid-check'").Scan(&piece); err != nil || piece != "paid output" {
		t.Fatalf("paid check lost: %q %v", piece, err)
	}
	if err = handle.Reader.QueryRow("SELECT input_snapshot FROM model_experiments WHERE id='paid-experiment'").Scan(&paidSnapshot); err != nil || paidSnapshot != "paid snapshot" {
		t.Fatalf("paid experiment lost: %q %v", paidSnapshot, err)
	}

	admit := "INSERT INTO writing_tests(id,user_id,operation_key,fingerprint,kind,factor,model_stage,count,status,source_post_slug,context,common_hash,prompt_version,created_at,updated_at) VALUES(?,?,?,'fp','ab','model','write',2,'queued',?,'{}','hash','v1','now','now')"
	exec(admit, "test-a", "alice", "request-a", "post-alice")
	if _, err = handle.Writer.Exec(admit, "foreign", "bob", "request-b", "post-alice"); err == nil {
		t.Fatal("cross-owner source accepted")
	}
	if _, err = handle.Writer.Exec(admit, "duplicate", "alice", "request-a", "post-alice"); err == nil {
		t.Fatal("duplicate admission accepted")
	}
	candidate := "INSERT INTO writing_test_candidates(id,user_id,test_id,seed_position,source_kind,source_id,source_revision,semantic_key,status) VALUES(?,?,?,?,'model','registered/model','v1',?,'succeeded')"
	exec(candidate, "left", "alice", "test-a", 0, "left")
	exec(candidate, "right", "alice", "test-a", 1, "right")
	if _, err = handle.Writer.Exec(candidate, "foreign", "bob", "test-a", 1, "foreign"); err == nil {
		t.Fatal("cross-owner candidate accepted")
	}
	exec("INSERT INTO writing_test_matches(id,user_id,test_id,round,match_index,left_candidate_id,right_candidate_id) VALUES('match','alice','test-a',1,0,'left','right')")
	if _, err = handle.Writer.Exec("UPDATE writing_test_matches SET winner_candidate_id='other',decision_key='decision',decided_at='now' WHERE id='match'"); err == nil {
		t.Fatal("third-party match winner accepted")
	}
	if _, err = handle.Writer.Exec("INSERT INTO writing_test_matches(id,user_id,test_id,round,match_index,left_candidate_id,right_candidate_id) VALUES('duplicate-match','alice','test-a',1,0,'left','right')"); err == nil {
		t.Fatal("duplicate bracket slot accepted")
	}
	if _, err = handle.Writer.Exec("DELETE FROM posts WHERE slug='post-alice'"); err == nil {
		t.Fatal("source deletion admitted before payload purge")
	}
	exec("UPDATE writing_tests SET purge_fence=1,common_snapshot=NULL WHERE id='test-a'")
	exec("DELETE FROM posts WHERE slug='post-alice'")
	var slug *string
	if err = handle.Reader.QueryRow("SELECT source_post_slug FROM writing_tests WHERE id='test-a'").Scan(&slug); err != nil || slug != nil {
		t.Fatalf("source detach: %v %v", slug, err)
	}
	var checks int
	if err = handle.Reader.QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&checks); err != nil || checks != 0 {
		t.Fatalf("foreign keys: %d %v", checks, err)
	}
	if _, err = provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = handle.Reader.QueryRow("SELECT COUNT(*) FROM writing_test_candidates").Scan(&n); err != nil || n != 2 {
		t.Fatalf("rollback destroyed history: %d %v", n, err)
	}
}
