package ai

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestDeclaredVideoRulesSelectOutputWithoutClassifyingOwnerOrLegacyText(t *testing.T) {
	g := clip.VideoGuidelines{Defaults: []string{"legacy words mention captions and story"}, Owner: []string{"caption-only sounding OWNER words remain everywhere"}, Stock: []clip.VideoStockRule{
		{Key: "plan", Text: "Use this at a plan stage despite saying caption", Applicability: []clip.VideoRuleApplicability{{Stage: "clip-storyline", Outputs: []string{"plan"}}}},
		{Key: "captions", Text: "Use this for captions despite saying story", Applicability: []clip.VideoRuleApplicability{{Stage: "clip-write", Outputs: []string{"captions"}}, {Stage: "clip-revise", Outputs: []string{"captions"}}}},
		{Key: "spoken", Text: "Use this for spoken lines despite saying cuts", Applicability: []clip.VideoRuleApplicability{{Stage: "clip-write", Outputs: []string{"narration"}}, {Stage: "clip-revise", Outputs: []string{"narration"}}}},
	}}
	for _, mode := range []string{"flow", "flow-follow-storyline", "flow-measured-speech", "flow-follow-storyline-measured-speech", "flow-revision", "narration", "narration-revision", "storyline", "storyline-revision", "spoken-script", "spoken-script-follow-storyline", "spoken-script-revision", "observe"} {
		selected := videoGuidelinesFor(g, mode)
		block := videoGuidelineBlock(selected)
		if strings.Contains(block, g.Defaults[0]) {
			t.Fatal("typed stock used flattened legacy fallback", mode)
		}
		if mode == "observe" {
			if block != "" {
				t.Fatal("observer gained writing rules")
			}
			continue
		}
		if !strings.Contains(block, g.Owner[0]) {
			t.Fatal("owner applicability inferred from its words", mode)
		}
		for index, rule := range g.Stock {
			want := index == 0 && strings.HasPrefix(mode, "storyline") || index == 1 && strings.HasPrefix(mode, "narration") || index == 2 && strings.HasPrefix(mode, "spoken-script")
			if strings.Contains(block, rule.Text) != want {
				t.Fatalf("incorrect declared responsibility for %s/%s", mode, rule.Key)
			}
		}
	}
	g.Stock = nil
	if !strings.Contains(videoGuidelineBlock(videoGuidelinesFor(g, "spoken-script-revision")), g.Defaults[0]) {
		t.Fatal("unknown historical stock was silently reclassified")
	}
	g.Stock = []clip.VideoStockRule{}
	g.Owner = nil
	if videoGuidelineBlock(videoGuidelinesFor(g, "spoken-script")) != "" {
		t.Fatal("no applicable rules still emitted a section")
	}
}

func TestStructuredSpokenContractRetainsDomainBoundsAndDoesNotClaimTiming(t *testing.T) {
	document := clip.NoTemplateComposition()
	in := clip.PlanningInput{Language: "en", Composition: &document}
	in.Policy.StructuredOutput = true
	system, _ := BuildSpokenScriptPrompt(in, clip.DefaultCompositionLimits())
	for _, bound := range []string{"At most 32 nonempty lines", "500 Unicode characters per line", "2000 in total", "at most 30 paragraphs", "1000 characters", "region_slots at most 30", "text and short_text at most 500 characters", "Text cannot predict the synthesized duration"} {
		if !strings.Contains(system, bound) {
			t.Fatal("provider structural projection lost domain bound", bound)
		}
	}
	if strings.Contains(system, compactContract(spokenScriptSchema)) {
		t.Fatal("duplicate closed structural contract")
	}
}
