package config

import "testing"

func TestClipCompletionBudgetsAreSeparateAndBounded(t *testing.T) {
	cfg := ClipAI(&Config{LLMReasoning: defaultLLMReasoningPolicy()})
	if cfg.ObserveCompletionTokens != 8192 || cfg.PlanCompletionTokens != 32768 || cfg.PlanCompletionTokens <= cfg.ObserveCompletionTokens || cfg.Analysis.MaxSources != 20 || cfg.Render.MaxDurationMS != 90000 || cfg.Analysis.ChunkMS != 60000 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.ObserveReasoning != defaultLLMReasoningPolicy().Observe || cfg.PlanReasoning != defaultLLMReasoningPolicy().Write {
		t.Fatal("stage policy drift")
	}
}
