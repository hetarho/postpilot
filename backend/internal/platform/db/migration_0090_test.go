package db

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

const at0090 = "2026-09-27T00:00:00Z"

// A stored plan envelope as the old binary wrote it: a legacy opening sentence,
// fact chips on a kept and a retired cut, the native-editing marker and the two
// legacy snapshot keys — and a caption whose own text names one of those keys.
const plan0090 = `{"Notices":null,"NoticeCutRevisions":null,"Version":6,"Ratio":"vertical",` +
	`"Plan":{"DurationMS":15000,"Cuts":[` +
	`{"ID":"a","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":7500,"TransitionMS":0,"Copies":[{"Text":"\"Chips\" 이야기"}],"Chips":["위치"],"VolumePermille":1000},` +
	`{"ID":"b","SourceID":"s","Fingerprint":"f","StartMS":7500,"EndMS":15000,"TransitionMS":0,"Copies":null,"Chips":null,"VolumePermille":1000}],"Hook":"연남 김밥"},` +
	`"Focals":{"a":{"X":0.5,"Y":0.5},"b":{"X":0.5,"Y":0.5}},` +
	`"Composition":{"NativeEditing":true,"RetiredCuts":[{"ID":"old","Chips":["가격"],"Copies":null}],` +
	`"Snapshot":{"LegacyRecipe":{"Name":"legacy"},"Version":1,"Body":"<clip version=\"1\"/>","TemplateID":"legacy","Legacy":true},"Elements":[]},` +
	`"Rates":{"a":1000,"b":1000},"SourceAudio":[]}`

// provider0090 takes a fresh database to 89 and seeds what 90 retires: a
// template with no body, one the server synthesized from a recipe, and an
// outline; a project frozen from each; a finalized project; answers; and a
// staged attempt result.
func provider0090(t *testing.T) (*DB, *goose.Provider) {
	t.Helper()
	handle := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(context.Background(), 89); err != nil {
		t.Fatal(err)
	}
	legacySnapshot := `{"legacy_recipe":{"name":"legacy","fields":[],"guidance":"","accent":"","preset":"cafe","pace":""},"version":1,"body":"<clip version=\"1\"/>","template_id":"synthesized","legacy":true}`
	outlineSnapshot := `{"version":1,"body":"<clip version=\"1\"/>","template_id":"outline","legacy":false}`
	inputs := `{"version":1,"values":{},"items":{},"associations":[]}`
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, []any{at0090}},
		{`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,accent,preset,caption_pace,created_at,updated_at)
		  VALUES('bodyless','alice','bodyless','[{"label":"상호","prompt":"이름"}]','안내','[]','coral','restaurant','rapid',?,?)`, []any{at0090, at0090}},
		{`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,accent,preset,caption_pace,composition_body,composition_legacy,created_at,updated_at)
		  VALUES('synthesized','alice','synthesized','[]','','[]',NULL,'cafe','',?,1,?,?)`, []any{`<clip version="1"/>`, at0090, at0090}},
		{`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,accent,preset,caption_pace,composition_body,composition_legacy,created_at,updated_at)
		  VALUES('outline','alice','outline','[]','','[]',NULL,'','',?,0,?,?)`, []any{`<clip version="1"/>`, at0090, at0090}},
		{`INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,created_at,updated_at,cta)
		  VALUES('p-bodyless','alice','bodyless','bodyless','vertical',15000,?,?,'place')`, []any{at0090, at0090}},
		{`INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,edit_plan_json,edit_plan_revision,composition_snapshot_json,composition_inputs_json,created_at,updated_at,cta)
		  VALUES('p-legacy','alice','legacy','synthesized','vertical',15000,?,1,?,?,?,?,'save')`, []any{plan0090, legacySnapshot, inputs, at0090, at0090}},
		{`INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,edit_plan_json,edit_plan_revision,composition_snapshot_json,composition_inputs_json,created_at,updated_at)
		  VALUES('p-outline','alice','outline','outline','vertical',15000,?,1,?,?,?,?)`, []any{plan0090, outlineSnapshot, inputs, at0090, at0090}},
		{`INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,edit_plan_json,edit_plan_revision,rendered_plan_revision,composition_snapshot_json,composition_inputs_json,
		    result_key,result_id,result_content_type,result_bytes,result_duration_ms,result_created_at,source_access_revoked_at,finalized_at,finalized_plan_revision,finalized_result_key,created_at,updated_at)
		  VALUES('p-final','alice','final','outline','vertical',15000,?,1,1,?,?,'r.mp4','result','video/mp4',10,15000,?,?,?,1,'r.mp4',?,?)`, []any{plan0090, outlineSnapshot, inputs, at0090, at0090, at0090, at0090, at0090}},
		{`INSERT INTO clip_project_answers(project_id,user_id,label,answer,updated_at) VALUES('p-legacy','alice','상호','연남 김밥',?)`, []any{at0090}},
		{`INSERT INTO clip_attempt_results(job_id,user_id,project_id,expected_revision,edit_plan_json,result_key,result_content_type,result_bytes,result_duration_ms,result_created_at)
		  VALUES('job','alice','p-outline',1,?,'staged.mp4','video/mp4',10,15000,?)`, []any{plan0090, at0090}},
	} {
		if _, err := handle.Writer.Exec(statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %q: %v", statement.sql, err)
		}
	}
	return handle, provider
}

