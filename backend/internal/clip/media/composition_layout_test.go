package media

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func measuredDeclared(t *testing.T, plan clip.EditPlan) declaredLayout {
	t.Helper()
	a, r := measured(t)
	var layout declaredLayout
	if err := a.WithWorkspace(t.Context(), "native-measure", func(ws clip.MediaWorkspace) error {
		var err error
		layout, err = r.layoutComposition(t.Context(), ws, plan)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return layout
}

func TestDeclaredRapidKeepsOneOwnedSentenceAndMeasuredPhraseWindows(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1" pace="rapid" styles="simple"><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></clip>`, "vertical")
	plan.Portable.Elements[0].Resolved.Text = "오늘은 철판 요리를 먹어요"
	layout := measuredDeclared(t, plan)
	elements := layout.elements()
	if len(elements) != 1 || len(elements[0].Cues) < 2 || len(layout.plan.Portable.Elements) != 1 || elements[0].Text != "오늘은 철판 요리를 먹어요" {
		t.Fatalf("lost owned sentence: %+v", elements)
	}
	if elements[0].Cues[0].Text != "오늘은" || elements[0].Cues[0].EndMS-elements[0].Cues[0].StartMS != 300 {
		t.Fatal("lost short opening beat")
	}
	for i, cue := range elements[0].Cues {
		if cue.Style != "simple" || i > 0 && cue.StartMS != elements[0].Cues[i-1].EndMS {
			t.Fatal("changed style or left a gap")
		}
	}
	repeated := measuredDeclared(t, layout.plan)
	if !reflect.DeepEqual(elements, repeated.elements()) {
		t.Fatal("preview and repeated render disagree")
	}
}

func TestDeclaredSentencesAreSequentialAndExplicitOutputIsIndependent(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1" styles="clean"><scene id="scene"><text id="first" kind="ai" role="caption" basis="cut">First scene sentence.</text><text id="second" kind="ai" role="caption" basis="cut">Second scene sentence.</text><text id="third" kind="ai" role="caption" basis="cut">Third scene sentence.</text></scene><text id="fixed" kind="fixed" role="badge" basis="whole">직접 쓴 문구</text></clip>`, "vertical")
	for i := range plan.Portable.Elements {
		if plan.Portable.Elements[i].Resolved.Element.Kind == "ai" {
			plan.Portable.Elements[i].Resolved.Text = "현재 장면"
		}
	}
	layout := measuredDeclared(t, plan)
	byID := map[string]clip.CompositionElement{}
	for _, element := range layout.elements() {
		byID[element.ElementID] = element
	}
	if len(byID) != 3 || byID["first"].EndMS != byID["second"].StartMS || byID["fixed"].StartMS != 0 || byID["fixed"].EndMS != 15000 {
		t.Fatalf("bad sequence: %+v", byID)
	}
	if len(layout.plan.Portable.Fallbacks) != 1 || layout.plan.Portable.Fallbacks[0].Reason != "sentence_count" {
		t.Fatal("missing omission reason")
	}
}

func TestDeclaredHeaderSharesOpticalHeightOnlyWhileVisible(t *testing.T) {
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		plan := declaredPlan(t, `<clip version="1"><text id="badge" kind="fixed" role="badge" basis="output-start" start="1" end="5">유료 제작 지원</text><text id="info" kind="fixed" role="info" basis="output-start" start="1" end="5"><row role="label">금액</row><row role="caption">9,900원</row></text><text id="later" kind="fixed" role="info" basis="output-start" start="6" end="10">별도 조건</text></clip>`, ratio)
		layout := measuredDeclared(t, plan)
		a, b, c := layout.visuals[0], layout.visuals[1], layout.visuals[2]
		if a.manifest.Region.Y != b.manifest.Region.Y || a.manifest.Region.Height != b.manifest.Region.Height || b.manifest.Region.X+b.manifest.Region.Width >= a.manifest.Region.X || c.manifest.Region.X != b.manifest.Region.X || c.manifest.Region.Y != b.manifest.Region.Y {
			t.Fatal("header reserves absent furniture or overlaps")
		}
		if a.furniture.Badge.Height != a.manifest.Region.Height || b.info.Plate.Height != a.manifest.Region.Height {
			t.Fatal("manifest and raster header geometry differ")
		}
	}
}

func TestAutomaticRepairUsesOnlyRetainedGroundedAlternatives(t *testing.T) {
	plan := declaredPlan(t, `<clip version="1" styles="clean"><scene id="scene"><text id="copy" kind="ai" role="caption" basis="cut">Describe the scene.</text></scene></clip>`, "vertical")
	plan.Portable.Elements[0].Resolved.Text = strings.Repeat("매우 긴 장면 설명 ", 30)
	plan.Portable.Elements[0].Alternatives = []clip.CopyAlternative{{Text: "현재 장면"}}
	layout := measuredDeclared(t, plan)
	if layout.plan.Portable.Elements[0].Resolved.Text != "현재 장면" || layout.plan.Portable.Elements[0].FallbackReason != "shorter_copy" {
		t.Fatal("approved alternative not used")
	}
	if plan.Portable.Elements[0].Resolved.Text == "현재 장면" {
		t.Fatal("mutated original plan")
	}
}

func declaredPlan(t *testing.T, body, ratio string) clip.EditPlan {
	t.Helper()
	limits := config.ClipCompositionLimits()
	doc, problem := composition.Parse(body, limits)
	if problem != nil {
		t.Fatal(problem)
	}
	section := ""
	if len(doc.Sections) > 0 {
		section = doc.Sections[0].ID
	}
	cuts := []composition.Cut{{ID: "cut", SectionID: section, SourceID: "source", StartMS: 0, EndMS: 15000}}
	resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: cuts}, limits, 30000)
	if problem != nil {
		t.Fatal(problem)
	}
	plan := clip.EditPlan{Ratio: ratio, DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}, Portable: &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: 1, Body: body}, Cuts: cuts}}
	for _, element := range resolved.Elements {
		plan.Portable.Elements = append(plan.Portable.Elements, clip.PortableText{Resolved: element, Pace: doc.Pace, Accent: doc.Accent})
	}
	return plan
}

