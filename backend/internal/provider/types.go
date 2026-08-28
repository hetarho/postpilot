// Package provider is the model-catalog context: what the registry offers, and the
// acting user's last choice per stage (PRD F-4). It owns model_selections.
//
// It does not run models. The generation and analysis contexts take a ModelRef in
// their own requests and call the llm port themselves; this context only remembers what
// to preselect in a dropdown.
package provider

import (
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Stage is one of the three places a model is chosen ([I3]).
type Stage string

const (
	StageObserve Stage = "observe"
	StageWrite   Stage = "write"
	StageAnalyze Stage = "analyze"
)

// Stages in display order.
var Stages = []Stage{StageObserve, StageWrite, StageAnalyze}

// ParseStage accepts the stored/wire form.
func ParseStage(s string) (Stage, error) {
	for _, stage := range Stages {
		if string(stage) == s {
			return stage, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownStage, s)
}

// Selection is the acting user's choice for one stage.
type Selection struct {
	Stage Stage
	Ref   llm.ModelRef
	// Missing: the ref is no longer registered. GetSelections sets this and clears the
	// row in the same call, so the client sees it exactly once.
	Missing   bool
	UpdatedAt time.Time
}

var (
	ErrUnknownStage       = errors.New("unknown stage")
	ErrModelNotRegistered = errors.New("model not registered")
	// ErrModelDisabled: the model exists but its provider has no key — it cannot be
	// selected, the same rule the dropdown enforces.
	ErrModelDisabled = errors.New("model disabled")
	// ErrModelUnsuitable: the model lacks what the stage needs (observe needs vision).
	ErrModelUnsuitable = errors.New("model unsuitable for stage")
)

// Suitable reports whether a model can serve a stage: observation looks at photos, so
// it needs a vision model (PRD §6.4); the other stages take any model.
func Suitable(stage Stage, info llm.ModelInfo) bool {
	return stage != StageObserve || info.Vision
}
