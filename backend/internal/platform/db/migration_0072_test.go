package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestMigration0072PermanentlyCutsOffPublishing(t *testing.T) {
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
	at := "2026-09-22T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `'),('bob','hash','` + at + `')`,
		`INSERT INTO sessions(token,user_id,expires_at,created_at) VALUES('human-session','alice','2026-10-22T00:00:00Z','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','Default',1,'` + at + `','` + at + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,status,content_revision,finalized_revision,created_at,updated_at) VALUES('source','alice','voice','preserve me','finalized',1,1,'` + at + `','` + at + `')`,
		`INSERT INTO images(id,post_slug,filename,r2_key,width,height,bytes,created_at) VALUES('image','source','photo.jpg','posts/alice/source/photo.jpg',100,100,123,'` + at + `')`,
		`INSERT INTO publishing_pairings(code_hash,user_id,label,expires_at,created_at) VALUES('pairing','alice','Alice Mac','2026-09-23T00:00:00Z','` + at + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,platform_account_id,platform_account_label,browser_label,compatibility_ready,executor_version,created_at,updated_at) VALUES('alice-agent','alice','alice-token','Alice Mac','naver_blog','alice-blog','Alice blog','Chrome',1,'postpilot-naver/1.0.0','` + at + `','` + at + `'),('bob-agent','bob','bob-token','Bob Mac','naver_blog','bob-blog','Bob blog','Chrome',1,'postpilot-naver/1.0.0','` + at + `','` + at + `')`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	type jobSeed struct{ id, userID, agentID, status, stage, committed, url string }
	jobs := []jobSeed{
		{"queued", "alice", "alice-agent", "queued", "queued", "", ""},
		{"running", "alice", "alice-agent", "running", "preparing", "", ""},
		{"attention", "bob", "bob-agent", "needs_attention", "opening_editor", "", ""},
		{"committed", "alice", "alice-agent", "running", "filling_settings", "2026-09-22T00:01:00Z", ""},
		{"verifying", "bob", "bob-agent", "running", "verifying", "", ""},
		{"published", "alice", "alice-agent", "published", "published", "2026-09-22T00:01:00Z", "https://blog.naver.com/alice-blog/1"},
		{"failed", "bob", "bob-agent", "failed", "preparing", "", ""},
	}
	for index, job := range jobs {
		if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO publish_job_ids(id,user_id,created_at) VALUES(?,?,?)`, job.id, job.userID, at); err != nil {
			t.Fatal(err)
		}
		var committed any
		if job.committed != "" {
			committed = job.committed
		}
		var url any
		if job.url != "" {
			url = job.url
		}
		if _, err := handle.Writer.ExecContext(ctx, `
			INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,progress_seq,attempt,content_revision,manifest_json,settings_json,lease_token_hash,lease_expires_at,platform_post_url,created_at,committed_at,updated_at)
			VALUES(?,?,?,?,?,'naver_blog',?,?,1,1,1,'{"content":"private"}','{}','lease','2026-09-23T00:00:00Z',?,?,?,?)`,
			job.id, job.userID, "post-"+job.id, at, job.agentID, job.status, job.stage, url, at, committed, at); err != nil {
			t.Fatalf("seed job %d %s: %v", index, job.id, err)
		}
	}
	if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO publish_assets(job_id,user_id,ordinal,filename,source_filename,staged_key,bytes,created_at) VALUES('queued','alice',0,'photo.jpg','photo.jpg','publishing/alice/queued/photo.jpg',123,?)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 72); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 72); err != nil {
		t.Fatal("second migration run", err)
	}

	for id, want := range map[string]string{"queued": "canceled", "running": "canceled", "attention": "canceled", "committed": "outcome_unknown", "verifying": "outcome_unknown", "published": "published", "failed": "failed"} {
		var status string
		var manifest, lease, expires any
		if err := handle.Reader.QueryRowContext(ctx, `SELECT status,manifest_json,lease_token_hash,lease_expires_at FROM publish_jobs WHERE id=?`, id).Scan(&status, &manifest, &lease, &expires); err != nil {
			t.Fatal(err)
		}
		if status != want {
			t.Errorf("job %s status=%s want=%s", id, status, want)
		}
		if want == "canceled" || want == "outcome_unknown" {
			if manifest != nil || lease != nil || expires != nil {
				t.Errorf("job %s retained executable data: manifest=%v lease=%v expires=%v", id, manifest, lease, expires)
			}
		}
	}
	var consumed, revoked string
	var ready int
	var executor string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT consumed_at FROM publishing_pairings WHERE code_hash='pairing'`).Scan(&consumed); err != nil || consumed == "" {
		t.Fatalf("pairing not consumed: %q %v", consumed, err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT revoked_at,compatibility_ready,executor_version FROM publishing_agents WHERE id='alice-agent'`).Scan(&revoked, &ready, &executor); err != nil || revoked == "" || ready != 0 || executor != "" {
		t.Fatalf("agent capability survived: revoked=%q ready=%d executor=%q err=%v", revoked, ready, executor, err)
	}
	var sessions, assets int
	var title, imageKey, publishedURL string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token='human-session'`).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("human session changed: %d %v", sessions, err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT title FROM posts WHERE slug='source'`).Scan(&title); err != nil || title != "preserve me" {
		t.Fatalf("source post changed: %q %v", title, err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT r2_key FROM images WHERE id='image'`).Scan(&imageKey); err != nil || imageKey != "posts/alice/source/photo.jpg" {
		t.Fatalf("source image changed: %q %v", imageKey, err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT COUNT(*) FROM publish_assets`).Scan(&assets); err != nil || assets != 1 {
		t.Fatalf("cleanup asset rows changed: %d %v", assets, err)
	}
	if err := handle.Reader.QueryRowContext(ctx, `SELECT platform_post_url FROM publish_jobs WHERE id='published'`).Scan(&publishedURL); err != nil || publishedURL != "https://blog.naver.com/alice-blog/1" {
		t.Fatalf("known URL changed: %q %v", publishedURL, err)
	}
	for name, statement := range map[string]string{
		"pairing":     `INSERT INTO publishing_pairings(code_hash,user_id,label,expires_at,created_at) VALUES('new','alice','Mac','2026-09-23T00:00:00Z','2026-09-22T00:00:00Z')`,
		"agent":       `INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,created_at,updated_at) VALUES('new-agent','alice','new-token','Mac','naver_blog','2026-09-22T00:00:00Z','2026-09-22T00:00:00Z')`,
		"reservation": `INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('new-job','alice','2026-09-22T00:00:00Z')`,
		"rearm":       `UPDATE publishing_agents SET revoked_at=NULL,compatibility_ready=1,executor_version='postpilot-naver/old' WHERE id='alice-agent'`,
		"lease":       `UPDATE publish_jobs SET status='running',lease_token_hash='lease',lease_expires_at='2026-09-23T00:00:00Z' WHERE id='queued'`,
		"commit":      `UPDATE publish_jobs SET committed_at='2026-09-22T01:00:00Z' WHERE id='queued'`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err == nil {
			t.Errorf("retirement guard accepted %s", name)
		}
	}
	var committed any
	var queuedStatus string
	if err := handle.Reader.QueryRowContext(ctx, `SELECT status,committed_at FROM publish_jobs WHERE id='queued'`).Scan(&queuedStatus, &committed); err != nil || queuedStatus != "canceled" || committed != nil {
		t.Fatalf("failed fence write changed authorization: status=%q committed=%v err=%v", queuedStatus, committed, err)
	}
	if _, err := provider.DownTo(ctx, 71); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('rollback-job','alice',?)`, at); err == nil {
		t.Fatal("migration down restored publishing capability")
	}
	if _, err := provider.UpTo(ctx, 72); err != nil {
		t.Fatal("reapply retirement migration", err)
	}
}
