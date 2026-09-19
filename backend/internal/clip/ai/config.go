package ai

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// DefaultConfig is the code-owned task budget of the clip AI stages,
// independent of blog completion sizing (ARCH-21). The only deployment-owned
// half is the clip Environment the render config is built from.
//
// The reasoning strengths are code-owned for the same reason the budgets are:
// changing a stage's reasoning strength changes generation behaviour rather
// than deployment topology. A model-level registry override still wins.
func DefaultConfig(env clip.Environment) Config {
	return Config{
		Analysis: clip.DefaultAnalysisLimits(),
		Render:   clip.DefaultRenderConfig(env), Template: clip.DefaultLimits(), ObserveCompletionTokens: 8192, PlanCompletionTokens: 32768,
		// One ceiling per writing call, generous first and lowered on measured
		// usage rather than guessed down before anything has been measured.
		FlowCompletionTokens: 32768, NarrationCompletionTokens: 32768,
		MaxResponseBytes: 2 * 1024 * 1024, MaxCutIDRunes: 100, TargetToleranceMS: 1000,
		ObserveReasoning: llm.ReasoningLow, PlanReasoning: llm.ReasoningLow,
	}
}
