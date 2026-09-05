package provider

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Service is the catalog's use-cases.
type Service struct {
	store   Store
	catalog Catalog
	credits Credits
	now     func() time.Time
}

// NewService wires the context.
func NewService(store Store, catalog Catalog, credits Credits) *Service {
	return &Service{store: store, catalog: catalog, credits: credits, now: time.Now}
}

// ListModels is the registry snapshot for one caller: the registry's own flags plus the
// two that depend on who is asking. The registry itself stays user-ignorant, so the
// pricing happens here, where the account is known.
//
// A model is priced at one call of it. That is the floor of what any job using it can
// hold, so a model the caller cannot afford at one call they certainly cannot afford at
// the several a real job makes.
func (s *Service) ListModels(ctx context.Context, userID string) ([]CatalogModel, error) {
	balance, unlimited, err := s.credits.Balance(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read balance: %w", err)
	}
	models := s.catalog.Models()
	out := make([]CatalogModel, 0, len(models))
	for _, info := range models {
		// A model registered only to a purpose no stage consumes yet (image/video
		// generation) is an operator setting, not a user-facing catalog entry — it never
		// crosses this wire.
		if len(info.Stages) == 0 {
			continue
		}
		// The catalog has no stage input, so its quote explicitly names the first stage the
		// registry publishes for this model instead of silently falling back to one process-wide
		// completion cap. Stage-specific pickers still filter this same entry by membership.
		quotedStage, err := ParseStage(info.Stages[0])
		if err != nil {
			continue
		}
		required := s.credits.ForCalls([]PlannedCall{{Ref: info.Ref, Count: 1, Stage: quotedStage, NativeEffort: info.ReasoningNativeEffort}})
		out = append(out, CatalogModel{
			Info: info, RequiredCredits: required,
			Affordable: unlimited || balance >= required,
		})
	}
	return out, nil
}

// EstimatePostCredits is what one generated post would hold with the given stage pair.
//
// It is the number a "your balance covers about N posts" estimate divides into, and it is
// computed here rather than in the browser because the charge formula — its per-request
// base especially — is a server-owned product rule the client must never re-implement.
func (s *Service) EstimatePostCredits(observe, write llm.ModelRef) int {
	calls := make([]PlannedCall, 0, 2)
	if observe != (llm.ModelRef{}) {
		observeInfo, _ := s.catalog.Lookup(observe)
		calls = append(calls, PlannedCall{Ref: observe, Count: 1, Stage: StageObserve, NativeEffort: observeInfo.ReasoningNativeEffort})
	}
	if write != (llm.ModelRef{}) {
		writeInfo, _ := s.catalog.Lookup(write)
		calls = append(calls, PlannedCall{Ref: write, Count: 1, Stage: StageWrite, NativeEffort: writeInfo.ReasoningNativeEffort})
	}
	if len(calls) == 0 {
		return 0
	}
	return s.credits.ForCalls(calls)
}

// GetSelections returns the user's per-stage choices. A choice whose model is no longer
// registered is reported `Missing` and cleared here (PRD §7: 마지막 선택 초기화), so the
// user sees the greyed entry once and then must choose again.
//
// A choice the caller cannot currently AFFORD is not reported here at all: a balance is
// temporary state that the next renewal clears, so it invalidates nothing. Only a model
// that has actually vanished or become unsuitable is cleared.
func (s *Service) GetSelections(ctx context.Context, userID string) ([]Selection, error) {
	selections, err := s.store.ListSelections(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list selections: %w", err)
	}
	for i := range selections {
		if selections[i].Slot == "" {
			selections[i].Slot = SlotActive
		}
		info, ok := s.catalog.Lookup(selections[i].Ref)
		// A model deregistered from this stage's purpose is as gone as one deleted: the
		// dropdown no longer lists it, so the choice is cleared the same way. This is also
		// the machinery that absorbs change 20's empty cutover — every pre-cutover
		// selection lands here on its next read, with no bespoke migration clearing.
		if ok && Suitable(selections[i].Stage, info) {
			continue
		}
		selections[i].Missing = true
		// Best effort: a failed clear only means the user is told again next time.
		if err := s.store.DeleteSelection(ctx, userID, selections[i]); err != nil {
			slog.Warn("clear vanished selection failed", "user", userID, "stage", selections[i].Stage, "err", err)
		}
	}
	return selections, nil
}

func (s *Service) GetComparisonPairs(ctx context.Context, userID string) ([]ComparisonPair, error) {
	selections, err := s.store.ListSelectionSlots(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list comparison pairs: %w", err)
	}
	byStage := map[Stage]*ComparisonPair{}
	for _, selection := range selections {
		if selection.Slot == SlotActive {
			continue
		}
		info, ok := s.catalog.Lookup(selection.Ref)
		switch {
		case !ok || info.Disabled || !Suitable(selection.Stage, info):
			selection.Missing = true
			if err := s.store.DeleteSelection(ctx, userID, selection); err != nil {
				slog.Warn("clear vanished comparison selection failed", "user", userID, "stage", selection.Stage, "slot", selection.Slot, "err", err)
			}
		}
		pair := byStage[selection.Stage]
		if pair == nil {
			pair = &ComparisonPair{Stage: selection.Stage}
			byStage[selection.Stage] = pair
		}
		if selection.Slot == SlotCandidateA {
			pair.CandidateA = selection
		} else if selection.Slot == SlotCandidateB {
			pair.CandidateB = selection
		}
	}
	out := make([]ComparisonPair, 0, len(Stages))
	for _, stage := range Stages {
		if pair := byStage[stage]; pair != nil {
			out = append(out, *pair)
		}
	}
	return out, nil
}

