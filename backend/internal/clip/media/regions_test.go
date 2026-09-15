package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

func regionVisual(kind string, rows ...string) declaredVisual {
	role := "hook"
	if kind == "outro" {
		role = "ending"
	}
	text := clip.PortableText{Accent: "coral", Resolved: composition.ResolvedElement{Element: composition.Element{ID: "owned-region", Role: role, Kind: "fixed", Span: composition.Span{Line: 7}, Align: "center", Position: "auto", Style: "auto"}, StartMS: 0, EndMS: 2500}}
	for _, row := range rows {
		text.Resolved.Rows = append(text.Resolved.Rows, composition.ResolvedRow{Text: row})
	}
	return declaredVisual{text: text, manifest: declaredManifest(text)}
}

func checkRegionPresets(t *testing.T, a *Adapter, r *Rendering, raster bool) {
	t.Helper()
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, choice := range []struct {
			kind, id string
			rows     []string
		}{
			{"intro", "a", []string{"오늘의 장면", "직접 남긴 기록"}},
			{"intro", "b", []string{"오늘의 장면", "직접 남긴 기록"}},
			{"outro", "b", []string{"다시 만나자", "오늘의 기록을 남겨요"}},
			{"outro", "e", []string{"오늘의 점수", "9.5", "다시 보고 싶은 장면"}},
		} {
			t.Run(ratio+"/"+choice.kind+choice.id, func(t *testing.T) {
				canvas, _ := clip.ClipCanvas(ratio)
				if err := a.WithWorkspace(t.Context(), "region", func(ws clip.MediaWorkspace) error {
					v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, ratio, regionVisual(choice.kind, choice.rows...), choice.kind, choice.id)
					if err != nil {
						return err
					}
					if err := design.VerifyRegion(choice.kind, choice.id, ratio, choice.rows, v.manifest.Parts); err != nil {
						return err
					}
					preset, _ := design.Region(choice.kind, choice.id)
					for i, line := range v.region.Lines {
						if line.Y != design.RegionBaseline(preset, ratio, preset.Slots[i].Y) || line.Size != design.RegionType(preset.Slots[i], ratio).Size {
							t.Fatal("changed baseline or size", line)
						}
					}
					for _, part := range v.manifest.Parts {
						if !inside(clip.Region(part.Region), canvas.Safe) || part.FontSize > 0 && part.FontSize < design.MinTypeSize() {
							t.Fatal("unsafe part", part)
						}
					}
					body, err := r.declaredSVG(canvas, v)
					if err != nil {
						return err
					}
					if strings.Contains(body, design.Accent["coral"]) || strings.Contains(body, `rx=`) {
						t.Fatal("region acquired an accent or rounded plate", body)
					}
					if raster {
						_, err = r.rasterize(t.Context(), ws, canvas, body, "region-proof")
						return err
					}
					golden(t, fmt.Sprintf("region-%s-%s-%s.svg", choice.kind, choice.id, ratio), body)
					for _, mutate := range []func(*design.Element){func(p *design.Element) { p.BaselineY++ }, func(p *design.Element) { p.Region.Y++ }, func(p *design.Element) { p.Fill = "#FF6B57" }} {
						parts := slices.Clone(v.manifest.Parts)
						mutate(&parts[0])
						if err := design.VerifyRegion(choice.kind, choice.id, ratio, choice.rows, parts); !errors.Is(err, design.ViolationRegion) {
							t.Fatal("V20 accepted a changed preset", err)
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestRegionPresetsOnEveryRatio(t *testing.T) {
	a, r := measured(t)
	checkRegionPresets(t, a, r, false)
}

func TestDeclaredRegionsUseDocumentSelection(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1" intro="a" caption="bold" outro="b"><text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>오늘의 장면</row><row>남긴 기록</row></text><text id="outro" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>다시 만나자</row><row>오늘을 남겨요</row></text></clip>`, "vertical")
	layout := measuredDeclared(t, plan)
	if len(layout.visuals) != 2 || layout.visuals[0].region.Lines[0].Y != 940 || layout.visuals[1].region.Lines[0].Y != 900 {
		t.Fatal("document design selection was ignored", layout.elements())
	}
}

func TestAuthoredAdmissionUsesSelectedRegionLimits(t *testing.T) {
	_, r := measured(t)
	for _, intro := range []string{"a", "b"} {
		body := `<clip version="1" intro="` + intro + `" caption="bold" outro="e"><text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>하나둘셋넷다섯여섯</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
		err := r.ValidateAuthoredInput(t.Context(), clip.PlanningInput{Ratio: "vertical", TargetDurationMS: 15000, Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Body: body}}})
		if intro == "b" {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			var problem *composition.Problem
			if !errors.As(err, &problem) || problem.ElementID != "intro" || problem.Reason != "copy_limit" {
				t.Fatal("intro A admitted a ninth headline character", err)
			}
		}
	}
}

func TestRegionSlotsKeepTheirPositionsAndRefuseOverflow(t *testing.T) {
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	if err := a.WithWorkspace(t.Context(), "slots", func(ws clip.MediaWorkspace) error {
		v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", regionVisual("outro", "점수", "", "오늘의 기록"), "outro", "e")
		if err != nil {
			return err
		}
		if len(v.region.Lines) != 2 || v.region.Lines[0].Y != 836 || v.region.Lines[1].Y != 1110 || len(v.region.Rules) != 1 {
			t.Fatal("empty slot reflowed", v.region)
		}
		v, err = r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", regionVisual("outro", "", "", ""), "outro", "e")
		if err != nil {
			return err
		}
		if len(v.manifest.Parts) != 0 || len(v.region.Rules) != 0 || len(v.region.Lines) != 0 {
			t.Fatal("empty region drew parts")
		}
		for _, rows := range [][]string{{"첫째", "둘째", "셋째"}, {"한 줄\n두 줄"}, {"하나둘셋넷다섯여섯일"}} {
			_, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", regionVisual("intro", rows...), "intro", "b")
			var problem *composition.Problem
			if !errors.As(err, &problem) || problem.Reason != "copy_limit" || problem.ElementID != "owned-region" || problem.Line != 7 {
				t.Fatalf("%v: %v", rows, err)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRegionPresetsRealFontSmoke(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real bundled-font renderer gate")
	}
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	checkRegionPresets(t, a, r, true)
}

func TestLegacyHookOverflowIsIdentifiedBeforeRerender(t *testing.T) {
	a, r := measured(t)
	for _, hook := range []string{"한 줄\n두 줄", "하나둘셋넷다섯여섯일"} {
		plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Hook: hook,
			Cuts: []clip.Cut{{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}}
		if err := a.WithWorkspace(t.Context(), "legacy-region", func(ws clip.MediaWorkspace) error {
			_, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{{ID: "s", Fingerprint: "s", Info: clip.MediaInfo{DurationMS: 15000}}}, func(_ context.Context, _ string, _ func(clip.MediaSource) error) error {
				t.Fatal("overflow fetched source media")
				return nil
			})
			var problem *composition.Problem
			if !errors.As(err, &problem) || problem.Reason != "copy_limit" || problem.ElementID != "legacy-hook" {
				t.Fatalf("%q: %v", hook, err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAuthoredRegionAdmissionChecksOnlyFixedRows(t *testing.T) {
	_, r := measured(t)
	for _, kind := range []string{"fixed", "ai"} {
		body := `<clip version="1" intro="b" caption="bold" outro="e"><text id="intro" kind="` + kind + `" role="hook" basis="output-start"><row kind="fixed">직접 입력</row><row kind="ai">Write a grounded phrase that is much longer than the generated slot limit.</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
		in := clip.PlanningInput{Ratio: "vertical", TargetDurationMS: 15000, Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Body: body}}}
		if err := r.ValidateAuthoredInput(t.Context(), in); err != nil {
			t.Fatal(err)
		}
		in.Composition.Snapshot.Body = strings.Replace(body, "직접 입력", "하나둘셋넷다섯여섯일곱", 1)
		var problem *composition.Problem
		if err := r.ValidateAuthoredInput(t.Context(), in); !errors.As(err, &problem) || problem.ElementID != "intro" || problem.Reason != "copy_limit" {
			t.Fatal(err)
		}
	}
}
