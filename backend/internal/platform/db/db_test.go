package db

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

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
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	// Roll back 0009, 0008 and 0007 first, then exercise 0006's Down.
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
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
	if _, err = provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	// 0009 and 0008 come off first; the third Down exercises 0007.
	if _, err = provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Down(ctx); err != nil {
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
	// 0009 first, then the 0008 rollback this test is about.
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
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
