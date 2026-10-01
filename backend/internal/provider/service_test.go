package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
)

type fakeStore struct {
	rows      map[string]provider.Selection // by stage
	deleted   []provider.Stage
	failDel   bool
	lastBatch []provider.Selection
	batchErr  error
	// sets are the operator's recommendation sets, in order.
	sets []provider.RecommendationSet
}

func (f *fakeStore) ListRecommendationSets(context.Context) ([]provider.RecommendationSet, error) {
	return append([]provider.RecommendationSet(nil), f.sets...), nil
}

func (f *fakeStore) CreateRecommendationSet(_ context.Context, set provider.RecommendationSet, limit int, _ time.Time) error {
	if len(f.sets) >= limit {
		return provider.ErrRecommendationLimit
	}
	f.sets = append(f.sets, set)
	return nil
}

func (f *fakeStore) ReplaceRecommendationSet(_ context.Context, set provider.RecommendationSet, _ time.Time) error {
	for i := range f.sets {
		if f.sets[i].ID == set.ID {
			f.sets[i] = set
			return nil
		}
	}
	return provider.ErrRecommendationNotFound
}

func (f *fakeStore) DeleteRecommendationSet(_ context.Context, id string) error {
	for i := range f.sets {
		if f.sets[i].ID == id {
			f.sets = append(f.sets[:i], f.sets[i+1:]...)
			return nil
		}
	}
	return provider.ErrRecommendationNotFound
}

func (f *fakeStore) MoveRecommendationSet(_ context.Context, id string, earlier bool) error {
	for i := range f.sets {
		if f.sets[i].ID != id {
			continue
		}
		target := i + 1
		if earlier {
			target = i - 1
		}
		if target >= 0 && target < len(f.sets) {
			f.sets[i], f.sets[target] = f.sets[target], f.sets[i]
		}
		return nil
	}
	return provider.ErrRecommendationNotFound
}

func (f *fakeStore) UpsertSelection(_ context.Context, _ string, s provider.Selection) error {
	key := string(s.Stage)
	if s.Slot != "" && s.Slot != provider.SlotActive {
		key += "/" + string(s.Slot)
	}
	f.rows[key] = s
	return nil
}

