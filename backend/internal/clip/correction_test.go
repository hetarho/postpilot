package clip_test

import (
	"encoding/json"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
	"reflect"
	"strings"
	"testing"
)

func correctionFixture(t *testing.T) (clip.Project, clip.CorrectionPlan) {
	t.Helper()
	sources := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "a", Fingerprint: "fa", Info: clip.MediaInfo{DurationMS: 40000, Width: 1920, Height: 1080}}, Filename: "a.mp4"}}, {Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "b", Fingerprint: "fb", Info: clip.MediaInfo{DurationMS: 40000, Width: 1920, Height: 1080}}, Filename: "b.mp4"}}}
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 19800, Cuts: []clip.Cut{{ID: "first", SourceID: "a", Fingerprint: "fa", EndMS: 10000, Focal: clip.Point{X: .3, Y: .4}, Copy: clip.Caption{Text: "hello", Anchor: "bottom", Align: "center", Style: "clean"}}, {ID: "second", SourceID: "b", Fingerprint: "fb", EndMS: 10000, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Caption{Text: "서울", Anchor: "top", Align: "left", Style: "memo"}}}}
	a, _ := json.Marshal(sources)
	raw, err := clip.EncodeEditPlan(plan, []string{"clean", "memo"})
	if err != nil {
		t.Fatal(err)
	}
	return clip.Project{Ratio: "vertical", Analysis: string(a), EditPlan: raw, EditPlanRevision: 1}, clip.CorrectionFromPlan(plan)
}
func TestCorrectionMutationsAndIntegerPersistence(t *testing.T) {
	p, draft := correctionFixture(t)
	cfg := config.ClipRender(&config.Config{})
	draft.Cuts[0], draft.Cuts[1] = draft.Cuts[1], draft.Cuts[0]
	draft.Cuts[0].StartMS = 1000
	draft.Cuts[0].EndMS = 20000
	draft.Cuts[0].VolumePermille = 123
	// The fade rides the cut it leads into, so the swap carries it along and the
	// cut now in front leads in from nothing.
	draft.Cuts[0].TransitionMS, draft.Cuts[1].TransitionMS = 0, 200
	draft.Cuts[0].Copy = clip.Caption{Text: "정확한 글자 & <copy>", Anchor: "lower_mid", Align: "center", Style: "clean", Accent: "coral", StartMS: 200, EndMS: 2000}
	draft.DurationMS = 28800
	next, styles, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if next.Ratio != "vertical" || next.Cuts[0].ID != "second" || next.Cuts[0].OriginalVolume() != .123 {
		t.Fatal(next)
	}
	raw, err := clip.EncodeEditPlan(next, styles)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, `"Volume":`) || !strings.Contains(raw, `"VolumePermille":123`) {
		t.Fatal(raw)
	}
	reloaded, _, err := clip.DecodeEditPlan(raw)
	if err != nil || !reflect.DeepEqual(clip.CorrectionFromPlan(reloaded), draft) {
		t.Fatal(reloaded, err)
	}
	draft.Cuts = draft.Cuts[:1]
	draft.DurationMS = 19000
	if _, _, err = clip.ApplyCorrection(cfg, p, draft); err != nil {
		t.Fatal("delete", err)
	}
	if err = clip.MatchRenderBatch(next, clip.SourceBatch{Sources: []clip.SourceLease{{SourceMetadata: clip.SourceMetadata{Fingerprint: "fa"}}, {SourceMetadata: clip.SourceMetadata{Fingerprint: "fb"}}}}); err != nil {
		t.Fatal(err)
	}
}
func TestCorrectionRejectsEveryInvalidMutationWithoutChangingInput(t *testing.T) {
	cases := map[string]func(*clip.CorrectionPlan){"empty": func(p *clip.CorrectionPlan) { p.Cuts = nil }, "new id": func(p *clip.CorrectionPlan) { p.Cuts[0].ID = "new" }, "duplicate": func(p *clip.CorrectionPlan) { p.Cuts[1] = p.Cuts[0] }, "source": func(p *clip.CorrectionPlan) { p.Cuts[0].SourceID = "b" }, "fingerprint": func(p *clip.CorrectionPlan) { p.Cuts[0].Fingerprint = "forged" }, "negative": func(p *clip.CorrectionPlan) { p.Cuts[0].StartMS = -1 }, "too long": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 40001 }, "empty cut": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 0 }, "fade": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 400 }, "duration": func(p *clip.CorrectionPlan) { p.DurationMS++ }, "min": func(p *clip.CorrectionPlan) { p.DurationMS = 14999 }, "max": func(p *clip.CorrectionPlan) { p.DurationMS = 90001 }, "caption before": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.StartMS = -1 }, "caption after": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.EndMS = 10001 }, "position": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Anchor = "free" }, "style": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Style = "bold" }, "accent": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Accent = "red" }, "volume low": func(p *clip.CorrectionPlan) { p.Cuts[0].VolumePermille = -1 }, "volume high": func(p *clip.CorrectionPlan) { p.Cuts[0].VolumePermille = 1001 }, "text": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Text = strings.Repeat("가", 501) }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p, input := correctionFixture(t)
			raw := p.EditPlan
			mutate(&input)
			_, _, err := clip.ApplyCorrection(config.ClipRender(&config.Config{}), p, input)
			if !errors.Is(err, clip.ErrInvalid) && !errors.Is(err, clip.ErrCopyTooLong) {
				t.Fatal(err)
			}
			if p.EditPlan != raw {
				t.Fatal("mutated original")
			}
		})
	}
}

