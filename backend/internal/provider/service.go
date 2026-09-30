package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// Service is the catalog's use-cases.
type Service struct {
	store       Store
	catalog     Catalog
	credits     Credits
	figures     PostFigures
	modelGrades bool
	now         func() time.Time
}

func (s *Service) WithModelGrades() *Service { s.modelGrades = true; return s }

// WithPostFigures attaches the per-post credit figures ListModels publishes (QUOTA-64). A
// service without them lists every model with no figure, which is what a catalog with no
// usage and no rate would show anyway.
func (s *Service) WithPostFigures(figures PostFigures) *Service { s.figures = figures; return s }

func (s *Service) tier(ctx context.Context, userID string) (plan.Plan, error) {
	if !s.modelGrades {
		return plan.Master, nil
	}
	reader, ok := s.credits.(interface {
		Tier(context.Context, string) (plan.Plan, error)
	})
	if !ok {
		return "", fmt.Errorf("model tier reader unavailable")
	}
	return reader.Tier(ctx, userID)
}

func modelAccess(tier plan.Plan, stage Stage, info llm.ModelInfo) StageAccess {
	grade := info.Levels[string(stage)]
	required, entitled := plan.AllowsModelGrade(tier, grade)
	access := StageAccess{Stage: stage, Grade: grade, RequiredPlan: required, Entitled: entitled,
		FreePathAvailable: grade != "free" || llm.ZeroUnitPrice(info.InputUSDPerMillion) && llm.ZeroUnitPrice(info.OutputUSDPerMillion)}
	switch {
	case grade == "":
		access.UnavailableReason = "MODEL_UNCLASSIFIED"
	case !entitled:
		access.UnavailableReason = "MODEL_PLAN_REQUIRED"
	case !access.FreePathAvailable:
		access.UnavailableReason = "MODEL_FREE_PATH_UNAVAILABLE"
	case info.Disabled:
		access.UnavailableReason = "MODEL_PROVIDER_UNAVAILABLE"
	}
	return access
}

func (s *Service) access(ctx context.Context, tier plan.Plan, stage Stage, info llm.ModelInfo, live bool) StageAccess {
	access := modelAccess(tier, stage, info)
	if access.Grade != "free" || access.UnavailableReason != "" {
		return access
	}
	qualifier, ok := s.catalog.(interface {
		QualifyFree(context.Context, string, llm.FreePath) (bool, error)
	})
	if !ok {
		return access
	}
	path := llm.FreeText
	if stage == StageObserve {
		path = llm.FreeImageInput
	}
	if !live {
		ctx = llm.AllowCachedEndpoints(ctx)
	}
	qualified, err := qualifier.QualifyFree(ctx, info.Ref.ModelID, path)
	if err != nil || !qualified {
		access.FreePathAvailable, access.UnavailableReason = false, "MODEL_FREE_PATH_UNAVAILABLE"
	}
	return access
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
	tier, err := s.tier(ctx, userID)
	if err != nil {
		return nil, err
	}
	balance, unlimited, err := s.credits.Balance(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("read balance: %w", err)
	}
	models := s.catalog.Models()
	out := make([]CatalogModel, 0, len(models))
	for _, info := range models {
		if s.modelGrades {
			classified := make([]string, 0, len(info.Stages))
			for _, stage := range info.Stages {
				if info.Levels[stage] != "" {
					classified = append(classified, stage)
				}
			}
			info.Stages = classified
		}
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
		unavailable := required < 0
		if unavailable {
			required = 0
		}
		entry := CatalogModel{
			Info: info, RequiredCredits: required,
			Affordable:       !unavailable && (unlimited || balance >= required),
			PriceUnavailable: unavailable,
		}
		if s.modelGrades {
			for _, name := range info.Stages {
				stage, err := ParseStage(name)
				if err == nil {
					entry.Access = append(entry.Access, s.access(ctx, tier, stage, info, false))
				}
			}
		}
		entry.PostCredits = s.postCredits(ctx, info)
		out = append(out, entry)
	}
	return out, nil
}

