package ai

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestBackwardRepairPreservesCopySourceTimeAndScene(t *testing.T) {
	for _, boundary := range []string{"none", "scene", "gap", "neighbor"} {
		t.Run(boundary, func(t *testing.T) {
			cfg := Config{Render: clip.RenderConfig{MinDurationMS: 15000, MaxDurationMS: 90000, MaxCuts: 100, FadeMS: 200}}
			in := clip.PlanningInput{Ratio: "vertical", TargetDurationMS: 15000}
			plan := clip.EditPlan{Ratio: "vertical"}
			for _, id := range []string{"a", "b"} {
				in.Analyses = append(in.Analyses, clip.SourceAnalysis{
					Source:   clip.AnalysisSource{RenderSource: clip.RenderSource{ID: id, Fingerprint: id, Info: clip.MediaInfo{DurationMS: 8000}}},
					Segments: []clip.Segment{{StartMS: 0, EndMS: 8000, Scene: "scenery"}},
				})
				plan.Cuts = append(plan.Cuts, clip.Cut{ID: id, SourceID: id, Fingerprint: id, StartMS: 2000, EndMS: 8000, Copies: []clip.Copy{{StartMS: 200, EndMS: 1000}}})
			}
			switch boundary {
			case "scene":
				for i := range in.Analyses {
					in.Analyses[i].Segments = []clip.Segment{{StartMS: 0, EndMS: 2000, Scene: "food"}, {StartMS: 2000, EndMS: 8000, Scene: "scenery"}}
				}
			case "gap":
				for i := range in.Analyses {
					in.Analyses[i].Segments = []clip.Segment{{StartMS: 0, EndMS: 1000, Scene: "scenery"}, {StartMS: 2000, EndMS: 8000, Scene: "scenery"}}
				}
			case "neighbor":
				// The second cut takes the footage immediately after the first,
				// off the same source: adjacent, not overlapping, so the first
				// has nowhere to grow forward and nothing to borrow.
				plan.Cuts[1].SourceID, plan.Cuts[1].Fingerprint = "a", "a"
				plan.Cuts[1].StartMS, plan.Cuts[1].EndMS = 0, 2000
				plan.Cuts[1].Copies = []clip.Copy{{StartMS: 200, EndMS: 1000}}
			}
			err := composeTimeline(cfg, in, &plan)
			if boundary == "none" {
				if err != nil || plan.DurationMS != 15000 {
					t.Fatal("reachable backward repair failed", err, plan.DurationMS)
				}
				for i, c := range plan.Cuts {
					if c.StartMS+c.Copies[0].StartMS != 2200 || c.StartMS+c.Copies[0].EndMS != 3000 {
						t.Fatal("copy moved relative to its source footage", c)
					}
					if scene, _ := clip.CutScene(c, in.Analyses[i]); scene != "scenery" {
						t.Fatal("opening scene changed", scene)
					}
				}
			} else {
				// The neighbour case hands four seconds of the second cut back
				// to the first's own source, so its shortfall is larger; what
				// every case shares is that nothing was taken backward.
				want := 3000
				if boundary == "neighbor" {
					want = 7000
				}
				d, ok := clip.DiagnosticFromError(err)
				if !ok || d.Phase != "timeline_grow" || d.Values["remaining_ms"] != want || d.Values["backward_ms"] != 0 {
					t.Fatalf("unsafe expansion across %s: %v %+v", boundary, err, d)
				}
			}
		})
	}
}

func TestFinalArithmeticFailureReportsBothTotals(t *testing.T) {
	plan := clip.EditPlan{DurationMS: 15000, Cuts: []clip.Cut{{StartMS: 0, EndMS: 8000}, {StartMS: 0, EndMS: 8000, TransitionMS: 200}}}
	err := planFailure(outputError("plan_timeline"), clip.PlanningInput{TargetDurationMS: 15000}, plan, "validation", 0)
	d, ok := clip.DiagnosticFromError(err)
	if !ok || d.Phase != "timeline_total" || d.Values["before_ms"] != 15800 || d.Values["after_ms"] != 15000 || d.Values["transition_ms"] != 200 {
		t.Fatal("arithmetic failure cannot be distinguished from insufficient footage", d)
	}
}