func TestRenderBatchRequiresExactFingerprintSubset(t *testing.T) {
	p, _ := correctionFixture(t)
	plan, _, _ := clip.DecodeEditPlan(p.EditPlan)
	plan.Cuts = plan.Cuts[:1]
	for _, values := range [][]string{nil, {"wrong"}, {"fa", "fb"}, {"fa", "fa"}} {
		b := clip.SourceBatch{}
		for _, v := range values {
			b.Sources = append(b.Sources, clip.SourceLease{SourceMetadata: clip.SourceMetadata{Fingerprint: v}})
		}
		if err := clip.MatchRenderBatch(plan, b); !errors.Is(err, clip.ErrSourceState) {
			t.Fatal(values, err)
		}
	}
	if err := clip.MatchRenderBatch(plan, clip.SourceBatch{Sources: []clip.SourceLease{{SourceMetadata: clip.SourceMetadata{Fingerprint: "fa"}}}}); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyGeneratedPlanRemainsReadableAndMigratesOnSave(t *testing.T) {
	p, _ := correctionFixture(t)
	plan, _, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	legacy, _ := json.Marshal(plan)
	decoded, styles, err := clip.DecodeEditPlan(string(legacy))
	if err != nil || !reflect.DeepEqual(plan, decoded) || !reflect.DeepEqual(styles, []string{"clean", "memo"}) {
		t.Fatal(decoded, styles, err)
	}
	next, err := clip.EncodeEditPlan(decoded, styles)
	if err != nil || strings.Contains(next, `"Volume":`) {
		t.Fatal(next, err)
	}
	for _, bad := range []string{"null", `{"Version":99}`, next + " {}"} {
		if _, _, err = clip.DecodeEditPlan(bad); err == nil {
			t.Fatal("invalid persisted version accepted", bad)
		}
	}
}

// A plan the OLD binary stored, read by this one before migration 0041 has run:
// the deploy window's belt. The tokens are the migration's own, so the two can
// never disagree about what a legacy row means.
func TestDecodeMapsTheRetiredPlanVocabularyOnRead(t *testing.T) {
	legacy := `{"Version":1,"Ratio":"vertical","Plan":{"DurationMS":20000,"Cuts":[` +
		`{"ID":"one","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":10000,` +
		`"Copy":{"Text":"center","Position":"center","Style":"diary","Accent":"","StartMS":0,"EndMS":0},"VolumePermille":1000},` +
		`{"ID":"two","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":10200,` +
		`"Copy":{"Text":"diary","Position":"top","Style":"emphasis","Accent":"","StartMS":0,"EndMS":0},"VolumePermille":1000}` +
		`]},"Focals":{"one":{"X":0.5,"Y":0.5},"two":{"X":0.5,"Y":0.5}},` +
		`"CopyStyles":["clean","diary","emphasis"]}`
	plan, styles, err := clip.DecodeEditPlan(legacy)
	if err != nil {
		t.Fatal(err)
	}
	first, second := plan.Cuts[0].Copy, plan.Cuts[1].Copy
	if first.Anchor != "lower_mid" || first.Align != "center" || first.Style != "memo" {
		t.Fatalf("first caption = %+v", first)
	}
	if second.Anchor != "top" || second.Align != "center" || second.Style != "bold" {
		t.Fatalf("second caption = %+v", second)
	}
	// A caption whose own text is one of the retired words keeps it: each token
	// carries its key, or the array's own delimiters, on both sides.
	if first.Text != "center" || second.Text != "diary" {
		t.Fatalf("caption text was rewritten: %q %q", first.Text, second.Text)
	}
	if !reflect.DeepEqual(styles, []string{"clean", "memo", "bold"}) {
		t.Fatalf("approved styles = %v", styles)
	}
	// Re-encoding what was read leaves nothing of the old vocabulary behind.
	raw, err := clip.EncodeEditPlan(plan, styles)
	for _, token := range []string{`"Position":"`, `"Style":"diary"`, `"Style":"emphasis"`, `,"diary"`, `,"emphasis"`} {
		if err != nil || strings.Contains(raw, token) {
			t.Fatalf("re-encoded plan still carries %s: %v\n%s", token, err, raw)
		}
	}
	if !strings.Contains(raw, `"CopyStyles":["clean","memo","bold"]`) {
		t.Fatalf("re-encoded approved set: %s", raw)
	}
}
