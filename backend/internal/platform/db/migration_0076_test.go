package db

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0076RequiresCleanupThenRemovesPublishingSchema(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 71); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-22T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES('human-session','alice','2026-10-22T00:00:00Z','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','Default',1,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,status,content,content_revision,finalized_revision,finalized_at,created_at,updated_at) VALUES('source','alice','voice','manual export','finalized','{"title":"manual export","blocks":[{"type":"TEXT","content":"preserve"}]}',1,1,'` + at + `','` + at + `','` + at + `')`,
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('image','source','photo.jpg','posts/alice/source/photo.jpg',100,100,123,'` + at + `')`,
		`INSERT INTO publishing_pairings(code_hash,user_id,label,expires_at,created_at) VALUES('pair','alice','Mac','2026-09-23T00:00:00Z','` + at + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,created_at,updated_at) VALUES('agent','alice','token','Mac','naver_blog','` + at + `','` + at + `')`,
		`INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('job','alice','` + at + `')`,
		`INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,settings_json,created_at,updated_at) VALUES('job','alice','source','` + at + `','agent','naver_blog','failed','preparing',1,'{}','` + at + `','` + at + `')`,
		`INSERT INTO publish_assets(job_id,user_id,ordinal,filename,source_filename,staged_key,bytes,created_at) VALUES('job','alice',0,'copy.jpg','photo.jpg','publishing/alice/job/copy.jpg',123,'` + at + `')`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if _, err := provider.UpTo(ctx, 72); err != nil {
		t.Fatal(err)
	}

	if _, err := provider.UpTo(ctx, 76); err == nil || !strings.Contains(err.Error(), "publishing cleanup required before final removal") {
		t.Fatalf("dirty final migration error = %v", err)
	}
	for _, table := range []string{"publish_assets", "publish_jobs", "publish_job_ids", "publishing_agents", "publishing_pairings"} {
		var rows int
		if err := handle.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("dirty refusal changed %s: rows=%d err=%v", table, rows, err)
		}
	}

	// This is the database half of the already-verified T312 cleanup sequence. Object
	// removal and its receipt happen before this final migration and are deliberately not
	// simulated by SQL here.
	for _, statement := range []string{
		`DELETE FROM publish_assets`,
		`DELETE FROM publish_jobs`,
		`DELETE FROM publish_job_ids`,
		`DELETE FROM publishing_agents`,
		`DELETE FROM publishing_pairings`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := provider.UpTo(ctx, 76); err != nil {
		t.Fatalf("clean final migration: %v", err)
	}
	assertPublishingTablesAbsent(t, handle)

	var sessions, images int
	var title, content, key string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token='human-session'`).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("human session changed: %d %v", sessions, err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT title,content FROM posts WHERE slug='source'`).Scan(&title, &content); err != nil || title != "manual export" || !strings.Contains(content, "preserve") {
		t.Fatalf("manual export changed: title=%q content=%q err=%v", title, content, err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT COUNT(*),MIN(r2_key) FROM images WHERE post_slug='source'`).Scan(&images, &key); err != nil || images != 1 || key != "posts/alice/source/photo.jpg" {
		t.Fatalf("source image changed: count=%d key=%q err=%v", images, key, err)
	}

	if _, err := provider.DownTo(ctx, 75); err != nil {
		t.Fatal(err)
	}
	assertPublishingTablesAbsent(t, handle)
}

func assertPublishingTablesAbsent(t *testing.T, handle *DB) {
	t.Helper()
	for _, table := range []string{"publish_assets", "publish_jobs", "publish_job_ids", "publishing_agents", "publishing_pairings"} {
		var count int
		if err := handle.Reader.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("retired table %s remains: count=%d err=%v", table, count, err)
		}
	}
}
