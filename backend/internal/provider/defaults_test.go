package provider_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
)

type defaultCatalog struct {
	models []llm.ModelInfo
	free   map[string]bool
	paths  []llm.FreePath
}

func (c *defaultCatalog) Models() []llm.ModelInfo { return c.models }
func (c *defaultCatalog) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	for _, info := range c.models {
		if info.Ref == ref {
			return info, true
		}
	}
	return llm.ModelInfo{}, false
}
func (c *defaultCatalog) QualifyFree(_ context.Context, model string, path llm.FreePath) (bool, error) {
	c.paths = append(c.paths, path)
	return c.free == nil || c.free[model], nil
}

type defaultCredits struct{ tier plan.Plan }

func (c defaultCredits) Tier(context.Context, string) (plan.Plan, error) { return c.tier, nil }
func (defaultCredits) ForCalls([]provider.PlannedCall) int {
	panic("initialization must not price or admit model work")
}
func (defaultCredits) Balance(context.Context, string) (int, bool, error) {
	panic("initialization must not read or debit a credit balance")
}

func defaultInfo(id, grade string, stages ...provider.Stage) llm.ModelInfo {
	info := llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "openrouter", ModelID: id}, Vision: true,
		InputUSDPerMillion: "0", OutputUSDPerMillion: "0", Levels: map[string]string{}}
	for _, stage := range stages {
		info.Stages = append(info.Stages, string(stage))
		info.Levels[string(stage)] = grade
	}
	return info
}

