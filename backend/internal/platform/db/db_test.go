package db

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

func migrationsThrough(t *testing.T, names ...string) fstest.MapFS {
	t.Helper()
	selected := fstest.MapFS{}
	for _, name := range names {
		data, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		selected[name] = &fstest.MapFile{Data: data}
	}
	return selected
}

func openTemp(t *testing.T) *DB {
	t.Helper()
	// A nested directory proves Open creates the parent — on a fresh volume the
	// mount point exists but the db's directory does not.
	handle, err := Open(filepath.Join(t.TempDir(), "nested", "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	return handle
}

// TestOpenAppliesPragmas is job 01 A11: WAL and exactly one write connection.
func TestOpenAppliesPragmas(t *testing.T) {
	handle := openTemp(t)

	var journalMode string
	if err := handle.Writer.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	// The pragmas ride the DSN, so they must hold on the reader pool too.
	var foreignKeys int
	if err := handle.Reader.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Error("foreign_keys is off on the reader — a cascade delete would silently not cascade")
	}

	if got := handle.Writer.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("writer MaxOpenConnections = %d, want 1 (the single serialized writer)", got)
	}
}

func TestMigrateAppliesSchemaAndIsIdempotent(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()

	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Every boot runs Migrate, so a second call must be a no-op, not a failure.
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("Migrate (second run): %v", err)
	}

	for _, table := range []string{"users", "sessions", "model_experiments", "model_experiment_candidates"} {
		var name string
		err := handle.Reader.QueryRow(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing after migration: %v", table, err)
		}
	}
}

