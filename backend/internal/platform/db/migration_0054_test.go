package db

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/pressly/goose/v3"
)

func TestMigration0054PreservesAuthoredBytesAndFrozenContent(t *testing.T) {
	d := openTemp(t)
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(t.Context(), 53); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','2026-09-15T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/restaurant-v2-before-design-selection.xml")
	if err != nil {
		t.Fatal(err)
	}
	restaurant := string(raw)
	original := `<clip version="1" styles="simple" accent="teal"><field id="literal" label="quoted &gt; value">keep styles="simple" and style="auto" exactly</field><text id="kept" kind="fixed" role="caption" basis="whole" style="simple">keep style="simple"</text></clip>`
	selected := `<clip version="1" intro="a" caption="bold" outro="b" styles="simple"><text id="kept" kind="fixed" role="caption" basis="whole" style="simple">same</text></clip>`
	for id, body := range map[string]string{"restaurant": restaurant, "literal": original, "selected": selected} {
		if _, err = d.Writer.Exec(`INSERT INTO video_templates(id,user_id,name,information_fields,cut_guidance,copy_styles,composition_body,created_at,updated_at) VALUES(?,'alice',?,'[]','','["simple"]',?,'2026-09-15T00:00:00Z','2026-09-15T00:00:00Z')`, id, id, body); err != nil {
			t.Fatal(err)
		}
	}
	plan := `{"Version":4,"Ratio":"vertical","Plan":{"DurationMS":15000,"Hook":"Styles literal","Cuts":[]},"CopyStyles":["simple"],"Styles":["bold"]}`
	if _, err = d.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,edit_plan_json,created_at,updated_at) VALUES('plan','alice','fixture','vertical',15000,?,'2026-09-15T00:00:00Z','2026-09-15T00:00:00Z')`, plan); err != nil {
		t.Fatal(err)
	}
	// Production also contains confirmed results: their exact plan is immutable,
	// including retired style fields. A migration must leave the guard intact.
	if _, err = d.Writer.Exec(`INSERT INTO clip_projects(id,user_id,title,ratio,target_duration_ms,edit_plan_json,edit_plan_revision,rendered_plan_revision,result_key,result_id,finalized_at,finalized_plan_revision,finalized_result_key,source_access_revoked_at,created_at,updated_at)
		VALUES('frozen','alice','confirmed','vertical',15000,?,1,1,'results/frozen.mp4','frozen-result','2026-09-15T00:00:00Z',1,'results/frozen.mp4','2026-09-15T00:00:00Z','2026-09-15T00:00:00Z','2026-09-15T00:00:00Z')`, plan); err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(t.Context(), 54); err != nil {
		t.Fatal(err)
	}
	limits := composition.Limits{SourceChars: 16000, Nodes: 1000, Fields: 10, Items: 20, Cuts: 100, Cues: 1000, LabelChars: 100, PromptChars: 4000, AnswerChars: 4000, CopyChars: 4000, GuideChars: 12000, MaxDurationMS: 90000}
	for id, body := range map[string]string{"restaurant": restaurant, "literal": original, "selected": selected} {
		want := body
		if id == "restaurant" {
			want = strings.ReplaceAll(strings.ReplaceAll(want, ` styles="simple"`, ``), ` style="auto"`, ``)
			want = strings.ReplaceAll(want, ` style="simple"`, ``)
		} else {
			want = strings.Replace(want, ` styles="simple"`, "", 1)
			at := strings.Index(want, "<text")
			want = want[:at] + strings.Replace(want[at:], ` style="simple"`, "", 1)
		}
		if id != "selected" {
			want = strings.Replace(want, `version="1"`, `version="1" intro="b" caption="bold" outro="e"`, 1)
		}
		var got, styles string
		if err = d.Reader.QueryRow(`SELECT composition_body,copy_styles FROM video_templates WHERE id=?`, id).Scan(&got, &styles); err != nil {
			t.Fatal(err)
		}
		if got != want || styles != "[]" {
			t.Fatalf("%s changed authored bytes\ngot: %s\nwant: %s", id, got, want)
		}
		doc, p := composition.ReadStored(got, limits)
		if p != nil {
			t.Fatal(id, p)
		}
		if id == "restaurant" {
			if doc.Sections[len(doc.Sections)-1].Elements[1].ID != "closing_verdict" {
				t.Fatal("ending moved")
			}
			if _, p := composition.Parse(got, limits); p == nil || p.Reason != "invalid_skeleton" || p.ElementID != "closing_verdict" {
				t.Fatal(p)
			}
		}
	}
	var got string
	if err = d.Reader.QueryRow(`SELECT edit_plan_json FROM clip_projects WHERE id='plan'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, `"CopyStyles"`) || strings.Contains(got, `"Styles"`) || !strings.Contains(got, `"Hook":"Styles literal"`) {
		t.Fatal(got)
	}
	if err = d.Reader.QueryRow(`SELECT edit_plan_json FROM clip_projects WHERE id='frozen'`).Scan(&got); err != nil || got != plan {
		t.Fatalf("confirmed plan changed: %q: %v", got, err)
	}
	if _, err = d.Writer.Exec(`UPDATE clip_projects SET edit_plan_json='{}' WHERE id='frozen'`); err == nil || !strings.Contains(err.Error(), "clip finalized") {
		t.Fatalf("confirmation guard lost: %v", err)
	}
	if err = Migrate(t.Context(), d.Writer); err != nil {
		t.Fatalf("restart after migration: %v", err)
	}
}
