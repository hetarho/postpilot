package main

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	clipai "github.com/postpilot/backend/internal/clip/ai"
)

func TestClipPricingCallsUseExactStageCompletionCaps(t *testing.T) {
	cfg := clipai.DefaultConfig(clip.Environment{})
	for _, n := range []int{1, 20, 49} {
		calls := clipPricingCalls("explicit/observer", "explicit/writer", n, ai.Budgets{Observe: cfg.ObserveCompletionTokens, Flow: cfg.FlowCompletionTokens, Narration: cfg.NarrationCompletionTokens})
		if len(calls) != 2 || calls[0].Ref != "explicit/observer" || calls[0].Count != n || calls[0].CompletionTokens != 8192 || calls[1].Ref != "explicit/writer" || calls[1].Count != 1 || calls[1].CompletionTokens != 32768 {
			t.Fatal(calls)
		}
	}
}