func TestMigration0012BackfillsLanguagesAndFailuresWithoutLosingLegacyRows(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	throughEleven := migrationsThrough(t,
		"0001_users_sessions.sql", "0002_posts_images_uploads.sql", "0003_model_selections.sql",
		"0004_generation_jobs.sql", "0005_voice_profiles_samples.sql", "0006_model_experiments.sql",
		"0007_voice_personalization.sql", "0008_generation_modes_and_finalization.sql",
		"0009_independent_voice_profiles.sql", "0010_publishing.sql", "0011_post_purposes.sql",
	)
	if err := migrate(ctx, handle.Writer, throughEleven); err != nil {
		t.Fatal(err)
	}
	const at = "2026-08-31T00:00:00Z"
	content := `{"title":"원문","blocks":[{"type":"TEXT","content":"바이트 보존"}]}`
	baseline := `{"title":"기계","blocks":[{"type":"TEXT","content":"기준선"}]}`
	observations := `[{"file":"one.jpg","visible_text":"원문 그대로"}]`
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','기본 말투',1,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,memo,observations,content,status,created_at,updated_at,content_revision,machine_baseline,machine_baseline_revision,machine_baseline_voice_id,finalized_revision,finalized_at) VALUES('content','alice','voice','제목','메모',? ,?,'finalized','` + at + `','` + at + `',7,?,5,'voice',7,'` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at) VALUES('empty','alice','voice','초안','','draft','` + at + `','` + at + `')`,
	} {
		var err error
		if strings.Contains(statement, "VALUES('content'") {
			_, err = handle.Writer.ExecContext(ctx, statement, observations, content, baseline)
		} else {
			_, err = handle.Writer.ExecContext(ctx, statement)
		}
		if err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}

	generatePayload := `{"topic":"제주","nested":{"keep":true}}`
	revisePayload := `{"content":{"title":"기존"}}`
	opaquePayload := "not-json\x00keep-exact"
	for _, row := range []struct {
		id, kind, payload, legacyError string
	}{
		{id: "generate", kind: "generate", payload: generatePayload, legacyError: "provider raw generate"},
		{id: "revise", kind: "revise", payload: revisePayload},
		{id: "opaque", kind: "generate", payload: opaquePayload},
		{id: "observe", kind: "observe", payload: `{"keep":"independent"}`},
	} {
		if _, err := handle.Writer.ExecContext(ctx,
			`INSERT INTO generation_jobs(id,user_id,kind,status,error,payload,created_at,updated_at) VALUES(?,?,?,'done',?,?,?,?)`,
			row.id, "alice", row.kind, nullIfEmpty(row.legacyError), row.payload, at, at,
		); err != nil {
			t.Fatalf("seed job %s: %v", row.id, err)
		}
	}

	writeSnapshot := `{"post":{"title":"A"},"nested":{"keep":1}}`
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO model_experiments(id,user_id,post_slug,voice_id,purpose_name,stage,status,input_snapshot,input_hash,prompt_version,apply_error,adoption_error,created_at) VALUES('write','alice','content','voice','리뷰','write','dismissed',?,'hash','v1','apply raw','adopt raw',?)`,
		writeSnapshot, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO model_experiments(id,user_id,voice_id,purpose_name,stage,status,input_snapshot,input_hash,prompt_version,created_at) VALUES('observe','alice',NULL,'','observe','dismissed','not-json','observe-hash','v1',?)`, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO model_experiment_candidates(id,experiment_id,model_provider_id,model_id,model_label,display_side,status,error) VALUES('candidate','write','p','m','Model','left','failed','candidate raw')`,
	); err != nil {
		t.Fatal(err)
	}

	// Seed all three durable voice failure paths under the pre-0012 schema. Their
	// prose and aggregate language provenance must survive the additive migration.
	for _, statement := range []string{
		`INSERT INTO voice_learning_events(id,user_id,voice_id,post_slug,baseline_revision,input_hash,baseline_content,final_content,model_ref,status,error,created_at) VALUES('voice-event','alice','voice','content',1,'voice-hash','{}','{}','p/m','failed','learning raw','` + at + `')`,
		`INSERT INTO voice_authored_sources(id,user_id,voice_id,post_slug,learning_event_id,title,tags,body,excerpt,created_at) VALUES('voice-source','alice','voice','content','voice-event','원문','[]','본문','본문','` + at + `')`,
		`INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('voice-rule','alice','voice','규칙','voice-rule','syntax',1,'candidate','manual','` + at + `','` + at + `')`,
		`INSERT INTO voice_rule_comparisons(id,user_id,voice_id,rule_id,source_id,profile_version,model_ref,target_length,input_snapshot,rule_on_side,status,created_at) VALUES('voice-comparison','alice','voice','voice-rule','voice-source',1,'p/m',1200,'{}','left','failed','` + at + `')`,
		`INSERT INTO voice_rule_comparison_candidates(id,comparison_id,display_side,status,error) VALUES('voice-candidate','voice-comparison','left','failed','comparison raw')`,
		`INSERT INTO voice_profile_validations(id,user_id,voice_id,profile_version,analyze_model_ref,write_model_ref,judge_enabled,status,created_at) VALUES('voice-validation','alice','voice',1,'p/a','p/w',0,'failed','` + at + `')`,
		`INSERT INTO voice_profile_validation_items(id,validation_id,source_id,voice_id,user_id,position,status,error) VALUES('voice-item','voice-validation','voice-source','voice','alice',0,'failed','validation raw')`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed voice migration row %q: %v", statement, err)
		}
	}

	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,created_at,updated_at) VALUES('agent','alice','token','Mac','naver_blog',?,?)`, at, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('publish','alice',?)`, at); err != nil {
		t.Fatal(err)
	}
	manifest := `{"job_id":"publish","content":{"title":"보존"}}`
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,manifest_json,settings_json,error_code,error_message,created_at,updated_at) VALUES('publish','alice','content',?,'agent','naver_blog','failed','opening_editor',7,?,'{}','legacy_code','publish raw',?,?)`,
		at, manifest, at, at,
	); err != nil {
		t.Fatal(err)
	}

	beforeCounts := map[string]int{}
	for _, table := range []string{"voices", "posts", "generation_jobs", "model_experiments", "model_experiment_candidates", "voice_learning_events", "voice_rule_comparisons", "voice_rule_comparison_candidates", "voice_profile_validations", "voice_profile_validation_items", "publish_jobs"} {
		var count int
		if err := handle.Reader.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		beforeCounts[table] = count
	}
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	// Boot runs migration discovery every time. The already-applied version must be a no-op.
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("second boot migrate: %v", err)
	}

	for _, table := range []string{"voices", "posts", "generation_jobs", "model_experiments", "model_experiment_candidates", "voice_learning_events", "voice_rule_comparisons", "voice_rule_comparison_candidates", "voice_profile_validations", "voice_profile_validation_items", "publish_jobs"} {
		var count int
		if err := handle.Reader.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil || count != beforeCounts[table] {
			t.Fatalf("%s row count = %d, want %d err=%v", table, count, beforeCounts[table], err)
		}
	}
	var gotObservations, gotContent, gotBaseline, target, provenance string
	if err := handle.Reader.QueryRowContext(ctx,
		`SELECT observations,content,machine_baseline,target_language,content_language FROM posts WHERE slug='content'`,
	).Scan(&gotObservations, &gotContent, &gotBaseline, &target, &provenance); err != nil {
		t.Fatal(err)
	}
	if gotObservations != observations || gotContent != content || gotBaseline != baseline || target != "ko" || provenance != "ko" {
		t.Fatalf("content post changed: observations=%q content=%q baseline=%q languages=%s/%s", gotObservations, gotContent, gotBaseline, target, provenance)
	}
	var emptyProvenance sql.NullString
	if err := handle.Reader.QueryRowContext(ctx, `SELECT content_language FROM posts WHERE slug='empty'`).Scan(&emptyProvenance); err != nil || emptyProvenance.Valid {
		t.Fatalf("contentless provenance = %+v err=%v", emptyProvenance, err)
	}
	var source string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT source_language FROM voices WHERE id='voice'`).Scan(&source); err != nil || source != "ko" {
		t.Fatalf("voice source = %q err=%v", source, err)
	}

	var generated, revised, opaque, observe string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT payload FROM generation_jobs WHERE id='generate'`).Scan(&generated); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT payload FROM generation_jobs WHERE id='revise'`).Scan(&revised); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT payload FROM generation_jobs WHERE id='opaque'`).Scan(&opaque); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT payload FROM generation_jobs WHERE id='observe'`).Scan(&observe); err != nil {
		t.Fatal(err)
	}
	if opaque != opaquePayload || observe != `{"keep":"independent"}` {
		t.Fatalf("opaque/observe payload changed: %q / %q", opaque, observe)
	}
	var generatedTarget, generatedContent, revisedTarget, revisedContent sql.NullString
	if err := handle.Reader.QueryRowContext(ctx,
		`SELECT json_extract(?,'$.target_language'),json_extract(?,'$.content_language'),json_extract(?,'$.target_language'),json_extract(?,'$.content_language')`,
		generated, generated, revised, revised,
	).Scan(&generatedTarget, &generatedContent, &revisedTarget, &revisedContent); err != nil {
		t.Fatal(err)
	}
	if generatedTarget.String != "ko" || generatedContent.Valid || revisedTarget.Valid || revisedContent.String != "ko" {
		t.Fatalf("payload classification generate=%+v/%+v revise=%+v/%+v", generatedTarget, generatedContent, revisedTarget, revisedContent)
	}
	var experimentTarget, migratedSnapshot string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT target_language,input_snapshot FROM model_experiments WHERE id='write'`).Scan(&experimentTarget, &migratedSnapshot); err != nil {
		t.Fatal(err)
	}
	var snapshotTarget string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT json_extract(?,'$.target_language')`, migratedSnapshot).Scan(&snapshotTarget); err != nil || experimentTarget != "ko" || snapshotTarget != "ko" {
		t.Fatalf("write experiment target=%q snapshot=%q err=%v", experimentTarget, snapshotTarget, err)
	}
	var observeTarget sql.NullString
	var observeSnapshot string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT target_language,input_snapshot FROM model_experiments WHERE id='observe'`).Scan(&observeTarget, &observeSnapshot); err != nil || observeTarget.Valid || observeSnapshot != "not-json" {
		t.Fatalf("observe experiment = target %+v snapshot %q err=%v", observeTarget, observeSnapshot, err)
	}

	for _, check := range []struct {
		query      string
		args       []any
		wantDetail string
	}{
		{query: `SELECT error_reason,error_params,technical_detail FROM generation_jobs WHERE id='generate'`, wantDetail: "provider raw generate"},
		{query: `SELECT error_reason,error_params,technical_detail FROM model_experiment_candidates WHERE id='candidate'`, wantDetail: "candidate raw"},
		{query: `SELECT apply_error_reason,apply_error_params,apply_technical_detail FROM model_experiments WHERE id='write'`, wantDetail: "apply raw"},
		{query: `SELECT adoption_error_reason,adoption_error_params,adoption_technical_detail FROM model_experiments WHERE id='write'`, wantDetail: "adopt raw"},
		{query: `SELECT error_reason,error_params,technical_detail FROM voice_learning_events WHERE id='voice-event'`, wantDetail: "learning raw"},
		{query: `SELECT error_reason,error_params,technical_detail FROM voice_rule_comparison_candidates WHERE id='voice-candidate'`, wantDetail: "comparison raw"},
		{query: `SELECT error_reason,error_params,technical_detail FROM voice_profile_validation_items WHERE id='voice-item'`, wantDetail: "validation raw"},
		{query: `SELECT error_reason,error_params,technical_detail FROM publish_jobs WHERE id='publish'`, wantDetail: "publish raw"},
	} {
		var reason, params, detail string
		if err := handle.Reader.QueryRowContext(ctx, check.query, check.args...).Scan(&reason, &params, &detail); err != nil || reason != "UNKNOWN_FAILURE" || params != "{}" || detail != check.wantDetail {
			t.Fatalf("failure backfill for %q = %q %q %q err=%v", check.query, reason, params, detail, err)
		}
	}
	var eventContent, eventSource, comparisonSource, validationSource string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT content_language,source_language FROM voice_learning_events WHERE id='voice-event'`).Scan(&eventContent, &eventSource); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT source_language FROM voice_rule_comparisons WHERE id='voice-comparison'`).Scan(&comparisonSource); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT source_language FROM voice_profile_validations WHERE id='voice-validation'`).Scan(&validationSource); err != nil {
		t.Fatal(err)
	}
	if eventContent != "ko" || eventSource != "ko" || comparisonSource != "ko" || validationSource != "ko" {
		t.Fatalf("voice language backfill event=%s/%s comparison=%s validation=%s", eventContent, eventSource, comparisonSource, validationSource)
	}
	var publishTarget, publishContent, publishSource, gotManifest string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT target_language,content_language,voice_source_language,manifest_json FROM publish_jobs WHERE id='publish'`).Scan(&publishTarget, &publishContent, &publishSource, &gotManifest); err != nil || publishTarget != "ko" || publishContent != "ko" || publishSource != "ko" || gotManifest != manifest {
		t.Fatalf("publish freeze = %s/%s/%s manifest=%q err=%v", publishTarget, publishContent, publishSource, gotManifest, err)
	}

	for name, statement := range map[string]string{
		"voice language":      `UPDATE voices SET source_language='fr' WHERE id='voice'`,
		"post language":       `UPDATE posts SET target_language='fr' WHERE slug='content'`,
		"array params":        `UPDATE generation_jobs SET error_params='[]' WHERE id='generate'`,
		"scalar params":       `UPDATE generation_jobs SET error_params='1' WHERE id='generate'`,
		"malformed params":    `UPDATE generation_jobs SET error_params='{bad' WHERE id='generate'`,
		"experiment language": `UPDATE model_experiments SET target_language='fr' WHERE id='write'`,
		"learning content":    `UPDATE voice_learning_events SET content_language='fr' WHERE id='voice-event'`,
		"learning source":     `UPDATE voice_learning_events SET source_language='fr' WHERE id='voice-event'`,
		"comparison language": `UPDATE voice_rule_comparisons SET source_language='fr' WHERE id='voice-comparison'`,
		"validation language": `UPDATE voice_profile_validations SET source_language='fr' WHERE id='voice-validation'`,
		"voice array params":  `UPDATE voice_rule_comparison_candidates SET error_params='[]' WHERE id='voice-candidate'`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := handle.Writer.ExecContext(ctx, statement); err == nil {
				t.Fatalf("constraint accepted %s", statement)
			}
		})
	}
	if _, err := handle.Writer.ExecContext(ctx, `UPDATE generation_jobs SET error_params='{"attempt":2}' WHERE id='generate'`); err != nil {
		t.Fatalf("valid object params rejected: %v", err)
	}
	rows, err := handle.Reader.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("migration left a foreign-key violation")
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 11); err != nil {
		t.Fatal(err)
	}
	for table, columns := range map[string][]string{
		"voices":                           {"source_language"},
		"posts":                            {"target_language", "content_language"},
		"generation_jobs":                  {"target_language", "error_reason", "error_params", "technical_detail"},
		"model_experiments":                {"target_language", "apply_error_reason", "adoption_error_reason"},
		"voice_learning_events":            {"content_language", "source_language", "error_reason", "error_params", "technical_detail"},
		"voice_rule_comparisons":           {"source_language"},
		"voice_rule_comparison_candidates": {"error_reason", "error_params", "technical_detail"},
		"voice_profile_validations":        {"source_language"},
		"voice_profile_validation_items":   {"error_reason", "error_params", "technical_detail"},
		"publish_jobs":                     {"target_language", "content_language", "voice_source_language", "error_reason"},
	} {
		for _, column := range columns {
			var count int
			if err := handle.Reader.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_info(?) WHERE name=?`, table, column).Scan(&count); err != nil || count != 0 {
				t.Fatalf("down retained %s.%s: count=%d err=%v", table, column, count, err)
			}
		}
	}
	var downContent, downObservations string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT content,observations FROM posts WHERE slug='content'`).Scan(&downContent, &downObservations); err != nil || downContent != content || downObservations != observations {
		t.Fatalf("down lost post bytes: content=%q observations=%q err=%v", downContent, downObservations, err)
	}
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func TestMigration0006UpgradesActiveSelectionsAndRollsBack(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	firstThree := fstest.MapFS{}
	for _, name := range []string{"0001_users_sessions.sql", "0002_posts_images_uploads.sql", "0003_model_selections.sql"} {
		data, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		firstThree[name] = &fstest.MapFile{Data: data}
	}
	if err := migrate(ctx, handle.Writer, firstThree); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO model_selections(user_id,stage,provider_id,model_id,updated_at) VALUES('alice','write','p','m','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	var slot, model string
	if err := handle.Reader.QueryRow(`SELECT slot,model_id FROM model_selections WHERE user_id='alice' AND stage='write'`).Scan(&slot, &model); err != nil {
		t.Fatal(err)
	}
	if slot != "active" || model != "m" {
		t.Fatalf("upgraded selection = %s/%s", slot, model)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	// Every migration above 0006 comes off, then 0006's own Down runs — named by target
	// version rather than by a count of Down calls, so adding a migration cannot silently
	// leave this test asserting against the wrong schema.
	if _, err := provider.DownTo(ctx, 5); err != nil {
		t.Fatal(err)
	}
	var columns int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('model_selections') WHERE name='slot'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("migration down retained slot column")
	}
	if err := handle.Reader.QueryRow(`SELECT model_id FROM model_selections WHERE user_id='alice' AND stage='write'`).Scan(&model); err != nil || model != "m" {
		t.Fatalf("down lost active selection: model=%q err=%v", model, err)
	}
}

func TestMigration0007PreservesLegacyVoiceAndRollsBack(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	throughSix := fstest.MapFS{}
	for _, name := range []string{"0001_users_sessions.sql", "0002_posts_images_uploads.sql", "0003_model_selections.sql", "0004_generation_jobs.sql", "0005_voice_profiles_samples.sql", "0006_model_experiments.sql"} {
		data, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		throughSix[name] = &fstest.MapFile{Data: data}
	}
	if err := migrate(ctx, handle.Writer, throughSix); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	style, rules := "  legacy style\n그대로  ", "rule A\n\nrule B"
	if _, err := handle.Writer.Exec(`INSERT INTO voice_profiles(user_id,styleguide,rules,updated_at) VALUES('alice',?,?,?)`, style, rules, "2026-08-29T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES('p','alice','t','','review','2026-08-29T00:00:00Z','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	var gotStyle, gotRules string
	if err := handle.Reader.QueryRow(`SELECT styleguide,rules FROM voice_profiles WHERE user_id='alice'`).Scan(&gotStyle, &gotRules); err != nil || gotStyle != style || gotRules != rules {
		t.Fatalf("legacy guidance changed: %q / %q err=%v", gotStyle, gotRules, err)
	}
	for _, table := range []string{"voice_profile_versions", "voice_learning_events", "voice_contrast_rules", "voice_profile_validations"} {
		var count int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s missing: count=%d err=%v", table, count, err)
		}
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.DownTo(ctx, 6); err != nil {
		t.Fatal(err)
	}
	var currentVersionColumns int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('voice_profiles') WHERE name='current_version'`).Scan(&currentVersionColumns); err != nil || currentVersionColumns != 0 {
		t.Fatalf("down retained current_version: %d err=%v", currentVersionColumns, err)
	}
	if err := handle.Reader.QueryRow(`SELECT styleguide,rules FROM voice_profiles WHERE user_id='alice'`).Scan(&gotStyle, &gotRules); err != nil || gotStyle != style || gotRules != rules {
		t.Fatalf("down lost legacy guidance: %q / %q err=%v", gotStyle, gotRules, err)
	}
}

func TestMigration0008PreservesTargetsAndAddsFinalizationProgress(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	throughSeven := fstest.MapFS{}
	for _, name := range []string{
		"0001_users_sessions.sql", "0002_posts_images_uploads.sql", "0003_model_selections.sql",
		"0004_generation_jobs.sql", "0005_voice_profiles_samples.sql", "0006_model_experiments.sql",
		"0007_voice_personalization.sql",
	} {
		data, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		throughSeven[name] = &fstest.MapFile{Data: data}
	}
	if err := migrate(ctx, handle.Writer, throughSeven); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		slug, status string
		target       int
	}{{"legacy-default", "draft", 1200}, {"legacy-custom", "review", 1777}} {
		if _, err := handle.Writer.Exec(`INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at,target_length) VALUES(?,'alice','','',?,'2026-08-29T00:00:00Z','2026-08-29T00:00:00Z',?)`, row.slug, row.status, row.target); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		slug, status string
		target       int
	}{{"legacy-default", "draft", 1200}, {"legacy-custom", "review", 1777}} {
		var status string
		var target int
		if err := handle.Reader.QueryRow(`SELECT status,target_length FROM posts WHERE slug=?`, row.slug).Scan(&status, &target); err != nil || status != row.status || target != row.target {
			t.Fatalf("preserved %s = status %q target %d err=%v", row.slug, status, target, err)
		}
	}
	// Past 0009 a post names its voice, so the insert reads the account's migrated default.
	if _, err := handle.Writer.Exec(`INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at) VALUES('new','alice',(SELECT id FROM voices WHERE user_id='alice'),'','','draft','2026-08-29T00:00:00Z','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	var nullTarget int
	if err := handle.Reader.QueryRow(`SELECT target_length IS NULL FROM posts WHERE slug='new'`).Scan(&nullTarget); err != nil || nullTarget != 1 {
		t.Fatalf("new target null=%d err=%v", nullTarget, err)
	}
	for table, columns := range map[string][]string{
		"posts":             {"finalized_revision", "finalized_at"},
		"model_experiments": {"adoption_error", "adopted_at"},
	} {
		for _, column := range columns {
			var count int
			if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name=?`, table, column).Scan(&count); err != nil || count != 1 {
				t.Fatalf("%s.%s missing: count=%d err=%v", table, column, count, err)
			}
		}
	}
	var foreignKeys int
	if err := handle.Reader.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign keys=%d err=%v", foreignKeys, err)
	}
	var indexCount int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name='idx_posts_user_updated'`).Scan(&indexCount); err != nil || indexCount != 1 {
		t.Fatalf("post index count=%d err=%v", indexCount, err)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 7); err != nil {
		t.Fatal(err)
	}
	var target int
	if err := handle.Reader.QueryRow(`SELECT target_length FROM posts WHERE slug='new'`).Scan(&target); err != nil || target != 1200 {
		t.Fatalf("down target=%d err=%v", target, err)
	}
	var finalizedColumns int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('posts') WHERE name='finalized_revision'`).Scan(&finalizedColumns); err != nil || finalizedColumns != 0 {
		t.Fatalf("down retained finalized column: %d err=%v", finalizedColumns, err)
	}
}

// TestMigrateFailurePropagates is plan 01 AC8's mechanism: a broken migration must
// return an error so cmd/api can exit non-zero before the listener starts, which is
// what lets the deploy's /health gate roll back ([I7]).
func TestMigrateFailurePropagates(t *testing.T) {
	handle := openTemp(t)

	broken := fstest.MapFS{
		"0001_broken.sql": &fstest.MapFile{
			Data: []byte("-- +goose Up\nCREATE TABLE ( this is not sql;\n"),
		},
	}

	if err := migrate(context.Background(), handle.Writer, broken); err == nil {
		t.Fatal("a broken migration reported success")
	}
}

func TestOpenRejectsUnusablePath(t *testing.T) {
	// A file where the directory should be: MkdirAll fails, and Open must say so at
	// boot rather than at the first query.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := Open(filepath.Join(blocker, "db.sqlite")); err == nil {
		t.Error("Open succeeded on a path whose parent is a regular file")
	}
}

// TestMigration0009PartitionsVoicesAndRollsBack is plan 10 A1: an existing account keeps
// exactly what it had, now owned by one default voice, and the rollback refuses as soon as a
// second voice exists — collapsing two voices would have to pick which history survives.
func TestMigration0009PartitionsVoicesAndRollsBack(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	throughEight := fstest.MapFS{}
	for _, name := range []string{
		"0001_users_sessions.sql", "0002_posts_images_uploads.sql", "0003_model_selections.sql",
		"0004_generation_jobs.sql", "0005_voice_profiles_samples.sql", "0006_model_experiments.sql",
		"0007_voice_personalization.sql", "0008_generation_modes_and_finalization.sql",
	} {
		data, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatal(err)
		}
		throughEight[name] = &fstest.MapFile{Data: data}
	}
	if err := migrate(ctx, handle.Writer, throughEight); err != nil {
		t.Fatal(err)
	}
	const at = "2026-08-29T00:00:00Z"
	style, rules, snapshot := "  legacy style\n그대로  ", "rule A\n\nrule B", `{"version":5,"lexical":{"description":"담백"}}`
	seed := []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `'),('bob','hash','` + at + `')`,
		`INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at,content_revision,machine_baseline,machine_baseline_revision) VALUES('p1','alice','제주','메모','review','` + at + `','` + at + `',3,'{"blocks":[]}',3)`,
		`INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES('p3','alice','부산','','draft','` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES('p2','bob','서울','','draft','` + at + `','` + at + `')`,
		`INSERT INTO voice_profiles(user_id,styleguide,rules,corpus_version,updated_at,current_version) VALUES('alice',?,?,2,'` + at + `',5)`,
		`INSERT INTO voice_samples(id,user_id,label,body,created_at) VALUES('s1','alice','샘플','본문','` + at + `')`,
		`INSERT INTO voice_profile_versions(id,user_id,version,snapshot,origin,created_at) VALUES('ver1','alice',5,?,'analysis','` + at + `')`,
		`INSERT INTO voice_manual_overrides(user_id,layer,field,value,updated_at) VALUES('alice','lexical','description','담백','` + at + `')`,
		`INSERT INTO voice_learning_events(id,user_id,post_slug,baseline_revision,input_hash,baseline_content,final_content,model_ref,status,created_at) VALUES('e1','alice','p1',3,'h','{}','{}','openrouter/x','done','` + at + `')`,
		`INSERT INTO voice_authored_sources(id,user_id,post_slug,learning_event_id,title,tags,body,excerpt,created_at) VALUES('src1','alice','p1','e1','제주','[]','본문','발췌','` + at + `')`,
		`INSERT INTO voice_contrast_rules(id,user_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('r1','alice','규칙','k1','endings',3,'active','diff','` + at + `','` + at + `')`,
		`INSERT INTO voice_rule_evidence(id,user_id,rule_id,event_id,origin,payload_ref,created_at) VALUES('ev1','alice','r1','e1','diff','p','` + at + `')`,
		`INSERT INTO voice_rule_confirmations(id,user_id,rule_id,proposed_statement,event_id,status,created_at) VALUES('conf1','alice','r1','바꾼 규칙','e1','pending','` + at + `')`,
		`INSERT INTO voice_rule_comparisons(id,user_id,rule_id,source_id,profile_version,model_ref,target_length,input_snapshot,rule_on_side,status,created_at) VALUES('cmp1','alice','r1','src1',5,'openrouter/x',1200,'{}','left','decided','` + at + `')`,
		`INSERT INTO voice_rule_comparison_candidates(id,comparison_id,display_side,status) VALUES('cmpc1','cmp1','left','succeeded')`,
		`INSERT INTO voice_sentence_feedback(id,user_id,post_slug,sentence_ref,kind,payload_ref,processing_state,created_at) VALUES('f1','alice','p1','s','thumbs','p','pending','` + at + `')`,
		`INSERT INTO voice_profile_validations(id,user_id,profile_version,analyze_model_ref,write_model_ref,judge_enabled,status,created_at) VALUES('val1','alice',5,'a','w',0,'done','` + at + `')`,
		`INSERT INTO voice_profile_validation_items(id,validation_id,source_id,position,status) VALUES('vali1','val1','src1',0,'scored')`,
		`INSERT INTO generation_jobs(id,post_slug,user_id,kind,status,created_at,updated_at) VALUES('j1',NULL,'alice','analyze_voice','done','` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,post_slug,user_id,kind,status,created_at,updated_at) VALUES('j2','p1','alice','generate','done','` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,post_slug,user_id,kind,status,created_at,updated_at) VALUES('j3','p1','alice','learn_voice','queued','` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,post_slug,user_id,kind,status,created_at,updated_at,started_at) VALUES('j4','p3','alice','learn_voice','running','` + at + `','` + at + `','` + at + `')`,
		`INSERT INTO model_experiments(id,user_id,stage,status,input_hash,prompt_version,created_at) VALUES('x1','alice','analyze','decided','h','v1','` + at + `')`,
		`INSERT INTO model_experiments(id,user_id,post_slug,stage,status,input_hash,prompt_version,created_at) VALUES('x2','alice','p1','write','decided','h2','v1','` + at + `')`,
	}
	for _, statement := range seed {
		var err error
		switch {
		case strings.Contains(statement, "voice_profiles"):
			_, err = handle.Writer.Exec(statement, style, rules)
		case strings.Contains(statement, "voice_profile_versions"):
			_, err = handle.Writer.Exec(statement, snapshot)
		default:
			_, err = handle.Writer.Exec(statement)
		}
		if err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}

	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}

	// One active default voice per account, named the same as a new account's first voice.
	var voices, defaults int
	if err := handle.Reader.QueryRow(`SELECT count(*), sum(is_default) FROM voices WHERE name='기본 말투' AND deleted_at IS NULL`).Scan(&voices, &defaults); err != nil || voices != 2 || defaults != 2 {
		t.Fatalf("voices=%d defaults=%d err=%v", voices, defaults, err)
	}
	var aliceVoice string
	if err := handle.Reader.QueryRow(`SELECT id FROM voices WHERE user_id='alice'`).Scan(&aliceVoice); err != nil {
		t.Fatal(err)
	}

	// Every migrated row belongs to its account's voice, and nothing crossed accounts.
	for _, table := range []string{
		"voice_profiles", "voice_samples", "voice_profile_versions", "voice_manual_overrides",
		"voice_learning_events", "voice_authored_sources", "voice_contrast_rules",
		"voice_rule_evidence", "voice_rule_confirmations", "voice_rule_comparisons",
		"voice_sentence_feedback", "voice_profile_validations", "voice_profile_validation_items",
	} {
		var mismatched int
		query := `SELECT count(*) FROM ` + table + ` t JOIN voices v ON v.id = t.voice_id WHERE v.user_id <> t.user_id`
		if err := handle.Reader.QueryRow(query).Scan(&mismatched); err != nil || mismatched != 0 {
			t.Fatalf("%s crossed accounts: %d err=%v", table, mismatched, err)
		}
		var unassigned int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM ` + table + ` WHERE voice_id IS NULL`).Scan(&unassigned); err != nil || unassigned != 0 {
			t.Fatalf("%s left rows unassigned: %d err=%v", table, unassigned, err)
		}
	}

	// The legacy guidance, the snapshot and the revisions are byte-identical.
	var gotStyle, gotRules, gotSnapshot string
	var corpus, current int
	if err := handle.Reader.QueryRow(`SELECT styleguide,rules,corpus_version,current_version FROM voice_profiles WHERE voice_id=?`, aliceVoice).Scan(&gotStyle, &gotRules, &corpus, &current); err != nil {
		t.Fatal(err)
	}
	if gotStyle != style || gotRules != rules || corpus != 2 || current != 5 {
		t.Fatalf("profile changed: %q %q %d %d", gotStyle, gotRules, corpus, current)
	}
	if err := handle.Reader.QueryRow(`SELECT snapshot FROM voice_profile_versions WHERE id='ver1'`).Scan(&gotSnapshot); err != nil || gotSnapshot != snapshot {
		t.Fatalf("snapshot changed: %q err=%v", gotSnapshot, err)
	}

	// An account that never had a profile still gets exactly one empty one — a read must
	// never be the thing that creates domain rows.
	var bobProfiles int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM voice_profiles p JOIN voices v ON v.id=p.voice_id WHERE v.user_id='bob' AND p.styleguide='' AND p.current_version=0`).Scan(&bobProfiles); err != nil || bobProfiles != 1 {
		t.Fatalf("bob profiles=%d err=%v", bobProfiles, err)
	}

	// A post that already had a machine baseline keeps its learning eligibility, because the
	// baseline is recorded as having been written under the voice it is now assigned to.
	var postVoice, baselineVoice string
	if err := handle.Reader.QueryRow(`SELECT voice_id, COALESCE(machine_baseline_voice_id,'') FROM posts WHERE slug='p1'`).Scan(&postVoice, &baselineVoice); err != nil {
		t.Fatal(err)
	}
	if postVoice != aliceVoice || baselineVoice != aliceVoice {
		t.Fatalf("p1 voice=%q baseline voice=%q want %q", postVoice, baselineVoice, aliceVoice)
	}
	var neverGenerated int
	if err := handle.Reader.QueryRow(`SELECT machine_baseline_voice_id IS NULL FROM posts WHERE slug='p2'`).Scan(&neverGenerated); err != nil || neverGenerated != 1 {
		t.Fatalf("p2 baseline voice not null: %d err=%v", neverGenerated, err)
	}

	// Voice-owned and post-backed durable work both freeze the voice they can publish to.
	for _, id := range []string{"j1", "j2", "j3", "j4"} {
		var jobVoice string
		if err := handle.Reader.QueryRow(`SELECT COALESCE(voice_id,'') FROM generation_jobs WHERE id=?`, id).Scan(&jobVoice); err != nil || jobVoice != aliceVoice {
			t.Fatalf("job %s voice=%q err=%v", id, jobVoice, err)
		}
	}
	// The old post-only guard allowed simultaneous learning jobs for two posts. Their durable
	// states survive unchanged; the trigger below prevents any new third row for that voice.
	for id, want := range map[string]string{"j3": "queued", "j4": "running"} {
		var status string
		if err := handle.Reader.QueryRow(`SELECT status FROM generation_jobs WHERE id=?`, id).Scan(&status); err != nil || status != want {
			t.Fatalf("job %s status=%q err=%v, want %q", id, status, err, want)
		}
	}
	for _, id := range []string{"x1", "x2"} {
		var experimentVoice string
		if err := handle.Reader.QueryRow(`SELECT COALESCE(voice_id,'') FROM model_experiments WHERE id=?`, id).Scan(&experimentVoice); err != nil || experimentVoice != aliceVoice {
			t.Fatalf("experiment %s voice=%q err=%v", id, experimentVoice, err)
		}
	}

	var integrity int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&integrity); err != nil || integrity != 0 {
		t.Fatalf("foreign key check found %d problems: %v", integrity, err)
	}

	// A second voice is what makes the rollback lossy, so it must refuse.
	if _, err := handle.Writer.Exec(`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('second','alice','두 번째',0,?,?)`, at, at); err != nil {
		t.Fatal(err)
	}
	// Same-account ids are not interchangeable: composite child FKs reject attaching
	// Alice's second voice to a rule/source/validation owned by her default voice.
	for _, statement := range []string{
		`INSERT INTO voice_authored_sources(id,user_id,voice_id,learning_event_id,title,tags,body,excerpt,created_at) VALUES('cross-source','alice','second','e1','x','[]','x','x','` + at + `')`,
		`INSERT INTO voice_rule_evidence(id,user_id,voice_id,rule_id,origin,payload_ref,created_at) VALUES('cross-evidence','alice','second','r1','manual','x','` + at + `')`,
		`INSERT INTO voice_profile_validation_items(id,validation_id,source_id,voice_id,user_id,position,status) VALUES('cross-item','val1','src1','second','alice',1,'pending')`,
	} {
		if _, err := handle.Writer.Exec(statement); err == nil {
			t.Fatalf("cross-voice partition insert succeeded: %q", statement)
		}
	}
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	// Publishing and purposes are independent of the voice partition and roll back first.
	if _, err := provider.DownTo(ctx, 9); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err == nil {
		t.Fatal("rollback collapsed a multi-voice database instead of refusing")
	}
	if _, err := handle.Writer.Exec(`DELETE FROM voices WHERE id='second'`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("rollback of a single-voice database failed: %v", err)
	}
	if err := handle.Reader.QueryRow(`SELECT styleguide,rules FROM voice_profiles WHERE user_id='alice'`).Scan(&gotStyle, &gotRules); err != nil || gotStyle != style || gotRules != rules {
		t.Fatalf("rollback lost legacy guidance: %q / %q err=%v", gotStyle, gotRules, err)
	}
	var voiceTables int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='voices'`).Scan(&voiceTables); err != nil || voiceTables != 0 {
		t.Fatalf("rollback kept the voices table: %d err=%v", voiceTables, err)
	}
}

func TestMigration0010PublishingConstraintsAndRollback(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-08-30T00:00:00.000000000Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO users(id,password_hash,created_at) VALUES('bob','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('av','alice','a',1,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('bv','bob','b',1,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,status,created_at,updated_at) VALUES('ap','alice','av','finalized','` + at + `','` + at + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,created_at,updated_at) VALUES('aa','alice','ath','Mac','naver_blog','` + at + `','` + at + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,created_at,updated_at) VALUES('ba','bob','bth','Mac','naver_blog','` + at + `','` + at + `')`,
		`INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('j1','alice','` + at + `')`,
		`INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('foreign','alice','` + at + `')`,
		`INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('duplicate','alice','` + at + `')`,
		`INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('retry','alice','` + at + `')`,
		`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,manifest_json,settings_json,created_at,updated_at) VALUES('j1','alice','ap','` + at + `','aa','naver_blog','queued','queued',1,'{}','{}','` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("statement failed: %v\n%s", err, statement)
		}
	}
	for _, index := range []string{
		"publish_jobs_agent_queue_idx",
		"publish_jobs_post_history_idx",
		"publish_jobs_deleted_post_history_idx",
		"publish_jobs_retryable_idx",
		"publish_jobs_expired_running_idx",
		"publish_jobs_terminal_cleanup_idx",
	} {
		var count int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&count); err != nil || count != 1 {
			t.Fatalf("publishing index %s count=%d err=%v", index, count, err)
		}
	}
	if _, err := handle.Writer.Exec(`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,manifest_json,settings_json,created_at,updated_at) VALUES('foreign','alice','ap',?,'ba','naver_blog','queued','queued',1,'{}','{}',?,?)`, at, at, at); err == nil {
		t.Fatal("cross-account agent was accepted")
	}
	if _, err := handle.Writer.Exec(`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,manifest_json,settings_json,created_at,updated_at) VALUES('duplicate','alice','ap',?,'aa','naver_blog','running','claimed',1,'{}','{}',?,?)`, at, at, at); err == nil {
		t.Fatal("second live publication was accepted")
	}
	if _, err := handle.Writer.Exec(`UPDATE publish_jobs SET status='failed' WHERE id='j1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,manifest_json,settings_json,created_at,updated_at) VALUES('retry','alice','ap',?,'aa','naver_blog','queued','queued',1,'{}','{}',?,?)`, at, at, at); err != nil {
		t.Fatalf("safe retry after failure was rejected: %v", err)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 9); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"publishing_pairings", "publishing_agents", "publish_job_ids", "publish_jobs", "publish_assets"} {
		var count int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rollback kept %s: count=%d err=%v", table, count, err)
		}
	}
}

