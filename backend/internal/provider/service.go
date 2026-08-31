package provider

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// Service is the catalog's use-cases.
type Service struct {
	store   Store
	catalog Catalog
	now     func() time.Time
}

// NewService wires the context.
func NewService(store Store, catalog Catalog) *Service {
	return &Service{store: store, catalog: catalog, now: time.Now}
}

// ListModels is the registry snapshot for one caller: the registry's own flags plus the
// one flag that depends on who is asking. The registry itself stays user-ignorant, so the
// comparison happens here, where the acting plan is known.
func (s *Service) ListModels(acting plan.Plan) []CatalogModel {
	models := s.catalog.Models()
	out := make([]CatalogModel, 0, len(models))
	for _, info := range models {
		out = append(out, CatalogModel{Info: info, Locked: !acting.Allows(info.MinPlan)})
	}
	return out
}

// GetSelections returns the user's per-stage choices. A choice whose model is no longer
// registered is reported `Missing` and cleared here (PRD §7: 마지막 선택 초기화), so the
// user sees the greyed entry once and then must choose again.
//
// A choice the CALLER'S PLAN may not run is reported the same way but kept: a downgrade
// is reversible state, not a broken pointer, so an upgrade must restore the user's
// choices with no re-selection.
func (s *Service) GetSelections(ctx context.Context, userID string, acting plan.Plan) ([]Selection, error) {
	selections, err := s.store.ListSelections(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list selections: %w", err)
	}
	for i := range selections {
		if selections[i].Slot == "" {
			selections[i].Slot = SlotActive
		}
		info, ok := s.catalog.Lookup(selections[i].Ref)
		if ok && !acting.Allows(info.MinPlan) {
			selections[i].Missing = true
			continue
		}
		// A model that lost `vision` in the yaml is as gone for observe as one deleted:
		// the dropdown no longer lists it, so the choice is cleared the same way.
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

func (s *Service) GetComparisonPairs(ctx context.Context, userID string, acting plan.Plan) ([]ComparisonPair, error) {
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
		case ok && !acting.Allows(info.MinPlan):
			// Locked by plan, kept by row — the same downgrade rule as an active selection.
			selection.Missing = true
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
func (s *Service) SaveSelection(ctx context.Context, userID string, acting plan.Plan, stage Stage, ref llm.ModelRef) (Selection, error) {
	if err := s.validateRef(acting, stage, ref); err != nil {
		return Selection{}, err
	}
	selection := Selection{Stage: stage, Slot: SlotActive, Ref: ref, UpdatedAt: s.now()}
	if err := s.store.UpsertSelection(ctx, userID, selection); err != nil {
		return Selection{}, fmt.Errorf("save selection: %w", err)
	}
	return selection, nil
}

func (s *Service) SaveComparisonPair(ctx context.Context, userID string, acting plan.Plan, stage Stage, a, b llm.ModelRef) (ComparisonPair, error) {
	if a == b {
		return ComparisonPair{}, ErrDuplicateCandidates
	}
	if err := s.validateRef(acting, stage, a); err != nil {
		return ComparisonPair{}, err
	}
	if err := s.validateRef(acting, stage, b); err != nil {
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

func (s *Service) ApplyRecommendationSet(ctx context.Context, userID string, acting plan.Plan, id string) (RecommendationSet, []Selection, []ComparisonPair, error) {
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
	// A set is applied whole, so the plan gate runs over all nine refs first: the user is
	// told every selection that blocks the set, not just the first one encountered.
	if err := plan.EnsureAllowed(acting, s.floorsOf(*selected)); err != nil {
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
		for _, ref := range []llm.ModelRef{stageSelection.Active, stageSelection.CandidateA, stageSelection.CandidateB} {
			if err := s.validateRef(acting, stageSelection.Stage, ref); err != nil {
				return RecommendationSet{}, nil, nil, err
			}
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

func (s *Service) validateRef(acting plan.Plan, stage Stage, ref llm.ModelRef) error {
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
		return fmt.Errorf("%w: %s has no vision, %s needs it", ErrModelUnsuitable, ref, stage)
	}
	return plan.EnsureAllowed(acting, []plan.ModelFloor{{Ref: ref.String(), MinPlan: info.MinPlan}})
}

// floorsOf collects the plan floor of every ref a set would save. An unregistered ref is
// left out — validateRef reports that as its own, more specific failure.
func (s *Service) floorsOf(set RecommendationSet) []plan.ModelFloor {
	floors := make([]plan.ModelFloor, 0, 9)
	for _, stageSelection := range set.Selections {
		for _, ref := range []llm.ModelRef{stageSelection.Active, stageSelection.CandidateA, stageSelection.CandidateB} {
			if info, ok := s.catalog.Lookup(ref); ok {
				floors = append(floors, plan.ModelFloor{Ref: ref.String(), MinPlan: info.MinPlan})
			}
		}
	}
	return floors
}