func TestDefaultsUseOrderedEligibleActiveRecommendationsIndependently(t *testing.T) {
	ctx := context.Background()
	locked := defaultInfo("locked", "top", provider.Stages...)
	first := defaultInfo("first", "free", provider.Stages...)
	second := defaultInfo("second", "free", provider.Stages...)
	store := &fakeStore{rows: map[string]provider.Selection{}, sets: []provider.RecommendationSet{
		{ID: "first-set", Selections: []provider.RecommendationStageSelection{
			{Stage: provider.StageObserve, Active: first.Ref, CandidateA: locked.Ref, CandidateB: locked.Ref},
			{Stage: provider.StageWrite, Active: locked.Ref},
			{Stage: provider.StageAnalyze, Active: gone},
		}},
		{ID: "second-set", Selections: []provider.RecommendationStageSelection{
			{Stage: provider.StageObserve, Active: second.Ref},
			{Stage: provider.StageWrite, Active: second.Ref},
			{Stage: provider.StageAnalyze, Active: first.Ref},
		}},
	}}
	catalog := &defaultCatalog{models: []llm.ModelInfo{second, first, locked}}
	svc := provider.NewService(store, catalog, defaultCredits{plan.Free}).WithModelGrades()
	got, err := svc.InitializeDefaultSelections(ctx, "alice")
	if err != nil || len(got) != 3 {
		t.Fatalf("defaults: %+v, %v", got, err)
	}
	if store.rows["observe"].Ref != first.Ref || store.rows["write"].Ref != second.Ref || store.rows["analyze"].Ref != first.Ref || len(store.rows) != 3 {
		t.Fatalf("recommendation active refs or comparison leakage: %+v", store.rows)
	}
	if _, err := svc.InitializeDefaultSelections(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if len(store.lastBatch) != 0 || len(store.deleted) != 0 {
		t.Fatal("initializer used manual batch replacement or removed selections")
	}
}

func TestDefaultsFallbackSortsGradesAndPreservesSourceOrder(t *testing.T) {
	freeWrite := defaultInfo("free-write", "free", provider.StageWrite)
	valueFirst := defaultInfo("z-first", "value", provider.StageObserve)
	valueSecond := defaultInfo("a-second", "value", provider.StageObserve)
	balanced := defaultInfo("balanced", "balanced", provider.StageObserve, provider.StageAnalyze)
	unset := defaultInfo("unset", "", provider.Stages...)
	unsupported := defaultInfo("unsupported", "future-grade", provider.Stages...)
	store := &fakeStore{rows: map[string]provider.Selection{}}
	catalog := &defaultCatalog{models: []llm.ModelInfo{unsupported, unset, balanced, valueFirst, valueSecond, freeWrite}}
	svc := provider.NewService(store, catalog, defaultCredits{plan.Basic}).WithModelGrades()
	if _, err := svc.InitializeDefaultSelections(context.Background(), "alice"); err != nil {
		t.Fatal(err)
	}
	if store.rows["observe"].Ref != valueFirst.Ref || store.rows["analyze"].Ref != balanced.Ref || store.rows["write"].Ref != freeWrite.Ref {
		t.Fatalf("catalog order: %+v", store.rows)
	}
	if catalog.models[0].Ref != unsupported.Ref {
		t.Fatal("fallback mutated the catalog source order")
	}
}

func TestDefaultsPreserveEveryExistingActiveAndComparisonSlot(t *testing.T) {
	ctx := context.Background()
	custom := defaultInfo("custom", "free", provider.StageObserve)
	locked := defaultInfo("locked", "top", provider.StageWrite)
	ready := defaultInfo("ready", "free", provider.Stages...)
	store := &fakeStore{rows: map[string]provider.Selection{
		"observe": {Stage: provider.StageObserve, Ref: custom.Ref},
		"write":   {Stage: provider.StageWrite, Ref: locked.Ref},
		"analyze": {Stage: provider.StageAnalyze, Ref: gone},
	}}
	for _, stage := range []provider.Stage{provider.StageObserve, provider.StageWrite} {
		for _, slot := range []provider.SelectionSlot{provider.SlotCandidateA, provider.SlotCandidateB, provider.SlotCandidateC, provider.SlotCandidateD, provider.SlotCandidateE} {
			store.rows[string(stage)+"/"+string(slot)] = provider.Selection{Stage: stage, Slot: slot, Ref: gone, UpdatedAt: time.Unix(1, 0)}
		}
	}
	before := make(map[string]provider.Selection, len(store.rows))
	for key, value := range store.rows {
		before[key] = value
	}
	svc := provider.NewService(store, &defaultCatalog{models: []llm.ModelInfo{custom, locked, ready}}, defaultCredits{plan.Free}).WithModelGrades()
	for range 3 {
		got, err := svc.InitializeDefaultSelections(ctx, "alice")
		if err != nil || len(got) != 3 {
			t.Fatalf("retained defaults: %+v, %v", got, err)
		}
		for _, selection := range got {
			if selection.Stage == provider.StageAnalyze && !selection.Missing || selection.Stage == provider.StageWrite && selection.UnavailableReason != "MODEL_PLAN_REQUIRED" {
				t.Fatalf("retained reason: %+v", selection)
			}
		}
	}
	if !reflect.DeepEqual(before, store.rows) || len(store.deleted) != 0 {
		t.Fatalf("initialization replaced existing work: %+v", store.rows)
	}
}

func TestDefaultsSkipUnavailableAndVerifyFreeEndpointsForTheStage(t *testing.T) {
	drifted := defaultInfo("drifted", "free", provider.Stages...)
	drifted.OutputUSDPerMillion = "1"
	disabledInfo := defaultInfo("disabled", "free", provider.Stages...)
	disabledInfo.Disabled = true
	unsafe := defaultInfo("unsafe-endpoint", "free", provider.Stages...)
	ready := defaultInfo("ready", "free", provider.StageObserve, provider.StageWrite)
	paid := defaultInfo("paid", "value", provider.StageAnalyze)
	store := &fakeStore{rows: map[string]provider.Selection{}}
	catalog := &defaultCatalog{models: []llm.ModelInfo{drifted, disabledInfo, unsafe, paid, ready}, free: map[string]bool{"ready": true}}
	svc := provider.NewService(store, catalog, defaultCredits{plan.Free}).WithModelGrades()
	got, err := svc.InitializeDefaultSelections(context.Background(), "alice")
	if err != nil || len(got) != 2 || store.rows["observe"].Ref != ready.Ref || store.rows["write"].Ref != ready.Ref {
		t.Fatalf("unavailable/free gates: %+v %v", got, err)
	}
	var image, text bool
	for _, path := range catalog.paths {
		image = image || path == llm.FreeImageInput
		text = text || path == llm.FreeText
	}
	if !image || !text {
		t.Fatalf("stage endpoint paths: %+v", catalog.paths)
	}
}

func TestDefaultsEmptyCatalogAndFailedPersistenceNeverInventARef(t *testing.T) {
	store := &fakeStore{rows: map[string]provider.Selection{}}
	svc := provider.NewService(store, &defaultCatalog{}, defaultCredits{plan.Free}).WithModelGrades()
	got, err := svc.InitializeDefaultSelections(context.Background(), "alice")
	if err != nil || len(got) != 0 || len(store.rows) != 0 {
		t.Fatalf("empty: %+v %v", got, err)
	}
	store.batchErr = errors.New("write unavailable")
	info := defaultInfo("ready", "free", provider.Stages...)
	svc = provider.NewService(store, &defaultCatalog{models: []llm.ModelInfo{info}}, defaultCredits{plan.Free}).WithModelGrades()
	if _, err := svc.InitializeDefaultSelections(context.Background(), "alice"); !errors.Is(err, store.batchErr) || len(store.rows) != 0 {
		t.Fatalf("failed write: %+v %v", store.rows, err)
	}
}
