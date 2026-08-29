package rpc

import (
	"testing"

	"github.com/postpilot/backend/internal/experiment"
)

func TestCandidateMappingIsBlindUntilVerdict(t *testing.T) {
	candidate := experiment.Candidate{
		ID: "opaque", Model: experiment.ModelRef{ProviderID: "secret-provider", ModelID: "secret-model"},
		ModelLabel: "Secret label", DisplaySide: experiment.SideLeft, Status: experiment.CandidateFailed,
		Output: []byte(`"style guide"`), Error: "upstream secret error",
		Usage: experiment.Usage{PromptTokens: 12, CompletionTokens: 3, CostMicrousd: 8, CostSource: experiment.CostReported, LatencyMS: 99},
	}
	blind := toProtoCandidate(experiment.Experiment{Stage: experiment.StageAnalyze, Status: experiment.StatusPartial}, candidate)
	if blind.GetModel() != nil || blind.GetModelLabel() != "" || blind.GetUsage() != nil || blind.GetError() != "" {
		t.Fatalf("pre-verdict response leaked identity/accounting: %+v", blind)
	}
	if blind.GetStyleguide() != "style guide" || blind.GetId() != "opaque" {
		t.Fatalf("blind output/id missing: %+v", blind)
	}

	revealed := toProtoCandidate(experiment.Experiment{Stage: experiment.StageAnalyze, Status: experiment.StatusDismissed}, candidate)
	if revealed.GetModel().GetModelId() != "secret-model" || revealed.GetModelLabel() != "Secret label" || revealed.GetUsage().GetCostMicrousd() != 8 || revealed.GetError() == "" {
		t.Fatalf("terminal response did not reveal snapshot: %+v", revealed)
	}
}
