package clip_test

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func correctionFixture(t *testing.T) (clip.Project, clip.CorrectionPlan) {
	t.Helper()
	sources := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "a", Fingerprint: "fa", Info: clip.MediaInfo{DurationMS: 40000, Width: 1920, Height: 1080}}, Filename: "a.mp4"}}, {Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "b", Fingerprint: "fb", Info: clip.MediaInfo{DurationMS: 40000, Width: 1920, Height: 1080}}, Filename: "b.mp4"}}}
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 19800, Cuts: []clip.Cut{{ID: "first", SourceID: "a", Fingerprint: "fa", EndMS: 10000, Focal: clip.Point{X: .3, Y: .4}, Copies: []clip.Caption{{Text: "hello", Anchor: "bottom", Align: "center", Style: "clean"}}}, {ID: "second", SourceID: "b", Fingerprint: "fb", EndMS: 10000, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Caption{{Text: "서울", Anchor: "top", Align: "left", Style: "memo"}}}}}
	a, _ := json.Marshal(sources)
	// A genuinely legacy stored row, which is what these tests are about: the
	// version-4 envelope this build still reads but no longer writes.
	raw := legacyStoredPlan(plan, []string{"clean", "memo"})
	stored, err := clip.DecodeEditPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	return clip.Project{Ratio: "vertical", Analysis: string(a), EditPlan: raw, EditPlanRevision: 1}, clip.CorrectionFromPlan(stored)
}

// legacyStoredPlan writes the version-4 envelope by hand. EncodeEditPlan writes
// version 6 now, so a test that needs a legacy row has to build one.
func legacyStoredPlan(p clip.EditPlan, styles []string) string {
	type storedCut struct {
		ID, SourceID, Fingerprint string
		StartMS, EndMS            int
		TransitionMS              int
		Copies                    []clip.Caption
		Chips                     []string
		VolumePermille            int
	}
	out := struct {
		Version int
		Ratio   string
		Plan    struct {
			DurationMS int
			Cuts       []storedCut
			Hook       string
		}
		Focals     map[string]clip.Point
		CopyStyles []string
	}{Version: 4, Ratio: p.Ratio, Focals: map[string]clip.Point{}, CopyStyles: styles}
	out.Plan.DurationMS, out.Plan.Hook = p.DurationMS, p.Hook
	for _, c := range p.Cuts {
		out.Plan.Cuts = append(out.Plan.Cuts, storedCut{c.ID, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS, c.TransitionMS, c.Copies, c.Chips, int(math.Round(c.OriginalVolume() * 1000))})
		out.Focals[c.ID] = c.Focal
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}

func TestCorrectionMutationsAndIntegerPersistence(t *testing.T) {
	p, draft := correctionFixture(t)
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	draft.Cuts[0], draft.Cuts[1] = draft.Cuts[1], draft.Cuts[0]
	draft.Cuts[0].StartMS = 1000
	draft.Cuts[0].EndMS = 20000
	draft.Cuts[0].VolumePermille = 123
	// The fade rides the cut it leads into, so the swap carries it along and the
	// cut now in front leads in from nothing.
	draft.Cuts[0].TransitionMS, draft.Cuts[1].TransitionMS = 0, 200
	draft.Cuts[0].Copies[0] = clip.Caption{Text: "정확한 글자 & <copy>", Anchor: "lower_mid", Align: "center", Style: "clean", Accent: "coral", StartMS: 200, EndMS: 2000}
	draft.DurationMS = 28800
	next, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if next.Ratio != "vertical" || next.Cuts[0].ID != "second" || next.Cuts[0].OriginalVolume() != .123 {
		t.Fatal(next)
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, `"Volume":`) || !strings.Contains(raw, `"VolumePermille":123`) {
		t.Fatal(raw)
	}
	reloaded, err := clip.DecodeEditPlan(raw)
	// The snapshot is the server's, never the draft's: saving derives it from
	// the plan's existing audio meaning and hands it back on the projection.
	expected := draft
	expected.SourceAudio = []clip.SourceAudioSetting{{SourceID: "a", Fingerprint: "fa", RetainOriginal: true}, {SourceID: "b", Fingerprint: "fb", RetainOriginal: true}}
	if err != nil || !reflect.DeepEqual(clip.CorrectionFromPlan(reloaded), expected) {
		t.Fatal(reloaded, err)
	}
	draft.Cuts = draft.Cuts[:1]
	draft.DurationMS = 19000
	if _, err = clip.ApplyCorrection(cfg, p, draft); err != nil {
		t.Fatal("delete", err)
	}
	if err = clip.MatchRenderBatch(next, clip.SourceBatch{Sources: []clip.SourceLease{{SourceMetadata: clip.SourceMetadata{Fingerprint: "fa"}}, {SourceMetadata: clip.SourceMetadata{Fingerprint: "fb"}}}}); err != nil {
		t.Fatal(err)
	}
}
func TestCorrectionRejectsEveryInvalidMutationWithoutChangingInput(t *testing.T) {
	cases := map[string]func(*clip.CorrectionPlan){"empty": func(p *clip.CorrectionPlan) { p.Cuts = nil }, "new id": func(p *clip.CorrectionPlan) { p.Cuts[0].ID = "new" }, "duplicate": func(p *clip.CorrectionPlan) { p.Cuts[1] = p.Cuts[0] }, "source": func(p *clip.CorrectionPlan) { p.Cuts[0].SourceID = "b" }, "fingerprint": func(p *clip.CorrectionPlan) { p.Cuts[0].Fingerprint = "forged" }, "negative": func(p *clip.CorrectionPlan) { p.Cuts[0].StartMS = -1 }, "too long": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 40001 }, "empty cut": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 0 }, "fade": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 400 }, "duration": func(p *clip.CorrectionPlan) { p.DurationMS++ }, "min": func(p *clip.CorrectionPlan) { p.DurationMS = 14999 }, "max": func(p *clip.CorrectionPlan) { p.DurationMS = 90001 }, "caption before": func(p *clip.CorrectionPlan) { p.Cuts[0].Copies[0].StartMS = -1 }, "caption after": func(p *clip.CorrectionPlan) { p.Cuts[0].Copies[0].EndMS = 10001 }, "position": func(p *clip.CorrectionPlan) { p.Cuts[0].Copies[0].Anchor = "free" }, "accent": func(p *clip.CorrectionPlan) { p.Cuts[0].Copies[0].Accent = "red" }, "volume low": func(p *clip.CorrectionPlan) { p.Cuts[0].VolumePermille = -1 }, "volume high": func(p *clip.CorrectionPlan) { p.Cuts[0].VolumePermille = 1001 }, "text": func(p *clip.CorrectionPlan) { p.Cuts[0].Copies[0].Text = strings.Repeat("가", 501) }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p, input := correctionFixture(t)
			raw := p.EditPlan
			mutate(&input)
			_, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, input)
			if !errors.Is(err, clip.ErrInvalid) && !errors.Is(err, clip.ErrCopyTooLong) {
				t.Fatal(err)
			}
			if p.EditPlan != raw {
				t.Fatal("mutated original")
			}
		})
	}
}

