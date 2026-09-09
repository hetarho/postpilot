package config

import (
	"github.com/postpilot/backend/internal/clip"
	clipai "github.com/postpilot/backend/internal/clip/ai"
)

// Code-owned task budgets, independent of blog completion sizing. At most 60
// segments per 60-second chunk and 100 cuts / 90 seconds bound the output shape.
func ClipAI(cfg *Config) clipai.Config {
	return clipai.Config{
		Analysis: clip.AnalysisLimits{ChunkMS: 60000, MaxSources: ClipSourceCount, MaxSourceDurationMS: ClipSourceDurationMS, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20},
		Render:   ClipRender(cfg), Template: ClipLimits(), ObserveCompletionTokens: 8192, PlanCompletionTokens: 32768,
		MaxResponseBytes: 2 * 1024 * 1024, MaxCutIDRunes: 100, TargetToleranceMS: 1000,
		ObserveReasoning: cfg.LLMReasoning.Observe, PlanReasoning: cfg.LLMReasoning.Write,
	}
}
