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
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
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

// HasPair reports whether a stage keeps an A/B comparison pair. Analyze keeps its active
// selection alone: the model lab compares observe and write only (MODEL-23, MODEL-30).
func HasPair(stage Stage) bool { return stage == StageObserve || stage == StageWrite }

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
	Info             llm.ModelInfo
	RequiredCredits  int
	Affordable       bool
	PriceUnavailable bool
	Access           []StageAccess
}

type StageAccess struct {
	Stage             Stage
	Grade             string
	RequiredPlan      plan.Plan
	Entitled          bool
	FreePathAvailable bool
	UnavailableReason string
}

// Selection is the acting user's choice for one stage.
type Selection struct {
	Stage Stage
	Slot  SelectionSlot
	Ref   llm.ModelRef
	// Missing: the ref is no longer registered. GetSelections sets this and clears the
	// row in the same call, so the client sees it exactly once.
	Missing           bool
	RequiredPlan      plan.Plan
	UnavailableReason string
	UpdatedAt         time.Time
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

// RecommendationSlot is one of the seven places a set names a model.
type RecommendationSlot struct {
	Slot SelectionSlot
	Ref  llm.ModelRef
}

// Slots lists the stage's slots in order: the active model, then the A/B pair on a stage that
// keeps one. Analyze's candidates are never part of a set (MODEL-23).
func (s RecommendationStageSelection) Slots() []RecommendationSlot {
	slots := []RecommendationSlot{{SlotActive, s.Active}}
	if HasPair(s.Stage) {
		slots = append(slots, RecommendationSlot{SlotCandidateA, s.CandidateA}, RecommendationSlot{SlotCandidateB, s.CandidateB})
	}
	return slots
}

var (
	ErrUnknownStage       = errors.New("unknown stage")
	ErrModelNotRegistered = errors.New("model not registered")
	// ErrModelDisabled: the model exists but its provider has no key — it cannot be
	// selected, the same rule the dropdown enforces.
	ErrModelDisabled = errors.New("model disabled")
	// ErrModelUnsuitable: the model is not registered to this stage's purpose (MODEL-25).
	ErrModelUnsuitable     = errors.New("model unsuitable for stage")
	ErrModelPlanRequired   = errors.New("model requires a higher plan")
	ErrModelUnclassified   = errors.New("model has no classification")
	ErrFreePathUnavailable = errors.New("free model has no verified zero-cost path")
	ErrDuplicateCandidates = errors.New("comparison candidates must differ")
	// ErrStageWithoutPair refuses a comparison pair for a stage that keeps none (HasPair).
	ErrStageWithoutPair       = errors.New("stage keeps no comparison pair")
	ErrRecommendationNotFound = errors.New("recommendation set not found")
	// ErrRecommendationLimit refuses a new set once MaxRecommendationSets exist (MODEL-69).
	ErrRecommendationLimit = errors.New("recommendation set limit reached")
)

const (
	ReasonModelPlanRequired = "MODEL_PLAN_REQUIRED"
	ReasonModelUnclassified = "MODEL_UNCLASSIFIED"
)

type ModelAccessError struct {
	Ref      llm.ModelRef
	Stage    Stage
	Grade    string
	Required plan.Plan
	Cause    error
}

func (e *ModelAccessError) Error() string {
	return fmt.Sprintf("%s: %s at %s", e.Cause, e.Ref, e.Stage)
}
func (e *ModelAccessError) Unwrap() error { return e.Cause }
func (e *ModelAccessError) Reason() string {
	switch e.Cause {
	case ErrModelPlanRequired:
		return ReasonModelPlanRequired
	case ErrFreePathUnavailable:
		return "MODEL_FREE_PATH_UNAVAILABLE"
	default:
		return ReasonModelUnclassified
	}
}
func (e *ModelAccessError) Params() map[string]string {
	return map[string]string{"model": e.Ref.String(), "stage": string(e.Stage), "grade": e.Grade, "required_plan": string(e.Required)}
}

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
	PlanLocked   []string
	Unclassified []string
	FreePath     []string
}

