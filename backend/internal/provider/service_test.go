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
		live:     {Ref: live},
		seeing:   {Ref: seeing, Vision: true},
		disabled: {Ref: disabled, Disabled: true, DisabledReason: llm.DisabledReasonNoKey},
	})
}

// A text-only model saved for observe (its `vision` flag was dropped in the yaml) is as
// gone as a deleted one: reported missing, cleared, and refused on save.
func TestObserveNeedsAVisionModel(t *testing.T) {
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
		t.Errorf("text-only for observe = %v", err)
	}
	if _, err := svc.SaveSelection(ctx, "alice", provider.StageObserve, seeing); err != nil {
		t.Errorf("vision for observe = %v", err)
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
		t.Fatalf("text-only observe candidate = %v", err)
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
			live:   {Ref: live},
			seeing: {Ref: seeing, Vision: true},
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
	svc := provider.NewService(store, catalog)
	if _, _, _, err := svc.ApplyRecommendationSet(context.Background(), "alice", "balanced"); !errors.Is(err, provider.ErrModelUnsuitable) {
		t.Fatalf("invalid recommendation = %v", err)
	}
	if len(store.lastBatch) != 0 {
		t.Fatalf("invalid recommendation partially wrote: %+v", store.lastBatch)
	}

	catalog.sets[0].Selections[0].CandidateB = llm.ModelRef{ProviderID: "p", ModelID: "vision-two"}
	visionTwo := catalog.sets[0].Selections[0].CandidateB
	catalog.fakeCatalog[visionTwo] = llm.ModelInfo{Ref: visionTwo, Vision: true}
	svc = provider.NewService(store, catalog)
	_, active, pairs, err := svc.ApplyRecommendationSet(context.Background(), "alice", "balanced")
	if err != nil || len(active) != 3 || len(pairs) != 3 || len(store.lastBatch) != 9 {
		t.Fatalf("apply = active:%d pairs:%d batch:%d err=%v", len(active), len(pairs), len(store.lastBatch), err)
	}
}
