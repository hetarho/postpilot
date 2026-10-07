package main

import (
	"testing"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
)

func TestOriginAdmissionUsesFrozenCapsAndAddsInputAllowanceBeforeHolding(t *testing.T) {
	budget := testCompletionBudget()
	request := generation.StartRequest{ObserveModel: "fixture/observer", WriteModel: "fixture/writer", ObserveCalls: 2, CompletionTokens: 12345}
	_, legacy := generationCalls(request, budget)
	if len(legacy) != 2 || legacy[1].CompletionTokens != budget.Write(nil, false) || legacy[1].PromptTokens != 0 {
		t.Fatal("legacy admission changed its completion/input contract")
	}
	request.OriginProtocolVersion = generation.OriginProtocolVersion
	counts, current := generationCalls(request, budget)
	if len(current) != 2 || current[1].CompletionTokens != request.CompletionTokens || current[1].PromptTokens != int(usage.HoldPromptTokenBound(0))+generation.OriginPromptTokenOverhead || current[0].CompletionTokens != budget.Observation() || current[0].PromptTokens != int(usage.HoldPromptTokenBound(0))+generation.ObserveOriginPromptTokenOverhead || counts[request.ObserveModel] != 2 {
		t.Fatalf("new protocol repriced in flight or added a call: %+v %+v", counts, current)
	}
	revision := generation.StartRevisionRequest{WriteModel: request.WriteModel, ContentChars: 8000, OriginProtocolVersion: generation.OriginProtocolVersion, CompletionTokens: 16384}
	priced := revisionPricingCalls(revision, budget)
	if len(priced) != 1 || priced[0].Count != 1 || priced[0].Stage != llm.StageNameWrite || priced[0].CompletionTokens != revision.CompletionTokens || priced[0].PromptTokens != current[1].PromptTokens {
		t.Fatalf("revision hold did not freeze source/annotation overhead: %+v", priced)
	}
	// The input floor belongs to usage; overhead is added after it, including
	// calls whose explicitly declared original input allowance already exceeds it.
	priced[0].PromptTokens = 50000
	adjusted := originPricingAllowance(priced, generation.OriginProtocolVersion)
	if adjusted[0].PromptTokens != 50000+generation.OriginPromptTokenOverhead {
		t.Fatal("larger frozen original input allowance was replaced by a smaller one")
	}
	plan := generation.StartStorylineRequest{WriteModel: request.WriteModel, OriginProtocolVersion: generation.OriginProtocolVersion, CompletionTokens: 12288}
	_, planCalls := storylineCalls(plan, budget)
	planEdit := storylineRevisionPricingCalls(generation.StartStorylineRevisionRequest{WriteModel: request.WriteModel, OriginProtocolVersion: generation.OriginProtocolVersion, CompletionTokens: plan.CompletionTokens}, budget)
	for _, calls := range [][]job.PlannedCall{planCalls, planEdit} {
		if len(calls) != 1 || calls[0].Count != 1 || calls[0].CompletionTokens != plan.CompletionTokens || calls[0].PromptTokens != current[1].PromptTokens {
			t.Fatalf("plan origin admission failed to freeze its single call: %+v", calls)
		}
	}
}
