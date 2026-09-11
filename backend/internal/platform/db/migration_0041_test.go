package db

import (
	"context"
	"io/fs"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/pressly/goose/v3"
)

// 0041 carries stored plans and templates into the CDS vocabulary. It is exercised
// on a database stopped at 40, because the rows have to be written in the old
// vocabulary the CHECK-free TEXT columns accepted before it.
func TestMigration0041RewritesStoredPlansAndTemplatesIntoTheCDSVocabulary(t *testing.T) {
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
	if _, err := provider.UpTo(ctx, 40); err != nil {
		t.Fatal(err)
	}
	at := "2026-09-11T00:00:00Z"
	if _, err := handle.Writer.Exec(
		"INSERT INTO users (id, password_hash, created_at) VALUES ('alice','hash',?)", at,
	); err != nil {
		t.Fatal(err)
	}
	templates := map[string]string{
		// A template without 깔끔하게 at all, and one that already has it.
		"legacy":   `["diary","emphasis"]`,
		"withitcd": `["clean","diary"]`,
		"empty":    `[]`,
	}
	for id, styles := range templates {
		if _, err := handle.Writer.Exec(
			`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,accent,created_at,updated_at)
			 VALUES(?,'alice',?,'[]','',?,'coral',?,?)`, id, id, styles, at, at,
		); err != nil {
			t.Fatal(err)
		}
	}
	// Exactly what EncodeEditPlan wrote before T102: Go field names, no whitespace.
	legacyPlan := `{"Version":1,"Ratio":"vertical","Plan":{"DurationMS":20000,"Cuts":[` +
		`{"ID":"one","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":10000,` +
		`"Copy":{"Text":"오늘 \"여기\"","Position":"center","Style":"diary","Accent":"coral","StartMS":0,"EndMS":0},"VolumePermille":1000},` +
		`{"ID":"two","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":10200,` +
		`"Copy":{"Text":"좋았다","Position":"bottom","Style":"emphasis","Accent":"","StartMS":0,"EndMS":0},"VolumePermille":1000}` +
		`]},"Focals":{"one":{"X":0.5,"Y":0.5},"two":{"X":0.5,"Y":0.5}},` +
		`"CopyStyles":["clean","diary","emphasis"]}`
	if _, err := handle.Writer.Exec(
		`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,edit_plan_json,created_at,updated_at)
		 VALUES('project','alice','제주','vertical',20000,?,?,?)`, legacyPlan, at, at,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}

	for id, want := range map[string]string{
		"legacy":   `["clean","memo","bold"]`,
		"withitcd": `["clean","memo"]`,
		"empty":    `["clean"]`,
	} {
		var got string
		if err := handle.Reader.QueryRow(`SELECT copy_styles FROM video_templates WHERE id=?`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s copy_styles = %s want %s", id, got, want)
		}
		var styles []string
		if err := jsonInto(got, &styles); err != nil || !clip.ValidCopyStyles(styles) {
			t.Fatalf("%s is not an approved style set: %s (%v)", id, got, err)
		}
	}
	var stored string
	if err := handle.Reader.QueryRow(`SELECT edit_plan_json FROM clip_projects WHERE id='project'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	plan, styles, err := clip.DecodeEditPlan(stored)
	if err != nil {
		t.Fatalf("migrated plan no longer decodes: %v\n%s", err, stored)
	}
	if len(plan.Cuts) != 2 {
		t.Fatal(len(plan.Cuts))
	}
	first, second := plan.Cuts[0].FirstCopy(), plan.Cuts[1].FirstCopy()
	if first.Anchor != "lower_mid" || first.Align != "center" || first.Style != "memo" {
		t.Fatalf("first caption = %+v", first)
	}
	if second.Anchor != "bottom" || second.Align != "center" || second.Style != "bold" {
		t.Fatalf("second caption = %+v", second)
	}
	// The quoted text is the reason the tokens carry their own key and quotes.
	if first.Text != `오늘 "여기"` {
		t.Fatalf("caption text was rewritten: %q", first.Text)
	}
	if len(styles) != 3 || styles[0] != "clean" || styles[1] != "memo" || styles[2] != "bold" {
		t.Fatalf("approved styles = %v", styles)
	}
	// The read-time belt is idempotent over an already migrated row.
	again, _, err := clip.DecodeEditPlan(stored)
	if err != nil || again.Cuts[0].Copies[0].Anchor != "lower_mid" {
		t.Fatalf("second decode changed the plan: %v", err)
	}
}