func (f *fakeStore) ListSelections(_ context.Context, _ string) ([]provider.Selection, error) {
	out := make([]provider.Selection, 0, len(f.rows))
	for _, stage := range provider.Stages {
		if s, ok := f.rows[string(stage)]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeStore) ListSelectionSlots(ctx context.Context, userID string) ([]provider.Selection, error) {
	var out []provider.Selection
	for _, stage := range provider.Stages {
		for _, slot := range []provider.SelectionSlot{provider.SlotActive, provider.SlotCandidateA, provider.SlotCandidateB, provider.SlotCandidateC, provider.SlotCandidateD, provider.SlotCandidateE} {
			key := string(stage)
			if slot != provider.SlotActive {
				key += "/" + string(slot)
			}
			if s, ok := f.rows[key]; ok {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

func (f *fakeStore) SaveSelections(ctx context.Context, userID string, selections []provider.Selection) error {
	f.lastBatch = append([]provider.Selection(nil), selections...)
	if f.batchErr != nil {
		return f.batchErr
	}
	for _, selection := range selections {
		if err := f.UpsertSelection(ctx, userID, selection); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeStore) ReplaceLabExtraCandidates(_ context.Context, _ string, stage provider.Stage, extras []provider.Selection) error {
	a, aok := f.rows[string(stage)+"/candidate_a"]
	b, bok := f.rows[string(stage)+"/candidate_b"]
	if !aok || !bok {
		return provider.ErrComparisonPairIncomplete
	}
	seen := map[llm.ModelRef]bool{a.Ref: true, b.Ref: true}
	for _, extra := range extras {
		if seen[extra.Ref] {
			return &provider.LabCandidateError{Slot: extra.Slot, Ref: extra.Ref, Cause: provider.ErrDuplicateCandidates}
		}
		seen[extra.Ref] = true
	}
	for _, slot := range []string{"candidate_c", "candidate_d", "candidate_e"} {
		delete(f.rows, string(stage)+"/"+slot)
	}
	for _, extra := range extras {
		f.rows[string(stage)+"/"+string(extra.Slot)] = extra
	}
	return nil
}

func (f *fakeStore) DeleteSelection(_ context.Context, _ string, s provider.Selection) error {
	if f.failDel {
		return errors.New("boom")
	}
	f.deleted = append(f.deleted, s.Stage)
	key := string(s.Stage)
	if s.Slot != "" && s.Slot != provider.SlotActive {
		key += "/" + string(s.Slot)
	}
	if current, ok := f.rows[key]; ok && current.Ref == s.Ref {
		delete(f.rows, key)
	}
	return nil
}

type fakeCatalog map[llm.ModelRef]llm.ModelInfo

func (c fakeCatalog) Models() []llm.ModelInfo {
	out := make([]llm.ModelInfo, 0, len(c))
	for _, m := range c {
		out = append(out, m)
	}
	return out
}

func (c fakeCatalog) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	m, ok := c[ref]
	return m, ok
}

var (
	live     = llm.ModelRef{ProviderID: "openrouter", ModelID: "live"}
	seeing   = llm.ModelRef{ProviderID: "openrouter", ModelID: "seeing"}
	disabled = llm.ModelRef{ProviderID: "anthropic", ModelID: "claude"}
	gone     = llm.ModelRef{ProviderID: "openrouter", ModelID: "gone"}
)

func TestLabExtraCandidatesReplaceAndPairStaySeparate(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{rows: map[string]provider.Selection{}}
	refs := []llm.ModelRef{live, seeing,
		{ProviderID: "openrouter", ModelID: "extra-c"},
		{ProviderID: "openrouter", ModelID: "extra-d"},
		{ProviderID: "openrouter", ModelID: "extra-e"},
	}
	catalog := fakeCatalog{}
	for _, ref := range refs {
		catalog[ref] = llm.ModelInfo{Ref: ref, Stages: allStages}
	}
	svc := provider.NewService(store, catalog, fakeCredits{})
	if _, err := svc.SaveLabExtraCandidates(ctx, "alice", provider.StageWrite, refs[2:]); !errors.Is(err, provider.ErrComparisonPairIncomplete) {
		t.Fatalf("missing pair: %v", err)
	}
	if _, err := svc.SaveComparisonPair(ctx, "alice", provider.StageWrite, refs[0], refs[1]); err != nil {
		t.Fatal(err)
	}
	pair, err := svc.SaveLabExtraCandidates(ctx, "alice", provider.StageWrite, refs[2:])
	if err != nil {
		t.Fatal(err)
	}
	if len(pair.ExtraCandidates) != 3 || pair.ExtraCandidates[0].Slot != provider.SlotCandidateC || pair.ExtraCandidates[2].Slot != provider.SlotCandidateE {
		t.Fatalf("extra slots: %+v", pair.ExtraCandidates)
	}
	if _, err := svc.SaveLabExtraCandidates(ctx, "alice", provider.StageWrite, []llm.ModelRef{refs[2], refs[0]}); !errors.Is(err, provider.ErrDuplicateCandidates) {
		t.Fatalf("pair duplicate: %v", err)
	}
	pairs, err := svc.GetComparisonPairs(ctx, "alice")
	if err != nil || len(pairs) != 1 || len(pairs[0].ExtraCandidates) != 3 {
		t.Fatalf("failed save changed rows: %+v %v", pairs, err)
	}
	updatedPair, err := svc.SaveComparisonPair(ctx, "alice", provider.StageWrite, refs[1], refs[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedPair.ExtraCandidates) != 3 {
		t.Fatalf("pair response erased extras: %+v", updatedPair)
	}
	pairs, err = svc.GetComparisonPairs(ctx, "alice")
	if err != nil || len(pairs[0].ExtraCandidates) != 3 {
		t.Fatalf("pair change erased extras: %+v %v", pairs, err)
	}
	store.sets = []provider.RecommendationSet{{ID: "set", Selections: []provider.RecommendationStageSelection{
		{Stage: provider.StageObserve, Active: refs[0], CandidateA: refs[0], CandidateB: refs[1]},
		{Stage: provider.StageAnalyze, Active: refs[0]},
		{Stage: provider.StageWrite, Active: refs[0], CandidateA: refs[0], CandidateB: refs[1]},
	}}}
	if _, _, _, err := svc.ApplyRecommendationSet(ctx, "alice", "set"); err != nil {
		t.Fatal(err)
	}
	pairs, err = svc.GetComparisonPairs(ctx, "alice")
	if err != nil || len(pairs[1].ExtraCandidates) != 3 {
		t.Fatalf("recommendation erased extras: %+v %v", pairs, err)
	}
	pair, err = svc.SaveLabExtraCandidates(ctx, "alice", provider.StageWrite, nil)
	if err != nil || len(pair.ExtraCandidates) != 0 || pair.CandidateA.Ref != refs[0] {
		t.Fatalf("clear extras: %+v %v", pair, err)
	}
}

func newService(store *fakeStore) *provider.Service {
	return provider.NewService(store, fakeCatalog{
		live:     {Ref: live, Stages: textStages},
		seeing:   {Ref: seeing, Vision: true, Stages: allStages},
		disabled: {Ref: disabled, Disabled: true, DisabledReason: llm.DisabledReasonNoKey, Stages: allStages},
	}, fakeCredits{})
}

// Stage membership comes from purpose registration (MODEL-14): a text model is registered
// to writing/style-analysis, a vision model to photo-analysis as well.
var (
	allStages  = []string{"observe", "write", "analyze"}
	textStages = []string{"write", "analyze"}
)

// fakeCredits prices every model the same and always affords it: these tests are about
// which models are listed, kept and refused for reasons OTHER than money.
type fakeCredits struct{}

func (fakeCredits) ForCalls(calls []provider.PlannedCall) int { return 5 * len(calls) }

func (fakeCredits) Balance(context.Context, string) (int, bool, error) { return 1000, false, nil }

type temporarilyUnpricedCredits struct{}

func (temporarilyUnpricedCredits) ForCalls(calls []provider.PlannedCall) int {
	if len(calls) > 0 && calls[0].Ref == seeing {
		return 0 // A free model needs no official rate.
	}
	return -1
}
func (temporarilyUnpricedCredits) Balance(context.Context, string) (int, bool, error) {
	return 1000, false, nil
}

func TestMissingFXKeepsFreeModelsVisibleAndMarksPaidPriceUnavailable(t *testing.T) {
	svc := provider.NewService(&fakeStore{rows: map[string]provider.Selection{}}, fakeCatalog{
		live: {Ref: live, Stages: textStages}, seeing: {Ref: seeing, Vision: true, Stages: allStages},
	}, temporarilyUnpricedCredits{})
	models, err := svc.ListModels(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range models {
		if model.Info.Ref == live && (!model.PriceUnavailable || model.Affordable || model.RequiredCredits != 0) {
			t.Fatalf("paid model without FX: %+v", model)
		}
		if model.Info.Ref == seeing && (model.PriceUnavailable || !model.Affordable || model.RequiredCredits != 0) {
			t.Fatalf("free model without FX: %+v", model)
		}
	}
}

type recordingCredits struct {
	calls [][]provider.PlannedCall
}

func (r *recordingCredits) ForCalls(calls []provider.PlannedCall) int {
	r.calls = append(r.calls, append([]provider.PlannedCall(nil), calls...))
	return 5
}

func (*recordingCredits) Balance(context.Context, string) (int, bool, error) {
	return 1000, false, nil
}

func TestCatalogAndPostEstimateNameTheStageTheyQuote(t *testing.T) {
	credits := &recordingCredits{}
	svc := provider.NewService(&fakeStore{rows: map[string]provider.Selection{}}, fakeCatalog{
		live:   {Ref: live, Stages: textStages, ReasoningNativeEffort: true},
		seeing: {Ref: seeing, Stages: allStages},
	}, credits)

	if _, err := svc.ListModels(context.Background(), "alice"); err != nil {
		t.Fatal(err)
	}
	quoted := map[llm.ModelRef]provider.Stage{}
	for _, calls := range credits.calls {
		if len(calls) == 1 {
			quoted[calls[0].Ref] = calls[0].Stage
		}
	}
	if quoted[live] != provider.StageWrite || quoted[seeing] != provider.StageObserve {
		t.Fatalf("catalog quote stages = %v, want live=write and seeing=observe", quoted)
	}
	for _, calls := range credits.calls {
		if len(calls) == 1 && calls[0].Ref == live && !calls[0].NativeEffort {
			t.Fatal("catalog quote dropped native-effort pricing")
		}
	}

}

// A model not registered to observe's purpose (photo-analysis) is as gone for observe as a
// deleted one: reported missing, cleared, and refused on save.
func TestObserveNeedsARegisteredModel(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{rows: map[string]provider.Selection{
		"observe": {Stage: provider.StageObserve, Ref: live},
	}}
	svc := newService(store)

	got, _ := svc.GetSelections(ctx, "alice")
	if len(got) != 1 || !got[0].Missing || len(store.deleted) != 1 {
		t.Fatalf("selections = %+v deleted = %v", got, store.deleted)
	}
	if _, err := svc.SaveSelection(ctx, "alice", provider.StageObserve, live); !errors.Is(err, provider.ErrModelUnsuitable) {
		t.Errorf("unregistered for observe = %v", err)
	}
	if _, err := svc.SaveSelection(ctx, "alice", provider.StageObserve, seeing); err != nil {
		t.Errorf("registered for observe = %v", err)
	}
	if _, err := svc.SaveSelection(ctx, "alice", provider.StageWrite, live); err != nil {
		t.Errorf("text-only for write = %v", err)
	}
}

// MODEL-24: a saved model that left the registry is reported missing once and cleared.
func TestGetSelections_MarksAndClearsVanishedModels(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{
		"observe": {Stage: provider.StageObserve, Ref: seeing},
		"write":   {Stage: provider.StageWrite, Ref: gone},
	}}
	svc := newService(store)

	got, err := svc.GetSelections(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Missing || !got[1].Missing {
		t.Fatalf("selections = %+v", got)
	}
	if len(store.deleted) != 1 || store.deleted[0] != provider.StageWrite {
		t.Errorf("deleted = %v, want just the vanished stage cleared", store.deleted)
	}

	again, _ := svc.GetSelections(context.Background(), "alice")
	if len(again) != 1 {
		t.Errorf("second read = %+v, the vanished choice should be gone", again)
	}
}

func TestGetSelections_ADeleteFailureStillAnswers(t *testing.T) {
	store := &fakeStore{failDel: true, rows: map[string]provider.Selection{
		"write": {Stage: provider.StageWrite, Ref: gone},
	}}
	got, err := newService(store).GetSelections(context.Background(), "alice")
	if err != nil || len(got) != 1 || !got[0].Missing {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestSaveSelection_Rules(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	svc := newService(store)
	ctx := context.Background()

	if _, err := svc.SaveSelection(ctx, "alice", provider.Stage("draw"), live); !errors.Is(err, provider.ErrUnknownStage) {
		t.Errorf("unknown stage = %v", err)
	}
	if _, err := svc.SaveSelection(ctx, "alice", provider.StageWrite, gone); !errors.Is(err, provider.ErrModelNotRegistered) {
		t.Errorf("unregistered = %v", err)
	}
	// MODEL-25: a disabled model cannot be selected, not even by a hand-made request.
	if _, err := svc.SaveSelection(ctx, "alice", provider.StageWrite, disabled); !errors.Is(err, provider.ErrModelDisabled) {
		t.Errorf("disabled = %v", err)
	}
	if len(store.rows) != 0 {
		t.Fatalf("a refused save wrote a row: %+v", store.rows)
	}

	saved, err := svc.SaveSelection(ctx, "alice", provider.StageWrite, live)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Ref != live || saved.UpdatedAt.IsZero() || store.rows["write"].Ref != live {
		t.Errorf("saved = %+v, rows = %+v", saved, store.rows)
	}
}

func TestComparisonPairValidation(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	svc := newService(store)
	ctx := context.Background()
	// MODEL-23: analyze keeps its active selection alone.
	if _, err := svc.SaveComparisonPair(ctx, "alice", provider.StageAnalyze, live, seeing); !errors.Is(err, provider.ErrStageWithoutPair) || len(store.lastBatch) != 0 {
		t.Fatalf("analyze pair = %v batch=%+v", err, store.lastBatch)
	}
	if _, err := svc.SaveComparisonPair(ctx, "alice", provider.StageWrite, live, live); !errors.Is(err, provider.ErrDuplicateCandidates) {
		t.Fatalf("duplicate pair = %v", err)
	}
	if _, err := svc.SaveComparisonPair(ctx, "alice", provider.StageObserve, seeing, live); !errors.Is(err, provider.ErrModelUnsuitable) {
		t.Fatalf("unregistered observe candidate = %v", err)
	}
	pair, err := svc.SaveComparisonPair(ctx, "alice", provider.StageWrite, live, seeing)
	if err != nil || pair.CandidateA.Slot != provider.SlotCandidateA || pair.CandidateB.Slot != provider.SlotCandidateB || len(store.lastBatch) != 2 {
		t.Fatalf("pair = %+v err=%v batch=%+v", pair, err, store.lastBatch)
	}
}

// MODEL-26: a set is seven refs — observe's and write's active model and pair, analyze's
// active model alone — validated whole before one batch writes them.
func TestRecommendationValidatesAllSevenBeforeOneBatch(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}, sets: []provider.RecommendationSet{{
		ID: "balanced", Label: "Balanced",
		Selections: []provider.RecommendationStageSelection{
			{Stage: provider.StageObserve, Active: seeing, CandidateA: seeing, CandidateB: live},
			{Stage: provider.StageAnalyze, Active: live},
			{Stage: provider.StageWrite, Active: live, CandidateA: live, CandidateB: seeing},
		},
	}}}
	catalog := fakeCatalog{
		live:   {Ref: live, Stages: textStages},
		seeing: {Ref: seeing, Vision: true, Stages: allStages},
	}
	svc := provider.NewService(store, catalog, fakeCredits{})
	if _, _, _, err := svc.ApplyRecommendationSet(context.Background(), "alice", "balanced"); !errors.Is(err, provider.ErrModelUnsuitable) {
		t.Fatalf("invalid recommendation = %v", err)
	}
	if len(store.lastBatch) != 0 {
		t.Fatalf("invalid recommendation partially wrote: %+v", store.lastBatch)
	}

	store.sets[0].Selections[0].CandidateB = llm.ModelRef{ProviderID: "p", ModelID: "vision-two"}
	visionTwo := store.sets[0].Selections[0].CandidateB
	catalog[visionTwo] = llm.ModelInfo{Ref: visionTwo, Vision: true, Stages: allStages}
	svc = provider.NewService(store, catalog, fakeCredits{})
	_, active, pairs, err := svc.ApplyRecommendationSet(context.Background(), "alice", "balanced")
	if err != nil || len(active) != 3 || len(pairs) != 2 || len(store.lastBatch) != 7 {
		t.Fatalf("apply = active:%d pairs:%d batch:%d err=%v", len(active), len(pairs), len(store.lastBatch), err)
	}
	for _, row := range store.lastBatch {
		if row.Stage == provider.StageAnalyze && row.Slot != provider.SlotActive {
			t.Fatalf("an analyze pair row was written: %+v", row)
		}
	}
}

// MODEL-25: the set's models are curated data, so a saved set can name one an operator has
// since retired or disabled. The refusal names every offending ref at once,
// grouped by cause — discovering them one apply at a time would be seven round trips.
func TestRecommendationRefusalNamesEveryOffendingRef(t *testing.T) {
	retired := llm.ModelRef{ProviderID: "openrouter", ModelID: "retired"}
	store := &fakeStore{rows: map[string]provider.Selection{}, sets: []provider.RecommendationSet{{
		ID: "balanced", Label: "Balanced",
		Selections: []provider.RecommendationStageSelection{
			// `live` is not registered to photo-analysis, so it is unusable for observe;
			// `retired` is gone from the catalog entirely; `disabled` is curated but
			// delisted.
			{Stage: provider.StageObserve, Active: seeing, CandidateA: seeing, CandidateB: live},
			{Stage: provider.StageAnalyze, Active: retired},
			{Stage: provider.StageWrite, Active: disabled, CandidateA: live, CandidateB: seeing},
		},
	}}}
	catalog := fakeCatalog{
		live:     {Ref: live, Stages: textStages},
		seeing:   {Ref: seeing, Vision: true, Stages: allStages},
		disabled: {Ref: disabled, Disabled: true, DisabledReason: llm.DisabledReasonDelisted, Stages: allStages},
	}
	svc := provider.NewService(store, catalog, fakeCredits{})

	_, _, _, err := svc.ApplyRecommendationSet(context.Background(), "alice", "balanced")
	var refusal *provider.SetRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a grouped SetRefusal", err)
	}
	if len(refusal.Unregistered) != 1 || refusal.Unregistered[0] != retired.String() {
		t.Errorf("unregistered = %v", refusal.Unregistered)
	}
	if len(refusal.Disabled) != 1 || refusal.Disabled[0] != disabled.String() {
		t.Errorf("disabled = %v", refusal.Disabled)
	}
	if len(refusal.Unsuitable) != 1 || refusal.Unsuitable[0] != live.String() {
		t.Errorf("unsuitable = %v", refusal.Unsuitable)
	}
	// The sentinels still match, so a caller that only handles the single-ref form is not
	// broken by the grouped one.
	for _, sentinel := range []error{provider.ErrModelNotRegistered, provider.ErrModelDisabled, provider.ErrModelUnsuitable} {
		if !errors.Is(err, sentinel) {
			t.Errorf("errors.Is(%v) = false", sentinel)
		}
	}
	if params := refusal.Params()["models"]; params == "" {
		t.Error("the refusal carries no models param for the client to render")
	}
	if len(store.lastBatch) != 0 {
		t.Fatalf("a refused set partially wrote: %+v", store.lastBatch)
	}
}

// A7: model access is not a tier decision any more. A free account may select, save and
// keep any model in the registry; the only thing that can refuse one is a balance, and a
// balance is temporary — so nothing here is ever reported missing or cleared for money.
func TestAnyTierMaySelectAndKeepAnyModel(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{rows: map[string]provider.Selection{
		"write": {Stage: provider.StageWrite, Ref: premium},
	}}
	svc := newTieredService(store)

	got, err := svc.GetSelections(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Missing {
		t.Fatalf("selections = %+v, want the choice usable whatever the tier", got)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("deleted = %v, want nothing cleared", store.deleted)
	}

	if _, err := svc.SaveSelection(ctx, "alice", provider.StageWrite, premium); err != nil {
		t.Errorf("save = %v, want any model saveable", err)
	}
	if _, err := svc.SaveComparisonPair(ctx, "alice", provider.StageWrite, live, premium); err != nil {
		t.Errorf("save pair = %v, want any model saveable", err)
	}
}

// A11: the catalog prices each model for the CALLER and says whether the balance covers
// it, so a picker can disable what is unaffordable and name the number.
func TestListModelsPricesPerCaller(t *testing.T) {
	ctx := context.Background()
	svc := provider.NewService(&fakeStore{rows: map[string]provider.Selection{}}, fakeCatalog{
		live:    {Ref: live, Stages: textStages},
		premium: {Ref: premium, Stages: textStages},
	}, poorCredits{})

	models, err := svc.ListModels(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) == 0 {
		t.Fatal("no models listed")
	}
	for _, model := range models {
		if model.RequiredCredits != 5 {
			t.Errorf("%s required = %d, want the estimator's answer", model.Info.Ref, model.RequiredCredits)
		}
		if model.Affordable {
			t.Errorf("%s reported affordable on a balance of 1", model.Info.Ref)
		}
	}
}

// poorCredits affords nothing, which is the case the picker has to render.
type poorCredits struct{}

func (poorCredits) ForCalls(calls []provider.PlannedCall) int { return 5 * len(calls) }

func (poorCredits) Balance(context.Context, string) (int, bool, error) { return 1, false, nil }

// An unlimited account affords everything without a balance being consulted at all.
func TestListModelsAffordsEverythingForAnUnlimitedAccount(t *testing.T) {
	svc := provider.NewService(&fakeStore{rows: map[string]provider.Selection{}}, fakeCatalog{
		premium: {Ref: premium, Stages: textStages},
	}, unlimitedCredits{})

	models, err := svc.ListModels(context.Background(), "root")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range models {
		if !model.Affordable {
			t.Errorf("%s was not affordable for an unlimited account", model.Info.Ref)
		}
	}
}

type unlimitedCredits struct{}

func (unlimitedCredits) ForCalls(calls []provider.PlannedCall) int { return 999 }

func (unlimitedCredits) Balance(context.Context, string) (int, bool, error) { return 0, true, nil }

var premium = llm.ModelRef{ProviderID: "openrouter", ModelID: "premium"}

// newTieredService is newService plus one expensive model.
func newTieredService(store *fakeStore) *provider.Service {
	return provider.NewService(store, fakeCatalog{
		live:    {Ref: live, Stages: textStages},
		seeing:  {Ref: seeing, Vision: true, Stages: allStages},
		premium: {Ref: premium, Stages: textStages},
	}, fakeCredits{})
}

// MODEL-23: a stray analyze pair row — a database from before the pair's retirement — never
// reads back as a pair.
func TestComparisonPairsNameObserveAndWriteOnly(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{
		"analyze": {Stage: provider.StageAnalyze, Slot: provider.SlotCandidateA, Ref: live},
		"write":   {Stage: provider.StageWrite, Slot: provider.SlotCandidateA, Ref: live},
	}}
	pairs, err := newService(store).GetComparisonPairs(context.Background(), "alice")
	if err != nil || len(pairs) != 1 || pairs[0].Stage != provider.StageWrite || pairs[0].CandidateA.Ref != live {
		t.Fatalf("pairs = %+v err=%v", pairs, err)
	}
}

type recordingFigures struct{ asked []string }

func (f *recordingFigures) StageFigure(_ context.Context, stage provider.Stage, info llm.ModelInfo) (plan.PostFigure, bool) {
	f.asked = append(f.asked, info.Ref.ModelID+"/"+string(stage))
	if stage == provider.StageAnalyze {
		return plan.PostFigure{}, false
	}
	return plan.PostFigure{Credits: 17, Basis: plan.PostCreditsRecentUsage}, true
}

// QUOTA-64: the list carries a per-post figure for each stage a model serves at a paid grade,
// none for a free or ungraded stage, and none for a stage the figures have no answer for.
func TestListModelsCarriesPerPostFiguresForPaidStagesOnly(t *testing.T) {
	graded := llm.ModelRef{ProviderID: "openrouter", ModelID: "graded"}
	ungraded := llm.ModelRef{ProviderID: "openrouter", ModelID: "ungraded"}
	figures := &recordingFigures{}
	svc := provider.NewService(&fakeStore{rows: map[string]provider.Selection{}}, fakeCatalog{
		graded: {Ref: graded, Vision: true, Stages: []string{"observe", "write", "analyze"},
			Levels: map[string]string{"observe": "free", "write": "balanced", "analyze": "value"}},
		ungraded: {Ref: ungraded, Stages: []string{"write"}},
	}, fakeCredits{}).WithPostFigures(figures)

	models, err := svc.ListModels(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	byModel := map[string][]provider.StagePostCredits{}
	for _, model := range models {
		byModel[model.Info.Ref.ModelID] = model.PostCredits
	}
	got := byModel["graded"]
	if len(got) != 1 || got[0].Stage != provider.StageWrite || got[0].Figure != (plan.PostFigure{Credits: 17, Basis: plan.PostCreditsRecentUsage}) {
		t.Fatalf("graded figures = %+v, want write only", got)
	}
	if len(byModel["ungraded"]) != 0 {
		t.Fatalf("ungraded figures = %+v, want none", byModel["ungraded"])
	}
	for _, asked := range figures.asked {
		if asked == "graded/observe" || asked == "ungraded/write" {
			t.Fatalf("figures were asked for a free or ungraded stage: %v", figures.asked)
		}
	}

	bare, err := provider.NewService(&fakeStore{rows: map[string]provider.Selection{}}, fakeCatalog{
		graded: {Ref: graded, Stages: []string{"write"}, Levels: map[string]string{"write": "balanced"}},
	}, fakeCredits{}).ListModels(context.Background(), "alice")
	if err != nil || len(bare) != 1 || len(bare[0].PostCredits) != 0 {
		t.Fatalf("a service without figures listed %+v err=%v", bare, err)
	}
}
