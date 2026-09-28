package ai

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func nativeComposition(in clip.PlanningInput) bool {
	return in.Composition != nil
}

func compositionLimits(cfg Config, in clip.PlanningInput) composition.Limits {
	return cfg.Template.Composition
}
