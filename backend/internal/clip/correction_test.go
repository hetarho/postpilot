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
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 19800, Cuts: []clip.Cut{{ID: "first", SourceID: "a", Fingerprint: "fa", EndMS: 10000, Focal: clip.Point{X: .3, Y: .4}, Copy: clip.Caption{Text: "hello", Position: "bottom", Style: "clean"}}, {ID: "second", SourceID: "b", Fingerprint: "fb", EndMS: 10000, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Caption{Text: "서울", Position: "top", Style: "diary"}}}}
	a, _ := json.Marshal(sources)
	raw, err := clip.EncodeEditPlan(plan, []string{"clean", "diary"})
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
	draft.Cuts[0].Copy = clip.Caption{Text: "정확한 글자 & <copy>", Position: "center", Style: "clean", Accent: "coral", StartMS: 200, EndMS: 1000}
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
	cases := map[string]func(*clip.CorrectionPlan){"empty": func(p *clip.CorrectionPlan) { p.Cuts = nil }, "new id": func(p *clip.CorrectionPlan) { p.Cuts[0].ID = "new" }, "duplicate": func(p *clip.CorrectionPlan) { p.Cuts[1] = p.Cuts[0] }, "source": func(p *clip.CorrectionPlan) { p.Cuts[0].SourceID = "b" }, "fingerprint": func(p *clip.CorrectionPlan) { p.Cuts[0].Fingerprint = "forged" }, "negative": func(p *clip.CorrectionPlan) { p.Cuts[0].StartMS = -1 }, "too long": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 40001 }, "empty cut": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 0 }, "fade": func(p *clip.CorrectionPlan) { p.Cuts[0].EndMS = 400 }, "duration": func(p *clip.CorrectionPlan) { p.DurationMS++ }, "min": func(p *clip.CorrectionPlan) { p.DurationMS = 14999 }, "max": func(p *clip.CorrectionPlan) { p.DurationMS = 90001 }, "caption before": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.StartMS = -1 }, "caption after": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.EndMS = 10001 }, "position": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Position = "free" }, "style": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Style = "emphasis" }, "accent": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Accent = "red" }, "volume low": func(p *clip.CorrectionPlan) { p.Cuts[0].VolumePermille = -1 }, "volume high": func(p *clip.CorrectionPlan) { p.Cuts[0].VolumePermille = 1001 }, "text": func(p *clip.CorrectionPlan) { p.Cuts[0].Copy.Text = strings.Repeat("가", 501) }}
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
	if err != nil || !reflect.DeepEqual(plan, decoded) || !reflect.DeepEqual(styles, []string{"clean", "diary"}) {
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