// postCredits is the model's per-post figure for each stage it serves at a paid grade. A free
// stage costs no credits, so it has no figure; neither does a stage the model holds no grade
// for, which ordinary selectors never list.
func (s *Service) postCredits(ctx context.Context, info llm.ModelInfo) []StagePostCredits {
	if s.figures == nil {
		return nil
	}
	var out []StagePostCredits
	for _, name := range info.Stages {
		stage, err := ParseStage(name)
		if err != nil {
			continue
		}
		if grade := info.Levels[name]; grade == "" || grade == "free" {
			continue
		}
		if figure, ok := s.figures.StageFigure(ctx, stage, info); ok {
			out = append(out, StagePostCredits{Stage: stage, Figure: figure})
		}
	}
	return out
}

// GetSelections returns the user's per-stage choices. A choice whose model is no longer
// registered is reported `Missing` and cleared here (PRD §7: 마지막 선택 초기화), so the
// user sees the greyed entry once and then must choose again.
//
// A choice the caller cannot currently AFFORD is not reported here at all: a balance is
// temporary state that the next renewal clears, so it invalidates nothing. Only a model
// that has actually vanished or become unsuitable is cleared.
func (s *Service) GetSelections(ctx context.Context, userID string) ([]Selection, error) {
	tier, err := s.tier(ctx, userID)
	if err != nil {
		return nil, err
	}
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
		// the machinery (MODEL-24) that absorbed the empty per-purpose cutover — every
		// pre-cutover selection lands here on its next read, with no bespoke migration clearing.
		if ok && Suitable(selections[i].Stage, info) {
			if s.modelGrades {
				access := s.access(ctx, tier, selections[i].Stage, info, false)
				selections[i].RequiredPlan, selections[i].UnavailableReason = access.RequiredPlan, access.UnavailableReason
			}
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
	tier, err := s.tier(ctx, userID)
	if err != nil {
		return nil, err
	}
	selections, err := s.store.ListSelectionSlots(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list comparison pairs: %w", err)
	}
	byStage := map[Stage]*ComparisonPair{}
	for _, selection := range selections {
		if selection.Slot == SlotActive || !HasPair(selection.Stage) {
			continue
		}
		info, ok := s.catalog.Lookup(selection.Ref)
		switch {
		case !ok || !Suitable(selection.Stage, info):
			selection.Missing = true
			if err := s.store.DeleteSelection(ctx, userID, selection); err != nil {
				slog.Warn("clear vanished comparison selection failed", "user", userID, "stage", selection.Stage, "slot", selection.Slot, "err", err)
			}
		}
		if s.modelGrades && !selection.Missing {
			access := s.access(ctx, tier, selection.Stage, info, false)
			selection.RequiredPlan, selection.UnavailableReason = access.RequiredPlan, access.UnavailableReason
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
	if err := s.validateRef(ctx, userID, stage, ref); err != nil {
		return Selection{}, err
	}
	selection := Selection{Stage: stage, Slot: SlotActive, Ref: ref, UpdatedAt: s.now()}
	if err := s.store.UpsertSelection(ctx, userID, selection); err != nil {
		return Selection{}, fmt.Errorf("save selection: %w", err)
	}
	return selection, nil
}

func (s *Service) SaveComparisonPair(ctx context.Context, userID string, stage Stage, a, b llm.ModelRef) (ComparisonPair, error) {
	if _, err := ParseStage(string(stage)); err != nil {
		return ComparisonPair{}, err
	}
	if !HasPair(stage) {
		return ComparisonPair{}, fmt.Errorf("%w: %s", ErrStageWithoutPair, stage)
	}
	if a == b {
		return ComparisonPair{}, ErrDuplicateCandidates
	}
	if err := s.validateRef(ctx, userID, stage, a); err != nil {
		return ComparisonPair{}, err
	}
	if err := s.validateRef(ctx, userID, stage, b); err != nil {
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

// RecommendationSets is every set in the operator's order (MODEL-71).
func (s *Service) RecommendationSets(ctx context.Context) ([]RecommendationSet, error) {
	sets, err := s.store.ListRecommendationSets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recommendation sets: %w", err)
	}
	return sets, nil
}

// SaveRecommendationSet creates a set (empty id, appended last) or replaces one whole. The
// draft is validated against the catalog as it is now and refused whole (MODEL-70); the
// operator is the caller, so no plan or balance is read. Nothing an account already chose is
// touched: an apply copies the set as it is at that moment (MODEL-71).
func (s *Service) SaveRecommendationSet(ctx context.Context, draft RecommendationSet) (RecommendationSet, error) {
	stored, err := s.store.ListRecommendationSets(ctx)
	if err != nil {
		return RecommendationSet{}, fmt.Errorf("list recommendation sets: %w", err)
	}
	otherLabels := make([]string, 0, len(stored))
	for _, set := range stored {
		if set.ID != draft.ID {
			otherLabels = append(otherLabels, set.Label)
		}
	}
	valid, refusal := ValidateRecommendationDraft(draft, s.catalog.Lookup, otherLabels)
	if refusal != nil {
		return RecommendationSet{}, refusal
	}
	now := s.now()
	if valid.ID == "" {
		valid.ID = NewRecommendationID()
		if err := s.store.CreateRecommendationSet(ctx, valid, MaxRecommendationSets, now); err != nil {
			if errors.Is(err, ErrRecommendationLimit) {
				return RecommendationSet{}, &SetLimitError{Limit: MaxRecommendationSets}
			}
			return RecommendationSet{}, fmt.Errorf("create recommendation set: %w", err)
		}
		return valid, nil
	}
	if err := s.store.ReplaceRecommendationSet(ctx, valid, now); err != nil {
		if errors.Is(err, ErrRecommendationNotFound) {
			return RecommendationSet{}, err
		}
		return RecommendationSet{}, fmt.Errorf("replace recommendation set: %w", err)
	}
	return valid, nil
}

// DeleteRecommendationSet removes one set. Accounts that applied it keep what they chose.
func (s *Service) DeleteRecommendationSet(ctx context.Context, id string) error {
	if err := s.store.DeleteRecommendationSet(ctx, id); err != nil {
		if errors.Is(err, ErrRecommendationNotFound) {
			return err
		}
		return fmt.Errorf("delete recommendation set: %w", err)
	}
	return nil
}

// MoveRecommendationSet moves one set a place up (earlier) or down in the operator's order.
func (s *Service) MoveRecommendationSet(ctx context.Context, id string, earlier bool) error {
	if err := s.store.MoveRecommendationSet(ctx, id, earlier); err != nil {
		if errors.Is(err, ErrRecommendationNotFound) {
			return err
		}
		return fmt.Errorf("move recommendation set: %w", err)
	}
	return nil
}

func (s *Service) ApplyRecommendationSet(ctx context.Context, userID string, id string) (RecommendationSet, []Selection, []ComparisonPair, error) {
	sets, err := s.RecommendationSets(ctx)
	if err != nil {
		return RecommendationSet{}, nil, nil, err
	}
	var selected *RecommendationSet
	for _, set := range sets {
		if set.ID == id {
			copy := set
			selected = &copy
			break
		}
	}
	if selected == nil {
		return RecommendationSet{}, nil, nil, ErrRecommendationNotFound
	}
	// A set is applied whole, and the models it names are curated data that changes while the
	// process runs — a set that was valid when the operator saved it can name a model since
	// retired. So the gate runs over all seven refs before anything is written, and reports
	// every selection that blocks the set rather than the first.
	if err := s.availabilityOf(ctx, userID, *selected); err != nil {
		return RecommendationSet{}, nil, nil, err
	}
	now := s.now()
	all := make([]Selection, 0, 7)
	active := make([]Selection, 0, 3)
	pairs := make([]ComparisonPair, 0, 2)
	for _, stageSelection := range selected.Selections {
		activeSelection := Selection{Stage: stageSelection.Stage, Slot: SlotActive, Ref: stageSelection.Active, UpdatedAt: now}
		all = append(all, activeSelection)
		active = append(active, activeSelection)
		if !HasPair(stageSelection.Stage) {
			continue
		}
		if stageSelection.CandidateA == stageSelection.CandidateB {
			return RecommendationSet{}, nil, nil, ErrDuplicateCandidates
		}
		a := Selection{Stage: stageSelection.Stage, Slot: SlotCandidateA, Ref: stageSelection.CandidateA, UpdatedAt: now}
		b := Selection{Stage: stageSelection.Stage, Slot: SlotCandidateB, Ref: stageSelection.CandidateB, UpdatedAt: now}
		all = append(all, a, b)
		pairs = append(pairs, ComparisonPair{Stage: stageSelection.Stage, CandidateA: a, CandidateB: b})
	}
	if err := s.store.SaveSelections(ctx, userID, all); err != nil {
		return RecommendationSet{}, nil, nil, fmt.Errorf("apply recommendation set: %w", err)
	}
	return *selected, active, pairs, nil
}

func (s *Service) validateRef(ctx context.Context, userID string, stage Stage, ref llm.ModelRef) error {
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
	if s.modelGrades {
		tier, err := s.tier(ctx, userID)
		if err != nil {
			return err
		}
		access := s.access(ctx, tier, stage, info, true)
		switch access.UnavailableReason {
		case "MODEL_UNCLASSIFIED":
			return &ModelAccessError{Ref: ref, Stage: stage, Cause: ErrModelUnclassified}
		case "MODEL_PLAN_REQUIRED":
			return &ModelAccessError{Ref: ref, Stage: stage, Grade: access.Grade, Required: access.RequiredPlan, Cause: ErrModelPlanRequired}
		case "MODEL_FREE_PATH_UNAVAILABLE":
			return &ModelAccessError{Ref: ref, Stage: stage, Grade: access.Grade, Cause: ErrFreePathUnavailable}
		}
	}
	return nil
}

// availabilityOf checks every ref a set would save against the catalog as it is right now,
// and reports all of them at once. The stage matters: the same model can be fine for write
// and unusable for observe.
func (s *Service) availabilityOf(ctx context.Context, userID string, set RecommendationSet) error {
	tier, err := s.tier(ctx, userID)
	if err != nil {
		return err
	}
	refusal := &SetRefusal{}
	for _, stageSelection := range set.Selections {
		for _, slot := range stageSelection.Slots() {
			ref := slot.Ref
			info, ok := s.catalog.Lookup(ref)
			switch {
			case !ok:
				refusal.Unregistered = append(refusal.Unregistered, ref.String())
			case info.Disabled:
				refusal.Disabled = append(refusal.Disabled, ref.String())
			case !Suitable(stageSelection.Stage, info):
				refusal.Unsuitable = append(refusal.Unsuitable, ref.String())
			case s.modelGrades:
				access := s.access(ctx, tier, stageSelection.Stage, info, true)
				switch access.UnavailableReason {
				case "MODEL_UNCLASSIFIED":
					refusal.Unclassified = append(refusal.Unclassified, ref.String())
				case "MODEL_PLAN_REQUIRED":
					refusal.PlanLocked = append(refusal.PlanLocked, ref.String()+" ("+string(access.RequiredPlan)+")")
				case "MODEL_FREE_PATH_UNAVAILABLE":
					refusal.FreePath = append(refusal.FreePath, ref.String())
				}
			}
		}
	}
	if refusal.Empty() {
		return nil
	}
	return refusal
}
