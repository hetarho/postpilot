package main

import (
	"testing"

	"github.com/postpilot/backend/internal/platform/config"
)

// MODEL-100: a reasoning observer's calls are priced and held at the observation headroom, the
// same budget its requests are sent with; a non-reasoning observer keeps the batch budget.
func TestObservePricingUsesTheReasoningObserverBudget(t *testing.T) {
	budget := config.LLMCompletionBudget{Observe: 2048, WriteFloor: 8192, WritePerChar: 4, Ceiling: 65536}
	_, plain := observeWriteCalls("stub/observer", "", 2, false, 0, budget)
	_, reasoning := observeWriteCalls("stub/observer", "", 2, true, 0, budget)
	if len(plain) != 1 || plain[0].CompletionTokens != budget.Observation(false) {
		t.Fatalf("plain observe pricing = %+v, want %d", plain, budget.Observation(false))
	}
	if len(reasoning) != 1 || reasoning[0].CompletionTokens != budget.Observation(true) || budget.Observation(true) <= budget.Observation(false) {
		t.Fatalf("reasoning observe pricing = %+v, want %d", reasoning, budget.Observation(true))
	}
}