func (e *SetRefusal) Error() string {
	parts := make([]string, 0, 6)
	for _, group := range []struct {
		what string
		refs []string
	}{
		{"not in the catalog", e.Unregistered},
		{"disabled", e.Disabled},
		{"unusable for their stage", e.Unsuitable},
		{"requires a higher plan", e.PlanLocked},
		{"unclassified", e.Unclassified},
		{"free path unavailable", e.FreePath},
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
		"plan_locked":  strings.Join(e.PlanLocked, ", "),
		"unclassified": strings.Join(e.Unclassified, ", "),
		"free_path":    strings.Join(e.FreePath, ", "),
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
	if len(e.PlanLocked) > 0 {
		out = append(out, ErrModelPlanRequired)
	}
	if len(e.Unclassified) > 0 {
		out = append(out, ErrModelUnclassified)
	}
	if len(e.FreePath) > 0 {
		out = append(out, ErrFreePathUnavailable)
	}
	return out
}

// All is every offending ref in report order.
func (e *SetRefusal) All() []string {
	out := make([]string, 0, len(e.Unregistered)+len(e.Disabled)+len(e.Unsuitable)+len(e.PlanLocked)+len(e.Unclassified)+len(e.FreePath))
	out = append(out, e.Unregistered...)
	out = append(out, e.Disabled...)
	out = append(out, e.Unsuitable...)
	out = append(out, e.PlanLocked...)
	out = append(out, e.Unclassified...)
	out = append(out, e.FreePath...)
	return out
}

// Empty reports whether the set passed.
func (e *SetRefusal) Empty() bool { return len(e.All()) == 0 }

// The operator's bounds on recommendation sets (MODEL-69).
const (
	MaxRecommendationSets       = 10
	MaxRecommendationLabelRunes = 60
)

// Wire reasons for an operator's recommendation-set write.
const (
	ReasonSetInvalid = "MODEL_SET_INVALID"
	ReasonSetLimit   = "MODEL_SET_LIMIT"
)

// Why one field of a draft set is refused (MODEL-70). The values are wire params the editor
// renders beside the field, so they are part of the contract with the browser.
const (
	DraftRequired     = "required"
	DraftTooLong      = "too_long"
	DraftDuplicate    = "duplicate"
	DraftUnregistered = "unregistered"
	DraftUnclassified = "unclassified"
)

// DraftLabelField is the label's field key; a slot's is DraftSlotField.
const DraftLabelField = "label"

// DraftSlotField names one of the seven slots on the wire: `<stage>_<slot>`.
func DraftSlotField(stage Stage, slot SelectionSlot) string {
	return string(stage) + "_" + string(slot)
}

// SetDraftRefusal names every field of a draft set that blocks saving it, each with its cause.
// A set is saved whole, so one refusal reports the whole draft rather than its first problem.
type SetDraftRefusal struct {
	Fields map[string]string
}

func (e *SetDraftRefusal) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, field := range e.fieldNames() {
		parts = append(parts, field+": "+e.Fields[field])
	}
	return "recommendation set draft refused — " + strings.Join(parts, ", ")
}

func (e *SetDraftRefusal) Reason() string { return ReasonSetInvalid }

// Params carry each offending field's cause under its own key, and `fields` lists the keys so
// a reader does not have to know the vocabulary to find them.
func (e *SetDraftRefusal) Params() map[string]string {
	params := maps.Clone(e.Fields)
	params["fields"] = strings.Join(e.fieldNames(), ",")
	return params
}

func (e *SetDraftRefusal) fieldNames() []string {
	return slices.Sorted(maps.Keys(e.Fields))
}

// SetLimitError refuses an operator's new set once the installation holds the most it offers.
type SetLimitError struct{ Limit int }

func (e *SetLimitError) Error() string {
	return fmt.Sprintf("at most %d recommendation sets", e.Limit)
}
func (e *SetLimitError) Reason() string { return ReasonSetLimit }
func (e *SetLimitError) Params() map[string]string {
	return map[string]string{"limit": strconv.Itoa(e.Limit)}
}
func (e *SetLimitError) Unwrap() error { return ErrRecommendationLimit }

// ValidateRecommendationDraft checks a draft set against the catalog as it is now (MODEL-70):
// a trimmed label of 1–MaxRecommendationLabelRunes runes, every one of the seven slots filled,
// distinct observe and write candidates, and every ref registered to its stage's purpose with
// a classification. It never asks about a plan or a balance — which tiers can apply the set is
// settled per account at apply time (MODEL-26). A nil return means the draft may be saved.
//
// The label is returned trimmed so the caller stores what was checked.
func ValidateRecommendationDraft(draft RecommendationSet, lookup func(llm.ModelRef) (llm.ModelInfo, bool)) (RecommendationSet, *SetDraftRefusal) {
	fields := map[string]string{}
	draft.Label = strings.TrimSpace(draft.Label)
	switch count := utf8.RuneCountInString(draft.Label); {
	case count == 0:
		fields[DraftLabelField] = DraftRequired
	case count > MaxRecommendationLabelRunes:
		fields[DraftLabelField] = DraftTooLong
	}
	byStage := map[Stage]RecommendationStageSelection{}
	for _, selection := range draft.Selections {
		byStage[selection.Stage] = selection
	}
	normalized := make([]RecommendationStageSelection, 0, len(Stages))
	for _, stage := range []Stage{StageObserve, StageAnalyze, StageWrite} {
		selection := byStage[stage]
		selection.Stage = stage
		if !HasPair(stage) {
			selection.CandidateA, selection.CandidateB = llm.ModelRef{}, llm.ModelRef{}
		}
		for _, entry := range selection.Slots() {
			field := DraftSlotField(stage, entry.Slot)
			if entry.Ref.ProviderID == "" || entry.Ref.ModelID == "" {
				fields[field] = DraftRequired
				continue
			}
			if entry.Slot == SlotCandidateB && entry.Ref == selection.CandidateA {
				fields[field] = DraftDuplicate
				continue
			}
			info, found := lookup(entry.Ref)
			switch {
			case !found || !Suitable(stage, info):
				fields[field] = DraftUnregistered
			case info.Levels[string(stage)] == "":
				fields[field] = DraftUnclassified
			}
		}
		normalized = append(normalized, selection)
	}
	draft.Selections = normalized
	if len(fields) > 0 {
		return RecommendationSet{}, &SetDraftRefusal{Fields: fields}
	}
	return draft, nil
}

// Suitable reports whether a model can serve a stage: pure membership in the stages the
// catalog registered it for (MODEL-14). Capability fitness — observe needing vision — is
// enforced upstream at registration, so no flag is re-derived here.
func Suitable(stage Stage, info llm.ModelInfo) bool {
	return info.ServesStage(string(stage))
}