func TestRenderBatchRequiresMatchingSubsetAndAllowsRetainedSuperset(t *testing.T) {
	p, _ := correctionFixture(t)
	plan, _ := clip.DecodeEditPlan(p.EditPlan)
	plan.Cuts = plan.Cuts[:1]
	for _, values := range [][]string{nil, {"wrong"}, {"fa", "fa"}} {
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
	if err := clip.MatchRenderBatch(plan, clip.SourceBatch{Sources: []clip.SourceLease{{SourceMetadata: clip.SourceMetadata{Fingerprint: "fa"}}, {SourceMetadata: clip.SourceMetadata{Fingerprint: "fb"}}}}); err != nil {
		t.Fatal(err)
	}

}

func TestLegacyGeneratedPlanRemainsReadableAndMigratesOnSave(t *testing.T) {
	p, _ := correctionFixture(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	// What T076 actually stored: the whole plan, with ONE copy per cut where
	// there is now a list (CDS-43). Marshalling today's plan would not be a
	// legacy row at all.
	type legacyCut struct {
		ID, SourceID, Fingerprint string
		StartMS, EndMS            int
		TransitionMS              int
		Focal                     clip.Point
		Copy                      clip.Caption
		Chips                     []string
		Volume                    *float64
	}
	old := struct {
		Ratio      string
		DurationMS int
		Cuts       []legacyCut
	}{plan.Ratio, plan.DurationMS, nil}
	for _, c := range plan.Cuts {
		old.Cuts = append(old.Cuts, legacyCut{c.ID, c.SourceID, c.Fingerprint, c.StartMS, c.EndMS, c.TransitionMS, c.Focal, c.FirstCopy(), c.Chips, c.Volume})
	}
	legacy, _ := json.Marshal(old)
	decoded, err := clip.DecodeEditPlan(string(legacy))
	if err != nil || !reflect.DeepEqual(plan, decoded) {
		t.Fatal(decoded, err)
	}
	next, err := clip.EncodeEditPlan(decoded)
	if err != nil || strings.Contains(next, `"Volume":`) {
		t.Fatal(next, err)
	}
	for _, bad := range []string{"null", `{"Version":99}`, next + " {}"} {
		if _, err = clip.DecodeEditPlan(bad); err == nil {
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
	plan, err := clip.DecodeEditPlan(legacy)
	if err != nil {
		t.Fatal(err)
	}
	first, second := plan.Cuts[0].FirstCopy(), plan.Cuts[1].FirstCopy()
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
	// Re-encoding what was read leaves nothing of the old vocabulary behind.
	raw, err := clip.EncodeEditPlan(plan)
	for _, token := range []string{`"Position":"`, `"Style":"diary"`, `"Style":"emphasis"`, `,"diary"`, `,"emphasis"`} {
		if err != nil || strings.Contains(raw, token) {
			t.Fatalf("re-encoded plan still carries %s: %v\n%s", token, err, raw)
		}
	}
	if strings.Contains(raw, `"CopyStyles"`) {
		t.Fatalf("re-encoded approved set: %s", raw)
	}
}

// A plan stored before CDS-43 carried one copy per cut, and reads back as the
// one copy it was — the same words, the same placement, the same window.
func TestStoredPlanBeforeTwoCopiesUpgradesToAList(t *testing.T) {
	legacy := `{"Version":2,"Ratio":"vertical","Plan":{"DurationMS":20000,"Hook":"","Cuts":[` +
		`{"ID":"one","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":10000,"TransitionMS":0,` +
		`"Copy":{"Text":"조용한 골목","Anchor":"bottom","Align":"center","Style":"clean","Accent":"","Keyword":"","StartMS":0,"EndMS":0},"Chips":null,"VolumePermille":1000},` +
		`{"ID":"two","SourceID":"s","Fingerprint":"f","StartMS":0,"EndMS":10000,"TransitionMS":0,` +
		`"Copy":{"Text":"9900원","Anchor":"top","Align":"left","Style":"memo","Accent":"","Keyword":"","StartMS":0,"EndMS":0},"Chips":null,"VolumePermille":1000}]},` +
		`"Focals":{"one":{"X":0.5,"Y":0.5},"two":{"X":0.5,"Y":0.5}},"CopyStyles":["clean","memo"]}`
	plan, err := clip.DecodeEditPlan(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Cuts) != 2 || len(plan.Cuts[0].Copies) != 1 || len(plan.Cuts[1].Copies) != 1 {
		t.Fatalf("the one copy did not become a list of one: %+v", plan.Cuts)
	}
	if plan.Cuts[0].FirstCopy().Text != "조용한 골목" || plan.Cuts[1].FirstCopy().Style != "memo" {
		t.Fatalf("the stored copy changed: %+v", plan.Cuts)
	}
	// Saved again, it is a version-6 assembly plan: the same result, now stating
	// its 1x rate and the original audio meaning its per-cut volume carried.
	next, err := clip.EncodeEditPlan(plan)
	if err != nil || !strings.Contains(next, `"Version":6`) || !strings.Contains(next, `"Copies":[`) ||
		!strings.Contains(next, `"Rates":{"one":1000,"two":1000}`) ||
		!strings.Contains(next, `"SourceAudio":[{"SourceID":"s","Fingerprint":"f","RetainOriginal":true}]`) {
		t.Fatalf("%s %v", next, err)
	}
	// The two cuts select the same footage twice. A plan saved before the rule
	// keeps that overlap and stays readable; only a correction is refused.
	if len(clip.SourceOverlaps(plan.Cuts)) != 1 {
		t.Fatal("legacy overlap lost")
	}
	again, err := clip.DecodeEditPlan(next)
	if err != nil || !reflect.DeepEqual(plan, again) {
		t.Fatalf("%+v %v", again, err)
	}
}
