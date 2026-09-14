package media

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// Captured before caption-safe ranking: compare the complete manifest bytes,
// including resolved anchors, glyph bounds, motion and phrase windows.
func TestAutomaticCaptionLayoutWithoutRegionsGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/automatic-anchor-defaults.json")
	var golden map[string]string
	if err != nil || json.Unmarshal(data, &golden) != nil || len(golden) != 30 {
		t.Fatal("invalid golden fixture", err)
	}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		for _, pace := range []string{"steady", "rapid"} {
			for _, style := range []string{"simple", "clean", "memo", "bold", "mark"} {
				key := ratio + "/" + pace + "/" + style
				plan := declaredPlan(t, `<clip version="1" pace="`+pace+`" styles="`+style+`"><scene id="scene"><text id="caption" kind="ai" role="caption" basis="cut">Describe.</text></scene></clip>`, ratio)
				plan.Portable.Elements[0].Resolved.Text = "현재 장면"
				plan.Portable.Elements[0].Keyword = "장면"
				plan.Portable.Observations = []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Info: clip.MediaInfo{Width: 1920, Height: 1080}}}, Segments: []clip.Segment{{EndMS: 15000, Scene: "food"}}}}
				actual, err := json.Marshal(measuredDeclared(t, plan).elements())
				if err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(actual)); got != golden[key] {
					t.Fatalf("%s manifest changed without regions: %s", key, got)
				}
			}
		}
	}
}

func TestAutomaticCaptionUsesItsScenesSpaceAndPinnedPositionsStay(t *testing.T) {
	for _, pace := range []string{"steady", "rapid"} {
		for _, mode := range []string{"automatic", "bottom subject", "template", "owner", "other source", "other time"} {
			t.Run(pace+"/"+mode, func(t *testing.T) {
				position := "auto"
				if mode == "template" {
					position = "bottom"
				}
				plan := declaredPlan(t, `<clip version="1" pace="`+pace+`" styles="simple"><scene id="scene"><text id="caption" kind="ai" role="caption" position="`+position+`" basis="cut">Describe.</text></scene></clip>`, "vertical")
				plan.Portable.Elements[0].Resolved.Text = "현재 장면"
				source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Info: clip.MediaInfo{Width: 1080, Height: 1920}}}
				segment := clip.Segment{EndMS: 15000, Scene: "food", CaptionSafe: []clip.Region{{X: 0, Y: 0, Width: 1, Height: .3}}}
				if mode == "bottom subject" {
					segment.Subject = clip.Region{X: .3, Y: .65, Width: .4, Height: .1}
				}
				if mode == "other source" {
					source.ID = "other"
				}
				if mode == "other time" {
					segment.StartMS = 15000
					segment.EndMS = 30000
				}
				if mode == "owner" {
					plan.Portable.Elements[0].Placement = &clip.CompositionPlacement{Style: "simple", Position: "bottom", StartMS: 120, EndMS: 14880}
				}
				plan.Portable.Observations = []clip.SourceAnalysis{{Source: source, Segments: []clip.Segment{segment}}}
				layout := measuredDeclared(t, plan)
				want := "bottom"
				if mode == "automatic" || mode == "bottom subject" {
					want = "top"
				}
				elements := layout.elements()
				if len(elements) != 1 || elements[0].Position != want {
					t.Fatalf("%s position: %+v", mode, elements)
				}
				for _, cue := range elements[0].Cues {
					if cue.Position != want {
						t.Fatal("phrase wandered away from chosen anchor")
					}
				}
				before, _ := json.Marshal(elements)
				after, _ := json.Marshal(measuredDeclared(t, layout.plan).elements())
				if string(before) != string(after) {
					t.Fatal("repeated render re-ranked stored placement")
				}
			})
		}
	}
}