// The service checks make ordinary refusals readable, while these database guards close the
// check/write gap between the voice, post, job, and experiment contexts. With SQLite's one
// writer, either the work row or the tombstone commits first and the other statement must fail.
func TestMigration0009SerializesVoiceDeletionAgainstNewWork(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-08-30T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-default','alice','기본 말투',1,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice-work','alice','리뷰',0,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at) VALUES('default-post','alice','voice-default','','','draft','` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at) VALUES('work-post','alice','voice-work','','','draft','` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,post_slug,user_id,voice_id,kind,status,created_at,updated_at) VALUES('active-job','work-post','alice','voice-work','generate','queued','` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}

	if _, err := handle.Writer.Exec(`UPDATE voices SET deleted_at=? WHERE id='voice-work'`, at); err == nil {
		t.Fatal("voice deletion committed while a frozen job was publishable")
	}
	if _, err := handle.Writer.Exec(`UPDATE generation_jobs SET status='done' WHERE id='active-job'`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`UPDATE voices SET deleted_at=? WHERE id='voice-work'`, at); err != nil {
		t.Fatalf("idle non-default voice did not delete: %v", err)
	}

	rejected := []string{
		`INSERT INTO generation_jobs(id,user_id,voice_id,kind,status,created_at,updated_at) VALUES('late-job','alice','voice-work','analyze_voice','queued','` + at + `','` + at + `')`,
		`INSERT INTO model_experiments(id,user_id,voice_id,stage,status,input_hash,prompt_version,created_at) VALUES('late-experiment','alice','voice-work','analyze','queued','h','v1','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at) VALUES('late-post','alice','voice-work','','','draft','` + at + `','` + at + `')`,
		`UPDATE posts SET voice_id='voice-work' WHERE slug='default-post' AND user_id='alice'`,
	}
	for _, statement := range rejected {
		if _, err := handle.Writer.Exec(statement); err == nil {
			t.Fatalf("deleted voice accepted cross-context work: %q", statement)
		}
	}

	var retained int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM posts WHERE slug='work-post' AND voice_id='voice-work'`).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("soft delete lost the existing post: retained=%d err=%v", retained, err)
	}
}

// A purpose is optional, account-scoped, and detachable: a post may have none, may never
// name another account's, and outlives the purpose it pointed at.
func TestMigration0011AddsOptionalAccountScopedPurposes(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-08-30T00:00:00.000000000Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO users(id,password_hash,created_at) VALUES('bob','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('av','alice','a',1,'` + at + `','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('bv','bob','b',1,'` + at + `','` + at + `')`,
		`INSERT INTO purposes(id,user_id,name,description,instructions,created_at,updated_at) VALUES('ap','alice','리뷰','','지침','` + at + `','` + at + `')`,
		`INSERT INTO purposes(id,user_id,name,description,instructions,created_at,updated_at) VALUES('bp','bob','리뷰','','지침','` + at + `','` + at + `')`,
		// A post with no purpose is the ordinary case, and the only one an existing row can be.
		`INSERT INTO posts(slug,user_id,voice_id,status,created_at,updated_at) VALUES('none','alice','av','draft','` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,purpose_id,status,created_at,updated_at) VALUES('with','alice','av','ap','draft','` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("statement failed: %v\n%s", err, statement)
		}
	}

	if _, err := handle.Writer.Exec(`INSERT INTO posts(slug,user_id,voice_id,purpose_id,status,created_at,updated_at) VALUES('foreign','alice','av','bp','draft',?,?)`, at, at); err == nil {
		t.Fatal("a post named another account's purpose")
	}
	if _, err := handle.Writer.Exec(`UPDATE posts SET purpose_id='bp' WHERE slug='none'`); err == nil {
		t.Fatal("a post was reassigned to another account's purpose")
	}
	if _, err := handle.Writer.Exec(`INSERT INTO purposes(id,user_id,name,description,instructions,created_at,updated_at) VALUES('dup','alice','리뷰','','지침',?,?)`, at, at); err == nil {
		t.Fatal("a duplicate name within one account was accepted")
	}

	// Deleting the purpose detaches, and deletes no post and no content.
	if _, err := handle.Writer.Exec(`DELETE FROM purposes WHERE id='ap'`); err != nil {
		t.Fatalf("delete refused: %v", err)
	}
	var posts, assigned int
	if err := handle.Reader.QueryRow(`SELECT count(*), count(purpose_id) FROM posts WHERE user_id='alice'`).Scan(&posts, &assigned); err != nil {
		t.Fatal(err)
	}
	if posts != 2 || assigned != 0 {
		t.Fatalf("detach removed posts or left assignments: posts=%d assigned=%d", posts, assigned)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 10); err != nil {
		t.Fatal(err)
	}
	var purposeTables, purposeColumns, experimentColumns int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='purposes'`).Scan(&purposeTables); err != nil || purposeTables != 0 {
		t.Fatalf("rollback kept purposes: count=%d err=%v", purposeTables, err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('posts') WHERE name='purpose_id'`).Scan(&purposeColumns); err != nil || purposeColumns != 0 {
		t.Fatalf("rollback kept posts.purpose_id: count=%d err=%v", purposeColumns, err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('model_experiments') WHERE name='purpose_name'`).Scan(&experimentColumns); err != nil || experimentColumns != 0 {
		t.Fatalf("rollback kept model_experiments.purpose_name: count=%d err=%v", experimentColumns, err)
	}
	// The rollback is about purposes only: the posts it detached are still there.
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM posts WHERE user_id='alice'`).Scan(&posts); err != nil || posts != 2 {
		t.Fatalf("rollback lost posts: count=%d err=%v", posts, err)
	}
}

