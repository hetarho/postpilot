package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

// CLIP-192: 105 lets a browser render's sampling job be cancelled and own a
// `sample` media stage, and gives the render row the grounds it measured. Both
// widened CHECKs rebuild their tables, and each table has children that cascade
// from it, so the rebuild must keep every job, stage and attempt; the way back
// drops the sampling work before the narrower CHECKs return.
func TestMigration0105AdmitsBrowserRenderSamplingWithoutLosingMediaWork(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 104); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-29T00:00:00Z"
	exec := func(statement string) error {
		_, err := handle.Writer.Exec(statement)
		return err
	}
	stage := func(id, job, key string) string {
		return `INSERT INTO clip_media_stages(id,parent_job_id,user_id,project_id,expected_revision,stage_key,operation,contract_version,input_digest,input_payload,renderer_version,asset_version,created_at,queue_deadline_at,deadline_at,lease_ttl_ns,attempt_limit) VALUES('` +
			id + `','` + job + `','alice','clip',1,'` + key + `','` + key + `',2,'digest','{}','r','a','2026-09-29T00:00:00Z','2026-09-29T00:10:00Z','2026-09-29T00:20:00Z',1000,3)`
	}
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('clip','alice','성수','vertical',30000,'` + at + `','` + at + `')`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('render','alice','clip','render_clip','running','` + at + `','` + at + `')`,
		stage("render-stage", "render", "render"),
		`INSERT INTO clip_media_attempts(id,stage_id,ordinal,worker_id,token_hash,lease_expires_at,started_at,selected_profile,runtime_manifest) VALUES('attempt','render-stage',1,'worker','hash','2026-09-29T00:01:00Z','` + at + `','cpu','{}')`,
		`INSERT INTO clip_attempt_checkpoints(project_id,user_id,job_id,checkpoint_json) VALUES('clip','alice','render','{"Version":1}')`,
		`INSERT INTO clip_browser_renders(id,user_id,project_id,plan_revision,ratio,duration_ms,has_audio,created_at) VALUES('browser','alice','clip',1,'vertical',30000,1,'` + at + `')`,
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	// What the old tables refused.
	if err := exec(`INSERT INTO generation_jobs(id,user_id,kind,status,cancel_requested_at,created_at,updated_at) VALUES('early','alice','sample_browser_render','running','` + at + `','` + at + `','` + at + `')`); err == nil {
		t.Fatal("the pre-105 table already accepted a cancelled sampling job")
	}
	if err := exec(stage("early-stage", "render", "sample")); err == nil {
		t.Fatal("the pre-105 table already accepted a sample stage")
	}
	if _, err := provider.UpTo(ctx, 105); err != nil {
		t.Fatal(err)
	}
	for table, where := range map[string]string{
		"generation_jobs":          "id='render'",
		"clip_media_stages":        "id='render-stage'",
		"clip_media_attempts":      "id='attempt'",
		"clip_attempt_checkpoints": "job_id='render'",
	} {
		var kept int
		if err := handle.Reader.QueryRow(`SELECT count(*) FROM ` + table + ` WHERE ` + where).Scan(&kept); err != nil || kept != 1 {
			t.Fatalf("the rebuild cascaded through %s: %d %v", table, kept, err)
		}
	}
	// One active clip job per project still holds: the render ends before the
	// sampling job starts.
	for _, statement := range []string{
		`UPDATE generation_jobs SET status='done', finished_at='` + at + `' WHERE id='render'`,
		`INSERT INTO generation_jobs(id,user_id,clip_project_id,kind,status,created_at,updated_at) VALUES('sample','alice','clip','sample_browser_render','queued','` + at + `','` + at + `')`,
		stage("sample-stage", "sample", "sample"),
		`UPDATE clip_browser_renders SET sample_job_id='sample', grounds_json='[{"InstanceID":"project-outro","Phrase":0}]', sampled_at='` + at + `' WHERE id='browser'`,
		`UPDATE generation_jobs SET cancel_requested_at='` + at + `' WHERE id='sample'`,
		`UPDATE generation_jobs SET status='cancelled', finished_at='` + at + `' WHERE id='sample'`,
	} {
		if err := exec(statement); err != nil {
			t.Fatalf("after 105 %q: %v", statement, err)
		}
	}
	if err := exec(`UPDATE clip_browser_renders SET grounds_json='not json' WHERE id='browser'`); err == nil {
		t.Fatal("grounds that are not JSON were stored")
	}
	if _, err := provider.DownTo(ctx, 104); err != nil {
		t.Fatalf("down: %v", err)
	}
	var left int
	if err := handle.Reader.QueryRow(`SELECT (SELECT count(*) FROM generation_jobs WHERE kind='sample_browser_render') + (SELECT count(*) FROM clip_media_stages WHERE stage_key='sample')`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("the way back kept %d pieces of sampling work: %v", left, err)
	}
	var kept int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM clip_media_attempts WHERE id='attempt'`).Scan(&kept); err != nil || kept != 1 {
		t.Fatalf("the way back lost the render's attempt: %d %v", kept, err)
	}
}
