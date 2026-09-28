package media

import (
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// The renderer draws exactly the project's slots in a template-free plan: each
// slot's words on its own numbered slot, a blank slot drawing nothing between
// its neighbours, the preset's geometry around them, and nothing at all for a
// region that is off (CDS-73, CDS-52 V20).
func TestProjectedRegionsRenderTheirExactSlotsOnEveryRatio(t *testing.T) {
	a, r := regionMeasured(t)
	regions := clip.ProjectRegions{
		Intro: clip.ProjectRegion{Enabled: true, Slots: []clip.RegionSlot{
			{ID: "project-intro-1", Text: "성수 로컬 가이드", OwnerFixed: true},
			{ID: "project-intro-2"},
			{ID: "project-intro-3", Text: "서울 성동구 · 저녁 영업", OwnerFixed: true},
		}},
		Outro: clip.ProjectRegion{Slots: []clip.RegionSlot{{ID: "project-outro-1", Text: "다시 만나요", OwnerFixed: true}, {ID: "project-outro-2"}}},
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		t.Run(ratio, func(t *testing.T) {
			plan := declaredPlan(t, `<clip version="1"/>`, ratio)
			plan.IntroPreset, plan.OutroPreset = "cover", "b"
			plan, _, err := clip.ProjectPlanRegions(plan, regions, plan.Design().RegionPresets())
			if err != nil {
				t.Fatal(err)
			}
			var layout declaredLayout
			if err := a.WithWorkspace(t.Context(), "projected-regions", func(ws clip.MediaWorkspace) error {
				layout, err = r.layoutComposition(t.Context(), ws, plan)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			elements := layout.elements()
			if len(elements) != 1 || elements[0].Role != "hook" || elements[0].InstanceID != "project-intro" {
				t.Fatal("the drawn elements are not the enabled region alone", elements)
			}
			words := map[int]string{}
			for _, part := range elements[0].Parts {
				if part.Kind == "copy" {
					words[part.Slot] += part.Text
				}
			}
			if len(words) != 2 || words[1] != "성수 로컬 가이드" || words[3] != "서울 성동구 · 저녁 영업" {
				t.Fatal("the slots do not draw the owner's exact words on their own slots", words)
			}
			rows := []string{"성수 로컬 가이드", "", "서울 성동구 · 저녁 영업"}
			if err := design.VerifyRegion("intro", "cover", ratio, rows, 0, len(rows), true, elements[0].Parts); err != nil {
				t.Fatal("the region left its preset's geometry", err)
			}
			if elements[0].StartMS != 0 || elements[0].EndMS != 2500 {
				t.Fatal("the intro left the output's opening", elements[0].StartMS, elements[0].EndMS)
			}
			if slices.ContainsFunc(layout.visuals, func(v declaredVisual) bool { return v.manifest.Role == "ending" }) {
				t.Fatal("a region that is off was drawn")
			}
		})
	}
}
