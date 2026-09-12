package media

import (
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestOverlappingPlansRemainReviewable(t *testing.T) {
	_, r := measured(t)
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, compiled := range []bool{false, true} {
			for _, collision := range []string{"chips across fade", "caption under card", "captions across fade"} {
				t.Run(ratio+"/"+map[bool]string{false: "manual", true: "compiled"}[compiled]+"/"+collision, func(t *testing.T) {
					plan := clip.EditPlan{Ratio: ratio, DurationMS: 15000, Disclosure: "ad", Accent: "coral",
						Facts: []clip.Answer{{Label: "위치", Text: "서울"}}, Styles: []string{"clean", "bold"},
						Cuts: []clip.EditCut{
							{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "오늘의 한 끼", Style: "clean", Anchor: "bottom", Align: "center"}}},
							{ID: "two", SourceID: "s", Fingerprint: "s", EndMS: 7600, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "함께한 시간", Style: "clean", Anchor: "bottom", Align: "center"}}},
						}}
					if compiled {
						plan.Decisions = []clip.Composition{{Class: "DESC"}, {Class: "DESC"}}
					}
					switch collision {
					case "chips across fade":
						for i := range plan.Cuts {
							plan.Cuts[i].Chips = []string{"위치"}
						}
					case "caption under card":
						plan.Hook = "오늘의 한 끼"
						plan.Facts = append(plan.Facts, clip.Answer{Label: "상호", Text: "오늘 식당"})
						plan.Cuts[0].Copies[0].Style, plan.Cuts[0].Copies[0].Anchor = "bold", "upper_mid"
						plan.Cuts[1].Copies[0].Anchor = "lower_mid"
					case "captions across fade":
						plan.Cuts[0].Copies[0].EndMS = 7600
						plan.Cuts[1].Copies[0].EndMS = 7480
					}
					sources := []clip.RenderSource{{ID: "s", Fingerprint: "s", Info: clip.MediaInfo{DurationMS: 7600, Width: 1920, Height: 1080}}}
					got, manifest, err := r.Layout(t.Context(), plan, sources)
					if err != nil {
						t.Fatalf("overlap blocked delivery: %v", err)
					}
					if !reflect.DeepEqual(got, plan) {
						t.Fatalf("overlap changed the owner's words or composition: %+v", got)
					}
					if err := design.VerifyApproved(manifest, ratio, plan.Styles); !errors.Is(err, design.ViolationOverlap) {
						t.Fatalf("strict diagnosis must still find the collision: %v", err)
					}
					if err := clip.VerifyLayout(ratio, plan.Styles, manifest); err != nil {
						t.Fatalf("final delivery verification refused the manifest: %v", err)
					}
				})
			}
		}
	}
}
