package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/provider"
)

type fakeStore struct {
	rows      map[string]provider.Selection // by stage
	deleted   []provider.Stage
	failDel   bool
	lastBatch []provider.Selection
	batchErr  error
}

func (f *fakeStore) UpsertSelection(_ context.Context, _ string, s provider.Selection) error {
	f.rows[string(s.Stage)] = s
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
	return f.ListSelections(ctx, userID)
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

func (f *fakeStore) DeleteSelection(_ context.Context, _ string, s provider.Selection) error {
	if f.failDel {
		return errors.New("boom")
	}
	f.deleted = append(f.deleted, s.Stage)
	if current, ok := f.rows[string(s.Stage)]; ok && current.Ref == s.Ref {
		delete(f.rows, string(s.Stage))
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

func (c fakeCatalog) RecommendationSets() []llm.RecommendationSet { return nil }

type recommendationCatalog struct {
	fakeCatalog
	sets []llm.RecommendationSet
}

func (c recommendationCatalog) RecommendationSets() []llm.RecommendationSet { return c.sets }

var (
	live     = llm.ModelRef{ProviderID: "openrouter", ModelID: "live"}
	seeing   = llm.ModelRef{ProviderID: "openrouter", ModelID: "seeing"}
	disabled = llm.ModelRef{ProviderID: "anthropic", ModelID: "claude"}
	gone     = llm.ModelRef{ProviderID: "openrouter", ModelID: "gone"}
)

func newService(store *fakeStore) *provider.Service {
	return provider.NewService(store, fakeCatalog{
		live:     {Ref: live, Stages: textStages},
		seeing:   {Ref: seeing, Vision: true, Stages: allStages},
		disabled: {Ref: disabled, Disabled: true, DisabledReason: llm.DisabledReasonNoKey, Stages: allStages},
	}, fakeCredits{})
}

// Stage membership comes from purpose registration (change 20): a text model is registered
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

// AC5: a saved model that left the registry is reported missing once and cleared.
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
	// AC2: a disabled model cannot be selected, not even by a hand-made request.
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

func TestRecommendationValidatesAllNineBeforeOneBatch(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	catalog := recommendationCatalog{
		fakeCatalog: fakeCatalog{
			live:   {Ref: live, Stages: textStages},
			seeing: {Ref: seeing, Vision: true, Stages: allStages},
		},
		sets: []llm.RecommendationSet{{
			ID: "balanced", Label: "Balanced",
			Selections: []llm.RecommendationSelection{
				{Stage: "observe", Active: seeing, CandidateA: seeing, CandidateB: live},
				{Stage: "analyze", Active: live, CandidateA: live, CandidateB: seeing},
				{Stage: "write", Active: live, CandidateA: live, CandidateB: seeing},
			},
		}},
	}
	svc := provider.NewService(store, catalog, fakeCredits{})
	if _, _, _, err := svc.ApplyRecommendationSet(context.Background(), "alice", "balanced"); !errors.Is(err, provider.ErrModelUnsuitable) {
		t.Fatalf("invalid recommendation = %v", err)
	}
	if len(store.lastBatch) != 0 {
		t.Fatalf("invalid recommendation partially wrote: %+v", store.lastBatch)
	}

	catalog.sets[0].Selections[0].CandidateB = llm.ModelRef{ProviderID: "p", ModelID: "vision-two"}
	visionTwo := catalog.sets[0].Selections[0].CandidateB
	catalog.fakeCatalog[visionTwo] = llm.ModelInfo{Ref: visionTwo, Vision: true, Stages: allStages}
	svc = provider.NewService(store, catalog, fakeCredits{})
	_, active, pairs, err := svc.ApplyRecommendationSet(context.Background(), "alice", "balanced")
	if err != nil || len(active) != 3 || len(pairs) != 3 || len(store.lastBatch) != 9 {
		t.Fatalf("apply = active:%d pairs:%d batch:%d err=%v", len(active), len(pairs), len(store.lastBatch), err)
	}
}

// A10 (plan 18): the set's models are curated data now, so a shipped set can name one an
// operator has since retired or disabled. The refusal names every offending ref at once,
// grouped by cause — discovering them one apply at a time would be nine round trips.
func TestRecommendationRefusalNamesEveryOffendingRef(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	retired := llm.ModelRef{ProviderID: "openrouter", ModelID: "retired"}
	catalog := recommendationCatalog{
		fakeCatalog: fakeCatalog{
			live:     {Ref: live, Stages: textStages},
			seeing:   {Ref: seeing, Vision: true, Stages: allStages},
			disabled: {Ref: disabled, Disabled: true, DisabledReason: llm.DisabledReasonDelisted, Stages: allStages},
		},
		sets: []llm.RecommendationSet{{
			ID: "balanced", Label: "Balanced",
			Selections: []llm.RecommendationSelection{
				// `live` is not registered to photo-analysis, so it is unusable for observe;
				// `retired` is gone from the catalog entirely; `disabled` is curated but
				// delisted.
				{Stage: "observe", Active: seeing, CandidateA: seeing, CandidateB: live},
				{Stage: "analyze", Active: retired, CandidateA: live, CandidateB: seeing},
				{Stage: "write", Active: disabled, CandidateA: live, CandidateB: seeing},
			},
		}},
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