func TestDeclaredLayoutHasNoImplicitFurniture(t *testing.T) {
	a, r := measured(t)
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		t.Run(ratio, func(t *testing.T) {
			plan := declaredPlan(t, `<clip version="1"/>`, ratio)
			plan.Disclosure, plan.Hook, plan.Preset, plan.CTA = "ad", "hidden hook", "restaurant", "profile"
			if err := a.WithWorkspace(t.Context(), "empty-native", func(ws clip.MediaWorkspace) error {
				layout, err := r.layoutComposition(t.Context(), ws, plan)
				if err == nil && len(layout.visuals) != 0 {
					t.Fatal("inserted undeclared elements")
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeclaredBadgeInfoAndCardUseOnlyAuthoredText(t *testing.T) {
	a, r := measured(t)
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		t.Run(ratio, func(t *testing.T) {
			plan := declaredPlan(t, `<clip version="1"><text id="badge" kind="fixed" role="badge" position="header" basis="whole">  제작비 일부 지원  </text><text id="info" kind="fixed" role="info" position="bottom" basis="output-start" start="0" end="4"><row role="label">포함 사항</row><row role="caption">체험권</row></text><text id="card" kind="fixed" role="hook" basis="output-start" start="2" end="5"><row role="hook">직접 쓴 제목</row><row role="body">오직 지정한 문구</row></text></clip>`, ratio)
			before := *plan.Portable
			if err := a.WithWorkspace(t.Context(), "authored-native", func(ws clip.MediaWorkspace) error {
				layout, err := r.layoutComposition(t.Context(), ws, plan)
				if err != nil {
					return err
				}
				if len(layout.visuals) != 3 || layout.visuals[0].furniture.BadgeText != "  제작비 일부 지원  " || len(layout.visuals[2].card.Lines) != 2 {
					t.Fatalf("changed authored content: %+v", layout.elements())
				}
				if !reflect.DeepEqual(before, *plan.Portable) {
					t.Fatal("layout mutated retained plan")
				}
				for i, visual := range layout.visuals {
					if visual.manifest.StartMS != plan.Portable.Elements[i].Resolved.StartMS || visual.manifest.EndMS != plan.Portable.Elements[i].Resolved.EndMS {
						t.Fatal("changed declared interval")
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeclaredCaptionKeepsExplicitStyleAndPosition(t *testing.T) {
	a, r := measured(t)
	plan := declaredPlan(t, `<clip version="1" styles="bold"><text id="one" kind="fixed" role="caption" style="bold" position="top" align="left" basis="whole">첫 문장</text><text id="two" kind="fixed" role="caption" style="bold" position="top" align="left" basis="whole">둘째 문장</text><text id="three" kind="fixed" role="caption" style="bold" position="top" align="left" basis="whole">셋째 문장</text></clip>`, "vertical")
	if err := a.WithWorkspace(t.Context(), "explicit-native", func(ws clip.MediaWorkspace) error {
		layout, err := r.layoutComposition(t.Context(), ws, plan)
		if err != nil {
			return err
		}
		if len(layout.visuals) != 3 {
			t.Fatal("frequency/overlap deleted explicit captions")
		}
		for _, visual := range layout.visuals {
			if visual.copy.Style != "bold" || visual.copy.Anchor != "top" || !visual.manifest.AuthoredPosition || !visual.manifest.AuthoredStyle {
				t.Fatal("repaired authored style or position")
			}
			if len(visual.manifest.Advisories) == 0 {
				t.Fatal("overlap was not reported for correction")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidLiteralReturnsItsIdentityWithoutTruncation(t *testing.T) {
	a, r := measured(t)
	plan := declaredPlan(t, `<clip version="1"><text id="owner" kind="fixed" role="info" basis="whole">절대로줄이거나생략하면안되는매우긴직접작성문구그대로모두남겨두고수정할수있어야합니다</text></clip>`, "vertical")
	exact := plan.Portable.Elements[0].Resolved.Text
	err := a.WithWorkspace(t.Context(), "invalid-native", func(ws clip.MediaWorkspace) error { _, err := r.layoutComposition(t.Context(), ws, plan); return err })
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.ElementID != "owner" || problem.Reason != "copy_limit" || plan.Portable.Elements[0].Resolved.Text != exact {
		t.Fatalf("literal damaged: %v", err)
	}
}
