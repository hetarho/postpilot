package db

import (
	"context"
	"strings"
	"testing"
)

// 0037 rebuilds publish_jobs to widen the stage CHECK. A rebuild is where an FK parent
// quietly loses its children, its indexes or a column, so this pins all four: the new stage
// is accepted, the old one still is, an illegal one is not, the asset rows survive with
// their cascade intact, and every index from 0010 is back.
func TestMigration0037WidensTheStageCheckWithoutLosingAssetsOrIndexes(t *testing.T) {
	d := openTemp(t)
	if err := Migrate(context.Background(), d.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-10T00:00:00Z"
	seed := []string{
		"INSERT INTO users(id,password_hash,plan,created_at) VALUES('fence-owner','hash','free','" + at + "')",
		"INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,created_at,updated_at) VALUES('agent-1','fence-owner','tok','Mac','naver_blog','" + at + "','" + at + "')",
		"INSERT INTO publish_job_ids(id,user_id,created_at) VALUES('job-1','fence-owner','" + at + "')",
		"INSERT INTO publish_jobs(id,user_id,post_slug,post_created_at,agent_id,platform,status,stage,content_revision,settings_json,created_at,updated_at) " +
			"VALUES('job-1','fence-owner','slug','" + at + "','agent-1','naver_blog','running','uploading_photos',1,'{}','" + at + "','" + at + "')",
		"INSERT INTO publish_assets(job_id,user_id,ordinal,filename,source_filename,staged_key,bytes,created_at) " +
			"VALUES('job-1','fence-owner',0,'0000.jpg','source.jpg','staged/0000.jpg',4,'" + at + "')",
	}
	for _, statement := range seed {
		if _, err := d.Writer.Exec(statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}

	// The stage PUB-13 r4 added is storable, and the ones around it still are.
	for _, stage := range []string{"uploading_photos", "filling_settings", "committing"} {
		if _, err := d.Writer.Exec("UPDATE publish_jobs SET stage=? WHERE id='job-1'", stage); err != nil {
			t.Fatalf("stage %q was refused: %v", stage, err)
		}
	}
	if _, err := d.Writer.Exec("UPDATE publish_jobs SET stage='filling_setting' WHERE id='job-1'"); err == nil {
		t.Fatal("a misspelled stage was accepted; the CHECK is gone")
	}

	// The rebuild dropped and renamed an FK parent, which is exactly when children vanish.
	var assets int
	if err := d.Reader.QueryRow("SELECT count(*) FROM publish_assets WHERE job_id='job-1'").Scan(&assets); err != nil {
		t.Fatal(err)
	}
	if assets != 1 {
		t.Fatalf("publish_assets holds %d rows for the job, want 1", assets)
	}
	// And the cascade still reaches them through the renamed table.
	if _, err := d.Writer.Exec("DELETE FROM publish_jobs WHERE id='job-1'"); err != nil {
		t.Fatal(err)
	}
	if err := d.Reader.QueryRow("SELECT count(*) FROM publish_assets WHERE job_id='job-1'").Scan(&assets); err != nil {
		t.Fatal(err)
	}
	if assets != 0 {
		t.Fatalf("%d asset rows survived the parent's deletion; ON DELETE CASCADE was lost", assets)
	}

	for _, index := range []string{
		"publish_jobs_one_live_or_success_idx", "publish_jobs_agent_queue_idx",
		"publish_jobs_post_history_idx", "publish_jobs_deleted_post_history_idx",
		"publish_jobs_retryable_idx", "publish_jobs_expired_running_idx",
		"publish_jobs_terminal_cleanup_idx",
	} {
		var name string
		if err := d.Reader.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", index).Scan(&name); err != nil {
			t.Fatalf("index %s did not come back from the rebuild: %v", index, err)
		}
	}

	// Every column 0012 added survives with its constraint.
	var ddl string
	if err := d.Reader.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='publish_jobs'").Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"target_language", "content_language", "voice_source_language", "error_reason", "error_params", "technical_detail"} {
		if !strings.Contains(ddl, column) {
			t.Fatalf("column %s was lost in the rebuild", column)
		}
	}
	if !strings.Contains(ddl, "json_valid(error_params)") {
		t.Fatal("error_params lost its json constraint in the rebuild")
	}
	if strings.Contains(ddl, "publish_jobs_new") {
		t.Fatal("the rebuilt table kept its scratch name")
	}
}
