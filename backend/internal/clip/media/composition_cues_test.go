package media

import (
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func captionPair(t *testing.T, pace string, duration int) clip.EditPlan {
	t.Helper()
	plan := declaredPlan(t, `<clip version="1" pace="`+pace+`" styles="simple"><scene id="scene"><text id="dish_intro" kind="ai" role="caption" basis="cut">Introduce the dish.</text><text id="dish_review" kind="ai" role="caption" basis="cut">Describe the dish.</text><text id="third" kind="ai" role="caption" basis="cut">One more sentence.</text></scene></clip>`, "vertical")
	plan.Cuts[0].EndMS = duration
	plan.Portable.Elements[0].Resolved.Text = "된장찌개가 나왔어요"
	plan.Portable.Elements[1].Resolved.Text = "국물이 정말 진해요"
	plan.Portable.Elements[2].Resolved.Text = "따뜻하게 먹어요"
	var err error
	plan, err = clip.ResolvePortableIntervals(plan, config.ClipCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestDeclaredRapidDishCaptionsOnThreeSecondCutNeverOverlap(t *testing.T) {
	plan := captionPair(t, "rapid", 3000)
	layout := measuredDeclared(t, plan)
	elements := layout.elements()
	if len(elements) != 2 || elements[0].ElementID != "dish_intro" || elements[1].ElementID != "dish_review" || elements[0].EndMS != elements[1].StartMS {
		t.Fatalf("overlapping or missing captions: %+v", elements)
	}
	for _, a := range elements[0].Cues {
		for _, b := range elements[1].Cues {
			if a.StartMS < b.EndMS && b.StartMS < a.EndMS {
				t.Fatalf("overlapping phrases: %+v %+v", a, b)
			}
		}
	}
	if !reflect.DeepEqual(elements, measuredDeclared(t, layout.plan).elements()) {
		t.Fatal("repeated rendering changed caption windows")
	}
	if got := layout.plan.Portable.Fallbacks; len(got) != 1 || got[0].Reason != "sentence_count" || got[0].ElementID != "third" {
		t.Fatalf("unexpected drops: %+v", got)
	}
}

func TestCaptionSchedulerPaceFloorsAndCountLimits(t *testing.T) {
	for _, tc := range []struct {
		pace     string
		duration int
		keep     int
	}{
		{"rapid", 3000, 2}, {"steady", 3000, 1}, {"rapid", 400, 1}, {"steady", 5000, 2},
	} {
		plan := captionPair(t, tc.pace, tc.duration)
		kept, drops := scheduleDeclaredCaptions(plan.Portable.Elements)
		if len(kept) != tc.keep || len(drops) != 3-tc.keep || drops[0].Reason != "sentence_count" || drops[0].ElementID != "third" {
			t.Fatalf("%+v kept=%d drops=%+v", tc, len(kept), drops)
		}
		if tc.keep == 1 && (drops[1].Reason != "readability" || drops[1].ElementID != "dish_review") {
			t.Fatalf("wrong floor reason: %+v", drops)
		}
		if tc.keep == 2 && kept[0].Resolved.EndMS != kept[1].Resolved.StartMS {
			t.Fatal("windows overlap")
		}
	}
}

func TestV18RefusesOverlappingCutCaptionsAtEveryPace(t *testing.T) {
	for _, pace := range []string{"steady", "rapid"} {
		layout := measuredDeclared(t, captionPair(t, pace, 5000))
		elements := layout.elements()
		// Widen the declared window so the existing interval check still passes;
		// only the new cross-caption check can detect this forged manifest.
		layout.plan.Portable.Elements[1].Resolved.StartMS = elements[0].StartMS
		elements[1].StartMS = elements[0].StartMS
		err := clip.VerifyCompositionManifest(layout.plan, elements, config.ClipCompositionLimits())
		var problem *composition.Problem
		if !errors.As(err, &problem) || problem.Reason != "caption_overlap" || problem.ElementID != "dish_review" {
			t.Fatalf("%s: %v", pace, err)
		}
	}
}
