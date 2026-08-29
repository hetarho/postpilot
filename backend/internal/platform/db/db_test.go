package db

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
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
	// Roll back 0008 and 0007 first, then exercise 0006's Down.
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
	// 0008 is the latest migration; the second Down exercises 0007.
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
	if _, err := handle.Writer.Exec(`INSERT INTO posts(slug,user_id,title,memo,status,created_at,updated_at) VALUES('new','alice','','','draft','2026-08-29T00:00:00Z','2026-08-29T00:00:00Z')`); err != nil {
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
