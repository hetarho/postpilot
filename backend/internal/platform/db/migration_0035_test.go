package db

import (
	"context"
	"strings"
	"testing"
)

func TestMigration0035KeepsSourcesOutOfProjects(t *testing.T) {
	d := openTemp(t)
	if err := Migrate(context.Background(), d.Writer); err != nil {
		t.Fatal(err)
	}
	var ddl string
	if err := d.Reader.QueryRow("SELECT sql FROM sqlite_master WHERE name='clip_projects'").Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(ddl), "source") {
		t.Fatal("durable source data in project schema")
	}
	for _, column := range []string{"analysis_json", "edit_plan_json", "result_key", "rendered_plan_revision", "edit_plan_revision"} {
		if !strings.Contains(ddl, column) {
			t.Fatalf("missing %s", column)
		}
	}
	const at = "2026-09-10T00:00:00Z"
	if _, err := d.Writer.Exec("INSERT INTO users(id,password_hash,plan,created_at) VALUES('clip-owner','hash','free',?)", at); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ratio    string
		duration int
	}{{"portrait", 30000}, {"vertical", 14999}, {"square", 90001}} {
		if _, err := d.Writer.Exec("INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at) VALUES('bad','clip-owner','x',?,?,?,?)", tc.ratio, tc.duration, at, at); err == nil {
			t.Fatal("invalid ratio or duration accepted")
		}
	}
}
