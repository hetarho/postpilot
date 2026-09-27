package db

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// CLIP-59, CLIP-185: 93 removes every guide from a stored video-template body and from a
// project's frozen snapshot body — one guide, several, a self-closing one, one alone on its line —
// and leaves a body without one, the rest of each snapshot and updated_at as they were.
func TestMigration0093DropsGuidesFromClipBodies(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 92); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at); err != nil {
		t.Fatal(err)
	}
	bodies := map[string][2]string{
		"one":     {`<clip version="1"><guide>말투</guide><stage name="외관">입구</stage></clip>`, `<clip version="1"><stage name="외관">입구</stage></clip>`},
		"several": {`<clip version="1"><guide>a</guide><stage name="s">x</stage><guide>b</guide><guide/></clip>`, `<clip version="1"><stage name="s">x</stage></clip>`},
		"line":    {"<clip version=\"1\">\n<guide>천천히\n보여주세요</guide>\n<stage name=\"s\">x</stage>\n</clip>", "<clip version=\"1\">\n\n<stage name=\"s\">x</stage>\n</clip>"},
		"none":    {`<clip version="1"><stage name="s">x</stage></clip>`, `<clip version="1"><stage name="s">x</stage></clip>`},
	}
	for id, body := range bodies {
		if _, err := handle.Writer.Exec(`INSERT INTO video_templates(id,user_id,name,composition_body,created_at,updated_at) VALUES(?,'alice',?,?,?,?)`,
			id, id, body[0], at, at); err != nil {
			t.Fatal(err)
		}
		snapshot, _ := json.Marshal(map[string]any{"version": 1, "body": body[0], "template_id": id})
		if _, err := handle.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,composition_snapshot_json,created_at,updated_at)
			VALUES(?,'alice',?,?,'vertical',15000,?,?,?)`, "p-"+id, id, id, string(snapshot), at, at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := provider.UpTo(ctx, 93); err != nil {
		t.Fatal(err)
	}
	for id, body := range bodies {
		var got, updated string
		if err := handle.Reader.QueryRow(`SELECT composition_body, updated_at FROM video_templates WHERE id=?`, id).Scan(&got, &updated); err != nil {
			t.Fatal(err)
		}
		if got != body[1] || updated != at {
			t.Errorf("template %s = %q (updated %s), want %q", id, got, updated, body[1])
		}
		var raw string
		if err := handle.Reader.QueryRow(`SELECT composition_snapshot_json, updated_at FROM clip_projects WHERE id=?`, "p-"+id).Scan(&raw, &updated); err != nil {
			t.Fatal(err)
		}
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot["body"] != body[1] || snapshot["template_id"] != id || snapshot["version"] != float64(1) || updated != at {
			t.Errorf("snapshot %s = %s (updated %s), want body %q", id, raw, updated, body[1])
		}
	}
}

// CLIP-59, CLIP-112: 93 strips `<guide>` from every stored template and every frozen snapshot,
// a finalized project's included, and leaves the finalized guard as it was. Production held a
// finalized project with a guide, and the guard refused the rewrite, so no deploy could boot.
func TestMigration0093StripsGuidesFromFinalizedSnapshotsToo(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 92); err != nil {
		t.Fatal(err)
	}
	const at = "2026-09-28T00:00:00Z"
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := handle.Writer.Exec(statement, args...); err != nil {
			t.Fatalf("%q: %v", statement, err)
		}
	}
	body := `<clip version="1"><guide>천천히</guide><field id="place" label="장소"/><guide/></clip>`
	exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, at)
	exec(`INSERT INTO video_templates(id,user_id,name,created_at,updated_at,composition_body) VALUES('t','alice','여행',?,?,?)`, at, at, body)
	raw, _ := json.Marshal(map[string]any{"version": 1, "body": body})
	snapshot := string(raw)
	insert := `INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,created_at,updated_at,composition_snapshot_json,edit_plan_revision,rendered_plan_revision,result_key,result_id,source_access_revoked_at,finalized_at,finalized_plan_revision,finalized_result_key) VALUES(?,'alice','제주','vertical',30000,?,?,?,?,?,?,?,?,?,?,?)`
	exec(insert, "open", at, at, snapshot, 0, 0, nil, nil, nil, nil, nil, nil)
	exec(insert, "final", at, at, snapshot, 1, 1, "key", "result", at, at, 1, "key")

	if _, err := provider.UpTo(ctx, 93); err != nil {
		t.Fatalf("93 over a finalized project with a guide: %v", err)
	}
	var stored string
	if err := handle.Reader.QueryRow(`SELECT composition_body FROM video_templates WHERE id='t'`).Scan(&stored); err != nil || strings.Contains(stored, "guide") {
		t.Fatalf("template body after 93 = %q (%v)", stored, err)
	}
	for _, id := range []string{"open", "final"} {
		if err := handle.Reader.QueryRow(`SELECT json_extract(composition_snapshot_json,'$.body') FROM clip_projects WHERE id=?`, id).Scan(&stored); err != nil || strings.Contains(stored, "guide") || !strings.Contains(stored, `<field id="place"`) {
			t.Fatalf("%s snapshot after 93 = %q (%v)", id, stored, err)
		}
	}
	// The guard is back: a finalized project's frozen content still cannot change.
	if _, err := handle.Writer.Exec(`UPDATE clip_projects SET title='다른' WHERE id='final'`); err == nil || !strings.Contains(err.Error(), "clip finalized") {
		t.Fatalf("the finalized guard did not return: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("the rest of the migrations after 93: %v", err)
	}
}
