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

// regionPlacement is what the plan gives this one entry: the slot its first line
// takes and how many of them the preset draws (CLIP-147).
func regionPlacement(v declaredVisual, kind, id string) clip.RegionPlacement {
	presets := composition.DesignSelection{Intro: "b", Outro: "e"}
	if kind == "intro" {
		presets.Intro = id
	} else {
		presets.Outro = id
	}
	return clip.RegionPlacements([]composition.ResolvedElement{v.text.Resolved}, presets)[v.text.Resolved.InstanceID]
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
					visual := regionVisual(choice.kind, choice.rows...)
					v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, ratio, visual, choice.kind, choice.id, regionPlacement(visual, choice.kind, choice.id))
					if err != nil {
						return err
					}
					if err := design.VerifyRegion(choice.kind, choice.id, ratio, 0, true, choice.rows, v.manifest.Parts); err != nil {
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
						if err := design.VerifyRegion(choice.kind, choice.id, ratio, 0, true, choice.rows, parts); !errors.Is(err, design.ViolationRegion) {
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

// The presets are the project's since CLIP-139, so the plan carries them and the
// frozen document's own attributes no longer choose anything.
func TestDeclaredRegionsUseTheProjectSelection(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1" intro="a" caption="bold" outro="b"><text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>오늘의 장면</row><row>남긴 기록</row></text><text id="outro" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>다시 만나자</row><row>오늘을 남겨요</row></text></clip>`, "vertical")
	plan.IntroPreset, plan.OutroPreset = "a", "b"
	layout := measuredDeclared(t, plan)
	if len(layout.visuals) != 2 || layout.visuals[0].region.Lines[0].Y != 940 || layout.visuals[1].region.Lines[0].Y != 900 {
		t.Fatal("document design selection was ignored", layout.elements())
	}
}

func TestAuthoredAdmissionUsesSelectedRegionLimits(t *testing.T) {
	_, r := measured(t)
	for _, intro := range []string{"a", "b"} {
		body := `<clip version="1" intro="` + intro + `" caption="bold" outro="e"><text id="intro" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>하나둘셋넷다섯여섯</row></text><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`
		err := r.ValidateAuthoredInput(t.Context(), clip.PlanningInput{Ratio: "vertical", TargetDurationMS: 15000, Design: clip.ProjectDesign{IntroPreset: intro, OutroPreset: "e"}, Composition: &clip.ProjectComposition{Snapshot: clip.CompositionSnapshot{Body: body}}})
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
		filled := regionVisual("outro", "점수", "", "오늘의 기록")
		v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", filled, "outro", "e", regionPlacement(filled, "outro", "e"))
		if err != nil {
			return err
		}
		if len(v.region.Lines) != 2 || v.region.Lines[0].Y != 836 || v.region.Lines[1].Y != 1110 || len(v.region.Rules) != 1 {
			t.Fatal("empty slot reflowed", v.region)
		}
		empty := regionVisual("outro", "", "", "")
		v, err = r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", empty, "outro", "e", regionPlacement(empty, "outro", "e"))
		if err != nil {
			return err
		}
		if len(v.manifest.Parts) != 0 || len(v.region.Rules) != 0 || len(v.region.Lines) != 0 {
			t.Fatal("empty region drew parts")
		}
		// A line past the preset's last slot is drawn by nobody and refuses
		// nothing; the notice for it is the plan's (CLIP-147).
		surplus := regionVisual("intro", "첫째", "둘째", "셋째")
		drawn, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", surplus, "intro", "b", regionPlacement(surplus, "intro", "b"))
		if err != nil {
			return err
		}
		// Two drawn lines and preset B's own two rules; the third line is nowhere.
		if len(drawn.region.Lines) != 2 || len(drawn.manifest.Parts) != 4 {
			t.Fatal("the surplus line was drawn or the drawn ones were not", drawn.region.Lines, drawn.manifest.Parts)
		}
		for _, rows := range [][]string{{"한 줄\n두 줄"}, {"하나둘셋넷다섯여섯일"}} {
			visual := regionVisual("intro", rows...)
			_, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", visual, "intro", "b", regionPlacement(visual, "intro", "b"))
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

// CLIP-147: two entries of one region fill its slots in order, so the second
// entry's line is drawn on the second slot rather than over the first.
func TestTwoRegionEntriesFillConsecutiveSlots(t *testing.T) {
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	presets := composition.DesignSelection{Intro: "b", Outro: "e"}
	first, second := regionVisual("intro", "성수 곱창"), regionVisual("intro", "성수동")
	second.text.Resolved.InstanceID, second.text.Resolved.Element.ID = "second", "second"
	placements := clip.RegionPlacements(clip.ResolvedElements([]clip.PortableText{first.text, second.text}), presets)
	if err := a.WithWorkspace(t.Context(), "slots", func(ws clip.MediaWorkspace) error {
		preset, _ := design.Region("intro", "b")
		for i, entry := range []declaredVisual{first, second} {
			v, err := r.layoutDeclaredRegion(t.Context(), ws, canvas, "vertical", entry, "intro", "b", placements[entry.text.Resolved.InstanceID])
			if err != nil {
				return err
			}
			if len(v.region.Lines) != 1 || v.region.Lines[0].Y != design.RegionBaseline(preset, "vertical", preset.Slots[i].Y) {
				t.Fatal("the entry did not land on its own slot", i, v.region.Lines)
			}
			if v.manifest.Parts[0].Slot != i+1 {
				t.Fatal("the manifest named another slot", v.manifest.Parts[0].Slot)
			}
			// Only the first entry paints the preset's rules, so nothing is
			// drawn twice (CDS-73).
			if rules := len(v.region.Rules); (i == 0) != (rules > 0) {
				t.Fatal("the rules were painted by the wrong entry", i, rules)
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
