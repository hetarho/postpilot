package rpc

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
)

// QUOTA-64: a stage's per-post figure crosses as credits and a basis, keyed by the wire stage.
func TestModelInfoCarriesStagePostCredits(t *testing.T) {
	wire := toProtoModel(provider.CatalogModel{
		Info: llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "openrouter", ModelID: "vendor/pen"}, Stages: []string{"observe", "write"}},
		PostCredits: []provider.StagePostCredits{
			{Stage: provider.StageObserve, Figure: plan.PostFigure{Credits: 31, Basis: plan.PostCreditsEstimate}},
			{Stage: provider.StageWrite, Figure: plan.PostFigure{Credits: 12, Basis: plan.PostCreditsRecentUsage}},
		},
	})
	got := wire.GetPostCredits()
	if len(got) != 2 {
		t.Fatalf("post credits = %+v", got)
	}
	if got[0].GetStage() != postpilotv1.Stage_STAGE_OBSERVE || got[0].GetCredits() != 31 ||
		got[0].GetBasis() != postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_ESTIMATE {
		t.Fatalf("observe = %+v", got[0])
	}
	if got[1].GetStage() != postpilotv1.Stage_STAGE_WRITE || got[1].GetCredits() != 12 ||
		got[1].GetBasis() != postpilotv1.PostCreditsBasis_POST_CREDITS_BASIS_RECENT_USAGE {
		t.Fatalf("write = %+v", got[1])
	}
}
