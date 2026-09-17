package db

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// The design selection moved onto the project (CLIP-139), and every project
// that already exists keeps rendering what it renders now (CLIP-144): the
// backfill COPIES the selection off the document the render already reads.
func TestMigration0062CopiesTheDesignSelectionOntoEveryProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "design.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Compare(entry.Name(), "0062_") >= 0 {
			continue
		}
		raw, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = &fstest.MapFile{Data: raw}
	}
	if err := migrate(t.Context(), d.Writer, before); err != nil {
		t.Fatal(err)
	}
	body := func(intro, outro string) string {
		return `<clip version="1" intro="` + intro + `" caption="bold" outro="` + outro + `">` +
			`<text id="intro" kind="fixed" role="hook" basis="output-start"/>` +
			`<text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
	}
	project := func(id, template, snapshot string) string {
		column, value := "", ""
		if snapshot != "" {
			column, value = ",composition_snapshot_json", `,'{"version":1,"body":`+quoted(snapshot)+`,"template_id":"t"}'`
		}
		return `INSERT INTO clip_projects(id,user_id,title,video_template_id,ratio,target_duration_ms,created_at,updated_at` + column +
			`) VALUES('` + id + `','owner','p',` + template + `,'vertical',15000,'2026-09-17T00:00:00Z','2026-09-17T00:00:00Z'` + value + `)`
	}
	for _, sql := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('owner','hash','2026-09-17T00:00:00Z')`,
		`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,accent,preset,caption_pace,composition_body,created_at,updated_at) VALUES('t','owner','t','[]','','[]','teal','stay','','` + body("a", "b") + `','2026-09-17T00:00:00Z','2026-09-17T00:00:00Z')`,
		// One made from that template, one whose generation froze a DIFFERENT
		// selection, and one with no template at all.
		project("from-template", `'t'`, ""),
		project("from-snapshot", `'t'`, body("b", "e")),
		project("no-template", `NULL`, ""),
	} {
		if _, err := d.Writer.Exec(sql); err != nil {
			t.Fatal(sql, err)
		}
	}
	if err := Migrate(t.Context(), d.Writer); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ id, intro, outro string }{
		{"from-template", "a", "b"},
		{"from-snapshot", "b", "e"},
		{"no-template", "b", "e"},
	} {
		var intro, outro, styles string
		if err := d.Reader.QueryRow(`SELECT intro_preset,outro_preset,allowed_caption_styles FROM clip_projects WHERE id=?`, want.id).Scan(&intro, &outro, &styles); err != nil {
			t.Fatal(err)
		}
		if intro != want.intro || outro != want.outro || styles != `["bold"]` {
			t.Fatalf("%s took %q/%q/%s, wanted %q/%q", want.id, intro, outro, styles, want.intro, want.outro)
		}
	}
	// Every boot runs Migrate: a second pass must change nothing.
	if err := Migrate(t.Context(), d.Writer); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.Reader.QueryRow(`SELECT COUNT(*) FROM clip_projects WHERE updated_at='2026-09-17T00:00:00Z'`).Scan(&n); err != nil || n != 3 {
		t.Fatal("the backfill edited the projects it copied onto", n, err)
	}
}

func quoted(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
