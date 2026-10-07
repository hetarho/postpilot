package main

import (
	"github.com/postpilot/backend/internal/guideline"
	"testing"
)

func TestClipStockMapperPreservesDeclaredStageOutputsAndOwnedOrder(t *testing.T) {
	source := []guideline.StockRule{{Key: "source-facts", Text: "verbatim\nstock text", SourceOrder: 7, Applicability: []guideline.RuleApplicability{{Stage: guideline.StageClipWrite, Outputs: []guideline.RuleOutput{guideline.OutputCaptions, guideline.OutputNarration}}, {Stage: guideline.StageClipStoryline, Outputs: []guideline.RuleOutput{guideline.OutputPlan}}}}}
	result := clipStockRules(source)
	if len(result) != 1 || result[0].Key != source[0].Key || result[0].Text != source[0].Text || result[0].SourceOrder != 7 || result[0].Applicability[0].Stage != "clip-write" || result[0].Applicability[0].Outputs[0] != "captions" || result[0].Applicability[1].Stage != "clip-storyline" {
		t.Fatal("clip rule identity or scope changed", result)
	}
	result[0].Applicability[0].Outputs[0] = "modified"
	if source[0].Applicability[0].Outputs[0] != guideline.OutputCaptions {
		t.Fatal("mapper shares mutable rule output slice")
	}
	if clipStockRules(nil) != nil || clipStockRules([]guideline.StockRule{}) == nil {
		t.Fatal("retained unknown metadata and current empty registry collapsed")
	}
}