func planKeys0090(t *testing.T, handle *DB, query, id string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, path := range []string{"$.Plan.Hook", "$.Plan.Cuts[0].Chips", "$.Plan.Cuts[1].Chips", "$.Composition.NativeEditing", "$.Composition.Snapshot.Legacy", "$.Composition.Snapshot.LegacyRecipe", "$.Composition.RetiredCuts[0].Chips"} {
		var kind sql.NullString
		if err := handle.Reader.QueryRow(`SELECT json_type(`+query+`, ?)`, path, id).Scan(&kind); err != nil {
			t.Fatal(err)
		}
		if kind.Valid {
			out[path] = kind.String
		}
	}
	return out
}

// CLIP-4, CLIP-14, CLIP-68, CDS-37, CDS-51: 0090 drops what only the category-preset path
// used. A template that is not an outline is deleted and its projects detached; a snapshot
// synthesized from a recipe is dropped; every stored plan loses the keys of retired fields
// and keeps everything else, finalized ones included; the recipe columns, the call to
// action and the answers go; the finalized-content trigger comes back without cta.
func TestMigration0090DropsTheLegacyClipTemplatePath(t *testing.T) {
	ctx := context.Background()
	handle, provider := provider0090(t)
	if _, err := provider.UpTo(ctx, 90); err != nil {
		t.Fatal(err)
	}
	var templates []string
	rows, err := handle.Reader.Query(`SELECT id FROM video_templates ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		templates = append(templates, id)
	}
	rows.Close()
	if len(templates) != 1 || templates[0] != "outline" {
		t.Fatalf("templates left = %v, want only the outline", templates)
	}
	for id, want := range map[string]string{"p-bodyless": "", "p-legacy": "", "p-outline": "outline", "p-final": "outline"} {
		var template sql.NullString
		if err := handle.Reader.QueryRow(`SELECT video_template_id FROM clip_projects WHERE id = ?`, id).Scan(&template); err != nil {
			t.Fatal(err)
		}
		if template.String != want {
			t.Fatalf("%s template = %q, want %q", id, template.String, want)
		}
	}
	var snapshot, projectInputs sql.NullString
	if err := handle.Reader.QueryRow(`SELECT composition_snapshot_json, composition_inputs_json FROM clip_projects WHERE id = 'p-legacy'`).Scan(&snapshot, &projectInputs); err != nil || snapshot.Valid || projectInputs.Valid {
		t.Fatalf("a synthesized snapshot survived: %v %v %v", snapshot, projectInputs, err)
	}
	if err := handle.Reader.QueryRow(`SELECT composition_snapshot_json FROM clip_projects WHERE id = 'p-outline'`).Scan(&snapshot); err != nil || snapshot.String != `{"version":1,"body":"<clip version=\"1\"/>","template_id":"outline"}` {
		t.Fatalf("outline snapshot = %q, %v", snapshot.String, err)
	}
	for _, id := range []string{"p-legacy", "p-outline", "p-final"} {
		if keys := planKeys0090(t, handle, `(SELECT edit_plan_json FROM clip_projects WHERE id = ?)`, id); len(keys) != 0 {
			t.Fatalf("%s plan kept retired keys: %v", id, keys)
		}
	}
	if keys := planKeys0090(t, handle, `(SELECT edit_plan_json FROM clip_attempt_results WHERE job_id = ?)`, "job"); len(keys) != 0 {
		t.Fatalf("staged plan kept retired keys: %v", keys)
	}
	// Everything else in the envelope is as it was, in order: a caption's own words
	// never matched a key, and the rebuilt arrays keep their cuts.
	var plan string
	if err := handle.Reader.QueryRow(`SELECT edit_plan_json FROM clip_projects WHERE id = 'p-final'`).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	want := `{"Notices":null,"NoticeCutRevisions":null,"Version":6,"Ratio":"vertical",` +
		`"Plan":{"DurationMS":15000,"Cuts":[` +
		`{"ID":"a","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":7500,"TransitionMS":0,"Copies":[{"Text":"\"Chips\" 이야기"}],"VolumePermille":1000},` +
		`{"ID":"b","SourceID":"s","Fingerprint":"f","StartMS":7500,"EndMS":15000,"TransitionMS":0,"Copies":null,"VolumePermille":1000}]},` +
		`"Focals":{"a":{"X":0.5,"Y":0.5},"b":{"X":0.5,"Y":0.5}},` +
		`"Composition":{"RetiredCuts":[{"ID":"old","Copies":null}],` +
		`"Snapshot":{"Version":1,"Body":"<clip version=\"1\"/>","TemplateID":"legacy"},"Elements":[]},` +
		`"Rates":{"a":1000,"b":1000},"SourceAudio":[]}`
	if plan != want {
		t.Fatalf("plan =\n%s\nwant\n%s", plan, want)
	}
	for table, columns := range map[string][]string{
		"video_templates": {"information_fields", "cut_guidance", "copy_styles", "accent", "preset", "caption_pace", "composition_legacy"},
		"clip_projects":   {"cta"},
	} {
		for _, column := range columns {
			if hasColumn0077(t, handle, table, column) {
				t.Fatalf("%s.%s survived 0090", table, column)
			}
		}
	}
	if _, ok := schemaObject0077(t, handle, "table", "clip_project_answers"); ok {
		t.Fatal("clip_project_answers survived 0090")
	}
	definition, ok := schemaObject0077(t, handle, "trigger", "clip_finalized_content")
	if !ok || containsWord0090(definition, "cta") {
		t.Fatalf("finalized-content trigger = %q", definition)
	}
	if _, err := handle.Writer.Exec(`UPDATE clip_projects SET edit_plan_json = '{}' WHERE id = 'p-final'`); err == nil {
		t.Fatal("a finalized plan was rewritten after 0090")
	}
	var integrity string
	if err := handle.Reader.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, %v", integrity, err)
	}
	var violations int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign key violations = %d, %v", violations, err)
	}

	// Down brings every retired column and the answers table back empty, with
	// their DDL, for a rolled-back binary that reads them.
	if _, err := provider.DownTo(ctx, 89); err != nil {
		t.Fatal(err)
	}
	for table, columns := range map[string][]string{
		"video_templates": {"information_fields", "cut_guidance", "copy_styles", "accent", "preset", "caption_pace", "composition_legacy"},
		"clip_projects":   {"cta"},
	} {
		for _, column := range columns {
			if !hasColumn0077(t, handle, table, column) {
				t.Fatalf("%s.%s did not come back", table, column)
			}
		}
	}
	var fields, preset string
	var legacy int
	if err := handle.Reader.QueryRow(`SELECT information_fields, preset, composition_legacy FROM video_templates WHERE id = 'outline'`).Scan(&fields, &preset, &legacy); err != nil || fields != "[]" || preset != "" || legacy != 0 {
		t.Fatalf("restored template = %q %q %d, %v", fields, preset, legacy, err)
	}
	if _, err := handle.Writer.Exec(`UPDATE video_templates SET caption_pace = 'fast' WHERE id = 'outline'`); err == nil {
		t.Error("the restored caption_pace lost its CHECK")
	}
	if _, err := handle.Writer.Exec(`INSERT INTO clip_project_answers(project_id,user_id,label,answer,updated_at) VALUES('p-outline','alice','상호','연남 김밥',?)`, at0090); err != nil {
		t.Fatalf("the restored answers table refused an answer: %v", err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO clip_project_answers(project_id,user_id,label,answer,updated_at) VALUES('p-final','alice','상호','연남 김밥',?)`, at0090); err == nil {
		t.Error("the restored answers table accepted an answer on a finalized project")
	}
	definition, ok = schemaObject0077(t, handle, "trigger", "clip_finalized_content")
	if !ok || !containsWord0090(definition, "cta") {
		t.Fatalf("restored finalized-content trigger = %q", definition)
	}
}

func containsWord0090(s, word string) bool {
	for i := 0; i+len(word) <= len(s); i++ {
		if s[i:i+len(word)] != word {
			continue
		}
		before := i == 0 || s[i-1] == ',' || s[i-1] == ' '
		after := i+len(word) == len(s) || s[i+len(word)] == ',' || s[i+len(word)] == ' '
		if before && after {
			return true
		}
	}
	return false
}
