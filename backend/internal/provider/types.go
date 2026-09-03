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
	"strings"
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

type SelectionSlot string

const (
	SlotActive     SelectionSlot = "active"
	SlotCandidateA SelectionSlot = "candidate_a"
	SlotCandidateB SelectionSlot = "candidate_b"
)

// ParseStage accepts the stored/wire form.
func ParseStage(s string) (Stage, error) {
	for _, stage := range Stages {
		if string(stage) == s {
			return stage, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownStage, s)
}

// CatalogModel is a registry entry as ONE caller sees it: the model's own facts plus what
// it would cost that caller and whether they can afford it. Both live here rather than on
// llm.ModelInfo because they are facts about the pair, not about the model.
//
// Affordable is display only, and unlike the plan floor it replaces it is temporary — the
// same model becomes affordable again at the next renewal — which is why nothing
// downstream treats an unaffordable selection as invalidated.
type CatalogModel struct {
	Info            llm.ModelInfo
	RequiredCredits int
	Affordable      bool
}

// Selection is the acting user's choice for one stage.
type Selection struct {
	Stage Stage
	Slot  SelectionSlot
	Ref   llm.ModelRef
	// Missing: the ref is no longer registered. GetSelections sets this and clears the
	// row in the same call, so the client sees it exactly once.
	Missing   bool
	UpdatedAt time.Time
}

type ComparisonPair struct {
	Stage      Stage
	CandidateA Selection
	CandidateB Selection
}

type RecommendationSet struct {
	ID         string
	Label      string
	Selections []RecommendationStageSelection
}

type RecommendationStageSelection struct {
	Stage      Stage
	Active     llm.ModelRef
	CandidateA llm.ModelRef
	CandidateB llm.ModelRef
}

var (
	ErrUnknownStage       = errors.New("unknown stage")
	ErrModelNotRegistered = errors.New("model not registered")
	// ErrModelDisabled: the model exists but its provider has no key — it cannot be
	// selected, the same rule the dropdown enforces.
	ErrModelDisabled = errors.New("model disabled")
	// ErrModelUnsuitable: the model is not registered to this stage's purpose (change 20).
	ErrModelUnsuitable        = errors.New("model unsuitable for stage")
	ErrDuplicateCandidates    = errors.New("comparison candidates must differ")
	ErrRecommendationNotFound = errors.New("recommendation set not found")
)

// ReasonSetUnavailable is the wire reason for a recommendation set naming refs the catalog
// cannot currently serve.
const ReasonSetUnavailable = "MODEL_SET_UNAVAILABLE"

// SetRefusal names every ref of a recommendation set that blocks applying it.
//
// A set is applied whole, and the models it names are curated data that changes while the
// process runs, so a refusal that stopped at the first bad ref would make the user discover
// the rest one attempt at a time — with no way to tell "one model was retired" from "this
// set is stale".
type SetRefusal struct {
	Unregistered []string
	Disabled     []string
	Unsuitable   []string
}

func (e *SetRefusal) Error() string {
	parts := make([]string, 0, 3)
	for _, group := range []struct {
		what string
		refs []string
	}{
		{"not in the catalog", e.Unregistered},
		{"disabled", e.Disabled},
		{"unusable for their stage", e.Unsuitable},
	} {
		if len(group.refs) > 0 {
			parts = append(parts, fmt.Sprintf("%s: %s", group.what, strings.Join(group.refs, ", ")))
		}
	}
	return "recommendation set cannot be applied — " + strings.Join(parts, "; ")
}

func (e *SetRefusal) Reason() string { return ReasonSetUnavailable }

// Params carry every offending ref, grouped by cause, so one refusal explains the whole
// set. `models` is the flat list for copy that only needs to name them.
func (e *SetRefusal) Params() map[string]string {
	return map[string]string{
		"models":       strings.Join(e.All(), ", "),
		"unregistered": strings.Join(e.Unregistered, ", "),
		"disabled":     strings.Join(e.Disabled, ", "),
		"unsuitable":   strings.Join(e.Unsuitable, ", "),
	}
}

// Unwrap keeps errors.Is matching the sentinels a single-ref refusal raises, so a caller
// that already handles "model not registered" is not broken by the grouped form.
func (e *SetRefusal) Unwrap() []error {
	out := make([]error, 0, 3)
	if len(e.Unregistered) > 0 {
		out = append(out, ErrModelNotRegistered)
	}
	if len(e.Disabled) > 0 {
		out = append(out, ErrModelDisabled)
	}
	if len(e.Unsuitable) > 0 {
		out = append(out, ErrModelUnsuitable)
	}
	return out
}

// All is every offending ref in report order.
func (e *SetRefusal) All() []string {
	out := make([]string, 0, len(e.Unregistered)+len(e.Disabled)+len(e.Unsuitable))
	out = append(out, e.Unregistered...)
	out = append(out, e.Disabled...)
	out = append(out, e.Unsuitable...)
	return out
}

// Empty reports whether the set passed.
func (e *SetRefusal) Empty() bool { return len(e.All()) == 0 }

// Suitable reports whether a model can serve a stage: pure membership in the stages the
// catalog registered it for (change 20). Capability fitness — observe needing vision — is
// enforced upstream at registration, so no flag is re-derived here.
func Suitable(stage Stage, info llm.ModelInfo) bool {
	return info.ServesStage(string(stage))
}