// SaveSelection records a choice. Only a registered, enabled model can be chosen — the
// same rule the dropdown shows, enforced where it can be trusted.
func (s *Service) SaveSelection(ctx context.Context, userID string, stage Stage, ref llm.ModelRef) (Selection, error) {
	if err := s.validateRef(stage, ref); err != nil {
		return Selection{}, err
	}
	selection := Selection{Stage: stage, Slot: SlotActive, Ref: ref, UpdatedAt: s.now()}
	if err := s.store.UpsertSelection(ctx, userID, selection); err != nil {
		return Selection{}, fmt.Errorf("save selection: %w", err)
	}
	return selection, nil
}

func (s *Service) SaveComparisonPair(ctx context.Context, userID string, stage Stage, a, b llm.ModelRef) (ComparisonPair, error) {
	if a == b {
		return ComparisonPair{}, ErrDuplicateCandidates
	}
	if err := s.validateRef(stage, a); err != nil {
		return ComparisonPair{}, err
	}
	if err := s.validateRef(stage, b); err != nil {
		return ComparisonPair{}, err
	}
	now := s.now()
	pair := ComparisonPair{Stage: stage,
		CandidateA: Selection{Stage: stage, Slot: SlotCandidateA, Ref: a, UpdatedAt: now},
		CandidateB: Selection{Stage: stage, Slot: SlotCandidateB, Ref: b, UpdatedAt: now},
	}
	if err := s.store.SaveSelections(ctx, userID, []Selection{pair.CandidateA, pair.CandidateB}); err != nil {
		return ComparisonPair{}, fmt.Errorf("save comparison pair: %w", err)
	}
	return pair, nil
}

func (s *Service) RecommendationSets() []RecommendationSet {
	sets := s.catalog.RecommendationSets()
	out := make([]RecommendationSet, 0, len(sets))
	for _, set := range sets {
		converted := RecommendationSet{ID: set.ID, Label: set.Label}
		for _, selection := range set.Selections {
			stage, err := ParseStage(selection.Stage)
			if err != nil {
				continue
			}
			converted.Selections = append(converted.Selections, RecommendationStageSelection{
				Stage: stage, Active: selection.Active, CandidateA: selection.CandidateA, CandidateB: selection.CandidateB,
			})
		}
		out = append(out, converted)
	}
	return out
}

func (s *Service) ApplyRecommendationSet(ctx context.Context, userID string, id string) (RecommendationSet, []Selection, []ComparisonPair, error) {
	var selected *RecommendationSet
	for _, set := range s.RecommendationSets() {
		if set.ID == id {
			copy := set
			selected = &copy
			break
		}
	}
	if selected == nil {
		return RecommendationSet{}, nil, nil, ErrRecommendationNotFound
	}
	// A set is applied whole, and the models it names are curated data rather than the
	// config they used to be — a set that was valid when it shipped can name a model an
	// operator has since retired. So the gate runs over all nine refs before anything is
	// written, and reports every selection that blocks the set rather than the first.
	if err := s.availabilityOf(*selected); err != nil {
		return RecommendationSet{}, nil, nil, err
	}
	now := s.now()
	all := make([]Selection, 0, 9)
	active := make([]Selection, 0, 3)
	pairs := make([]ComparisonPair, 0, 3)
	for _, stageSelection := range selected.Selections {
		if stageSelection.CandidateA == stageSelection.CandidateB {
			return RecommendationSet{}, nil, nil, ErrDuplicateCandidates
		}
		activeSelection := Selection{Stage: stageSelection.Stage, Slot: SlotActive, Ref: stageSelection.Active, UpdatedAt: now}
		a := Selection{Stage: stageSelection.Stage, Slot: SlotCandidateA, Ref: stageSelection.CandidateA, UpdatedAt: now}
		b := Selection{Stage: stageSelection.Stage, Slot: SlotCandidateB, Ref: stageSelection.CandidateB, UpdatedAt: now}
		all = append(all, activeSelection, a, b)
		active = append(active, activeSelection)
		pairs = append(pairs, ComparisonPair{Stage: stageSelection.Stage, CandidateA: a, CandidateB: b})
	}
	if err := s.store.SaveSelections(ctx, userID, all); err != nil {
		return RecommendationSet{}, nil, nil, fmt.Errorf("apply recommendation set: %w", err)
	}
	return *selected, active, pairs, nil
}

func (s *Service) validateRef(stage Stage, ref llm.ModelRef) error {
	if _, err := ParseStage(string(stage)); err != nil {
		return err
	}
	info, ok := s.catalog.Lookup(ref)
	if !ok {
		return fmt.Errorf("%w: %s", ErrModelNotRegistered, ref)
	}
	if info.Disabled {
		return fmt.Errorf("%w: %s (%s)", ErrModelDisabled, ref, info.DisabledReason)
	}
	if !Suitable(stage, info) {
		return fmt.Errorf("%w: %s is not registered for %s", ErrModelUnsuitable, ref, stage)
	}
	return nil
}

// availabilityOf checks every ref a set would save against the catalog as it is right now,
// and reports all of them at once. The stage matters: the same model can be fine for write
// and unusable for observe.
func (s *Service) availabilityOf(set RecommendationSet) error {
	refusal := &SetRefusal{}
	for _, stageSelection := range set.Selections {
		for _, ref := range []llm.ModelRef{stageSelection.Active, stageSelection.CandidateA, stageSelection.CandidateB} {
			info, ok := s.catalog.Lookup(ref)
			switch {
			case !ok:
				refusal.Unregistered = append(refusal.Unregistered, ref.String())
			case info.Disabled:
				refusal.Disabled = append(refusal.Disabled, ref.String())
			case !Suitable(stageSelection.Stage, info):
				refusal.Unsuitable = append(refusal.Unsuitable, ref.String())
			}
		}
	}
	if refusal.Empty() {
		return nil
	}
	return refusal
}
