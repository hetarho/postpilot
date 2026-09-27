package media

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// The example plan the local harness documents is one it can write: its outline
// parses and resolves over its cuts, and its regions are the outline's own.
func TestLocalPlanExampleResolves(t *testing.T) {
	body, err := os.ReadFile("testdata/local-plan.example.json")
	if err != nil {
		t.Fatal(err)
	}
	var lp localPlan
	if err := json.Unmarshal(body, &lp); err != nil {
		t.Fatal(err)
	}
	plan := clip.EditPlan{Ratio: lp.Ratio, DurationMS: lp.DurationMS}
	for i, c := range lp.Cuts {
		cut := clip.EditCut{ID: fmt.Sprintf("cut-%02d", i), SourceID: "source", Fingerprint: "source", StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS}
		for _, cp := range c.Copies {
			cut.Copies = append(cut.Copies, clip.Copy{Text: cp.Text, Style: cp.Style, Anchor: cp.Anchor, Align: cp.Align, Keyword: cp.Keyword})
		}
		plan.Cuts = append(plan.Cuts, cut)
	}
	portable, err := localPortable(lp, plan, clip.DefaultCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]int{}
	for _, e := range portable.Elements {
		roles[e.Resolved.Element.Role]++
	}
	if roles["hook"] != 1 || roles["ending"] != 1 || roles["caption"] == 0 {
		t.Fatalf("example regions and captions: %v", roles)
	}
}
