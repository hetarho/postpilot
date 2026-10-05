package media

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestV18RefusesOverlappingCaptionsAtEveryPace(t *testing.T) {
	for _, pace := range []string{"steady", "rapid"} {
		plan := narrationPlan(t,
			narrationText("narration-1", "고기를 올렸어요", 1000, 5000),
			narrationText("narration-2", "국물도 나왔어요", 6000, 10000),
		)
		plan.CaptionPace = pace
		layout := measuredDeclared(t, plan)
		elements := layout.elements()
		second := -1
		for i, element := range elements {
			if element.ElementID == "narration-2" {
				second = i
			}
		}
		if second < 0 {
			t.Fatalf("%s: the second caption was not laid out: %+v", pace, elements)
		}
		// Forge the second caption to start under the first, in the plan and the
		// manifest alike, so every per-element check still passes and only the
		// cross-caption check can see it.
		start := 2000
		for i := range layout.plan.Portable.Elements {
			if text := &layout.plan.Portable.Elements[i].Resolved; text.Element.ID == "narration-2" {
				text.StartMS, text.Element.StartMS = start, &start
			}
		}
		elements[second].StartMS = start
		err := clip.VerifyCompositionManifest(layout.plan, elements, clip.DefaultCompositionLimits())
		var problem *composition.Problem
		if !errors.As(err, &problem) || problem.Reason != "caption_overlap" || problem.ElementID != "narration-2" {
			t.Fatalf("%s: %v", pace, err)
		}
	}
}
