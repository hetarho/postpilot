package main

import (
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/platform/config"
	"testing"
)

func TestClipPricingCallsUseExactStageCompletionCaps(t *testing.T) {
	cfg := config.ClipAI(&config.Config{})
	for _, n := range []int{1, 20, 49} {
		calls := clipPricingCalls("explicit/observer", "explicit/writer", n, ai.Budgets{Observe: cfg.ObserveCompletionTokens, Plan: cfg.PlanCompletionTokens})
		if len(calls) != 2 || calls[0].Ref != "explicit/observer" || calls[0].Count != n || calls[0].CompletionTokens != 8192 || calls[1].Ref != "explicit/writer" || calls[1].Count != 1 || calls[1].CompletionTokens != 32768 {
			t.Fatal(calls)
		}
	}
}