// A1: the plan column arrives with the operator's existing accounts backfilled to `master`.
// Nothing about their authority may regress on the deploy that introduces the ladder, while a
// NEW account provisioned afterwards gets the column's `free` default.
func TestMigration0013BackfillsExistingAccountsToMasterAndRollsBack(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()

	beforePlans := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "0013" {
			continue
		}
		data, err := fs.ReadFile(migrationsFS, "migrations/"+entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		beforePlans[entry.Name()] = &fstest.MapFile{Data: data}
	}
	if err := migrate(ctx, handle.Writer, beforePlans); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('operator','hash','2026-08-29T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}

	var plan string
	if err := handle.Reader.QueryRow(`SELECT plan FROM users WHERE id='operator'`).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan != "master" {
		t.Fatalf("pre-existing account plan = %q, want master", plan)
	}

	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('newcomer','hash','2026-09-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow(`SELECT plan FROM users WHERE id='newcomer'`).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	if plan != "free" {
		t.Fatalf("new account plan = %q, want the free default", plan)
	}

	// The CHECK is what keeps a hand-edited row off the ladder.
	if _, err := handle.Writer.Exec(`UPDATE users SET plan='pro' WHERE id='newcomer'`); err == nil {
		t.Fatal("an off-ladder plan was accepted")
	}

	// A ledger row must not outlive its account.
	if _, err := handle.Writer.Exec(
		`INSERT INTO usage_events(user_id,kind,job_id,stage,model,prompt_tokens,completion_tokens,cost_microusd,cost_source,created_at)
		 VALUES('newcomer','generate','job','write','p/m',1,2,3,'reported','2026-09-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`DELETE FROM users WHERE id='newcomer'`); err != nil {
		t.Fatal(err)
	}
	var events int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM usage_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 0 {
		t.Fatalf("usage_events = %d, want the cascade to have removed them", events)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 12); err != nil {
		t.Fatal(err)
	}
	var columns int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_table_info('users') WHERE name='plan'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatal("migration down retained the plan column")
	}
	var accounts int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM users WHERE id='operator'`).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if accounts != 1 {
		t.Fatal("down lost the account")
	}
}

// A12/A13: the two guideline tables arrive empty, both foreign keys carry the account, and
// deleting a purpose unlinks it from every guideline in the same statement while the
// guideline rows themselves survive as "applies nowhere until rescoped".
func TestMigration0014AddsAccountScopedGuidelinesAndCascadesLinks(t *testing.T) {
	handle := openTemp(t)
	ctx := context.Background()
	if err := Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-01T00:00:00.000000000Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO users(id,password_hash,created_at) VALUES('bob','hash','` + at + `')`,
		`INSERT INTO purposes(id,user_id,name,description,instructions,created_at,updated_at) VALUES('ap','alice','리뷰','','지침','` + at + `','` + at + `')`,
		`INSERT INTO purposes(id,user_id,name,description,instructions,created_at,updated_at) VALUES('ap2','alice','후기','','지침','` + at + `','` + at + `')`,
		`INSERT INTO purposes(id,user_id,name,description,instructions,created_at,updated_at) VALUES('bp','bob','리뷰','','지침','` + at + `','` + at + `')`,
		`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('ag','alice','CCTV 언급 금지','purposes','` + at + `','` + at + `')`,
		`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('agg','alice','없는 사실 쓰지 않기','global','` + at + `','` + at + `')`,
		`INSERT INTO guideline_purposes(guideline_id,purpose_id,user_id) VALUES('ag','ap','alice')`,
		`INSERT INTO guideline_purposes(guideline_id,purpose_id,user_id) VALUES('ag','ap2','alice')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("statement failed: %v\n%s", err, statement)
		}
	}

	// A fresh database seeds nothing; the two rows above are the only ones that exist.
	var seeded int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM guidelines WHERE user_id='bob'`).Scan(&seeded); err != nil || seeded != 0 {
		t.Fatalf("migration seeded guidelines: count=%d err=%v", seeded, err)
	}

	if _, err := handle.Writer.Exec(`INSERT INTO guideline_purposes(guideline_id,purpose_id,user_id) VALUES('ag','bp','alice')`); err == nil {
		t.Fatal("a guideline linked another account's purpose")
	}
	if _, err := handle.Writer.Exec(`INSERT INTO guideline_purposes(guideline_id,purpose_id,user_id) VALUES('ag','bp','bob')`); err == nil {
		t.Fatal("a link named a guideline from another account")
	}
	if _, err := handle.Writer.Exec(`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('dup','alice','CCTV 언급 금지','global',?,?)`, at, at); err == nil {
		t.Fatal("a duplicate text within one account was accepted")
	}
	if _, err := handle.Writer.Exec(`INSERT INTO guidelines(id,user_id,text,scope,created_at,updated_at) VALUES('bad','alice','x','voice',?,?)`, at, at); err == nil {
		t.Fatal("an unknown scope was accepted")
	}

	// Deleting a purpose unlinks it and keeps every guideline row.
	if _, err := handle.Writer.Exec(`DELETE FROM purposes WHERE id='ap'`); err != nil {
		t.Fatalf("delete refused: %v", err)
	}
	var links, rows int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM guideline_purposes WHERE user_id='alice'`).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM guidelines WHERE user_id='alice'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if links != 1 || rows != 2 {
		t.Fatalf("purpose delete cascaded wrong: links=%d guidelines=%d", links, rows)
	}
	// The remaining link vanishing leaves an orphaned 'purposes' guideline, not a deletion.
	if _, err := handle.Writer.Exec(`DELETE FROM purposes WHERE id='ap2'`); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM guidelines WHERE id='ag'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("orphaning removed the guideline: count=%d err=%v", rows, err)
	}

	// Deleting a guideline cascades only its own links.
	if _, err := handle.Writer.Exec(`INSERT INTO purposes(id,user_id,name,description,instructions,created_at,updated_at) VALUES('ap3','alice','재개','','지침',?,?)`, at, at); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO guideline_purposes(guideline_id,purpose_id,user_id) VALUES('ag','ap3','alice')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`DELETE FROM guidelines WHERE id='ag'`); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM guideline_purposes`).Scan(&links); err != nil || links != 0 {
		t.Fatalf("guideline delete left links: count=%d err=%v", links, err)
	}
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM purposes WHERE id='ap3'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("guideline delete removed a purpose: count=%d err=%v", rows, err)
	}

	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 13); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"guidelines", "guideline_purposes"} {
		var remaining int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("rollback kept %s: count=%d err=%v", table, remaining, err)
		}
	}
	// The rollback is about guidelines only: the purposes they were scoped to remain.
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM purposes WHERE user_id='alice'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rollback lost purposes: count=%d err=%v", rows, err)
	}
}

func TestDeterministicPublisherMigrationFencesLegacyExecutorAndRollback(t *testing.T) {
	ctx := context.Background()
	handle := openTemp(t)
	selected := migrationsThrough(t,
		"0001_users_sessions.sql",
		"0010_publishing.sql",
		"0015_deterministic_publishing_executor.sql",
	)
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, selected, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 10); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-01T00:00:00Z"
	if _, err := handle.Writer.Exec(
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,hermes_version,created_at,updated_at) VALUES('agent','alice','token','Mac','naver_blog','legacy-0.20',?,?)`, at, at,
	); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"precommit", "committed"} {
		if _, err := handle.Writer.Exec(
			`INSERT INTO publish_job_ids(id,user_id,created_at) VALUES(?,'alice',?)`, id, at,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := handle.Writer.Exec(
		`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,progress_seq,attempt,content_revision,manifest_json,settings_json,lease_token_hash,lease_expires_at,created_at,updated_at)
		 VALUES('precommit','alice','precommit-post',?,'agent','naver_blog','running','opening_editor',2,1,1,'{}','{}','lease-pre',? ,?,?)`,
		at, at, at, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(
		`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,progress_seq,attempt,content_revision,manifest_json,settings_json,lease_token_hash,lease_expires_at,created_at,committed_at,updated_at)
		 VALUES('committed','alice','committed-post',?,'agent','naver_blog','running','committing',6,1,1,'{}','{}','lease-commit',?,?,?,?)`,
		at, at, at, at, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 15); err != nil {
		t.Fatal(err)
	}
	var legacy, executor string
	var ready int
	if err := handle.Reader.QueryRow(
		`SELECT hermes_version,executor_version,compatibility_ready FROM publishing_agents WHERE id='agent'`,
	).Scan(&legacy, &executor, &ready); err != nil {
		t.Fatal(err)
	}
	if legacy != "legacy-0.20" || executor != "" || ready != 0 {
		t.Fatalf("capability migration legacy=%q executor=%q ready=%d", legacy, executor, ready)
	}
	var status string
	var manifest, lease sql.NullString
	if err := handle.Reader.QueryRow(
		`SELECT status,manifest_json,lease_token_hash FROM publish_jobs WHERE id='precommit'`,
	).Scan(&status, &manifest, &lease); err != nil {
		t.Fatal(err)
	}
	if status != "needs_attention" || !manifest.Valid || lease.Valid {
		t.Fatalf("precommit transition status=%q manifest=%+v lease=%+v", status, manifest, lease)
	}
	if err := handle.Reader.QueryRow(
		`SELECT status,manifest_json,lease_token_hash FROM publish_jobs WHERE id='committed'`,
	).Scan(&status, &manifest, &lease); err != nil {
		t.Fatal(err)
	}
	if status != "outcome_unknown" || manifest.Valid || lease.Valid {
		t.Fatalf("committed transition status=%q manifest=%+v lease=%+v", status, manifest, lease)
	}

	// A rolled-back server updates the retired physical column. The transition
	// trigger must clear readiness and any previously recorded replacement proof.
	if _, err := handle.Writer.Exec(
		`UPDATE publishing_agents SET compatibility_ready=1,hermes_version='legacy-rollback',executor_version='postpilot-naver/1.0.0' WHERE id='agent'`,
	); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow(
		`SELECT hermes_version,executor_version,compatibility_ready FROM publishing_agents WHERE id='agent'`,
	).Scan(&legacy, &executor, &ready); err != nil {
		t.Fatal(err)
	}
	if legacy != "legacy-rollback" || executor != "" || ready != 0 {
		t.Fatalf("retired sync legacy=%q executor=%q ready=%d", legacy, executor, ready)
	}
	if _, err := handle.Writer.Exec(
		`UPDATE publishing_agents SET compatibility_ready=1,executor_version='postpilot-naver/1.0.0' WHERE id='agent'`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if err := handle.Reader.QueryRow(
		`SELECT hermes_version,compatibility_ready FROM publishing_agents WHERE id='agent'`,
	).Scan(&legacy, &ready); err != nil || legacy != "legacy-rollback" || ready != 0 {
		t.Fatalf("rollback compatibility value=%q ready=%d err=%v", legacy, ready, err)
	}
	if _, err := handle.Reader.Exec(`SELECT executor_version FROM publishing_agents`); err == nil {
		t.Fatal("rollback retained executor_version")
	}
}
