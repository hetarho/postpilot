package db

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestMigration0046PreservesLegacyClipsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	prior := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Compare(entry.Name(), "0046_") >= 0 {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		prior[entry.Name()] = &fstest.MapFile{Data: body}
	}
	ctx := context.Background()
	if err = migrate(ctx, d.Writer, prior); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO users(id,password_hash,plan,created_at) VALUES('owner','hash','free','2026-09-01T00:00:00Z')`,
		`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,created_at,updated_at) VALUES('template','owner','legacy','[]','& exact guide','["clean"]','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z')`,
		`INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,analysis_json,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at,edit_plan_revision,rendered_plan_revision,created_at,updated_at) VALUES('project','owner','saved','template','vertical',15000,'analysis exact','plan exact','result.mp4','video/mp4',123,15000,'2026-09-01T00:00:00Z',7,6,'2026-09-01T00:00:00Z','2026-09-01T00:00:00Z')`,
		`INSERT INTO clip_project_answers(project_id,user_id,label,answer,updated_at) VALUES('project','owner','같은 질문','  exact answer  ','2026-09-01T00:00:00Z')`,
	} {
		if _, err = d.Writer.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err = Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	if err = d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	var analysis, plan, key, answer, guide string
	var revision, rendered int
	var snapshot, inputs, body sql.NullString
	err = d.Reader.QueryRow(`SELECT p.analysis_json,p.edit_plan_json,p.result_key,p.edit_plan_revision,p.rendered_plan_revision,p.composition_snapshot_json,p.composition_inputs_json,t.composition_body,t.cut_guidance,a.answer FROM clip_projects p JOIN video_templates t ON t.id=p.video_template_id JOIN clip_project_answers a ON a.project_id=p.id WHERE p.id='project'`).Scan(&analysis, &plan, &key, &revision, &rendered, &snapshot, &inputs, &body, &guide, &answer)
	if err != nil || analysis != "analysis exact" || plan != "plan exact" || key != "result.mp4" || revision != 7 || rendered != 6 || snapshot.Valid || inputs.Valid || body.Valid || guide != "& exact guide" || answer != "  exact answer  " {
		t.Fatal("migration rewrote retained data", err, analysis, plan, key, revision, rendered, snapshot, inputs, body, guide, answer)
	}
}
