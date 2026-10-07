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

type inspectionSelectionStore struct {
	provider.Store // Any unlisted behavior is forbidden in this read-only test.
	selections     []provider.Selection
	users          []string
	writes         int
	err            error
}

func (s *inspectionSelectionStore) ListSelections(_ context.Context, userID string) ([]provider.Selection, error) {
	s.users = append(s.users, userID)
	return append([]provider.Selection(nil), s.selections...), s.err
}
func (s *inspectionSelectionStore) UpsertSelection(context.Context, string, provider.Selection) error {
	s.writes++
	return errors.New("inspection attempted a selection save")
}
func (s *inspectionSelectionStore) DeleteSelection(context.Context, string, provider.Selection) error {
	s.writes++
	return errors.New("inspection attempted selection cleanup")
}
func (s *inspectionSelectionStore) InsertDefaultSelections(context.Context, string, []provider.Selection) error {
	s.writes++
	return errors.New("inspection attempted default initialization")
}

type inspectionSelectionCatalog struct {
	models             map[llm.ModelRef]llm.ModelInfo
	lookups            []llm.ModelRef
	listCalls          int
	qualificationCalls int
	freePathCalls      int
}

func (c *inspectionSelectionCatalog) Models() []llm.ModelInfo {
	c.listCalls++
	return nil
}
func (c *inspectionSelectionCatalog) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	c.lookups = append(c.lookups, ref)
	info, found := c.models[ref]
	return info, found
}
func (c *inspectionSelectionCatalog) QualifyFree(context.Context, string, llm.FreePath) (bool, error) {
	c.qualificationCalls++
	return false, errors.New("uncached endpoint qualification would call the provider")
}
func (c *inspectionSelectionCatalog) HasFreePath(context.Context, string, llm.FreePath) (bool, error) {
	c.freePathCalls++
	return false, errors.New("free path inspection would call the provider")
}

type inspectionSelectionCredits struct {
	tier  plan.Plan
	users []string
	err   error
}

func (c *inspectionSelectionCredits) Tier(_ context.Context, userID string) (plan.Plan, error) {
	c.users = append(c.users, userID)
	return c.tier, c.err
}
func (*inspectionSelectionCredits) Balance(context.Context, string) (int, bool, error) {
	panic("inspection read credit balance")
}
func (*inspectionSelectionCredits) ForCalls([]provider.PlannedCall) int {
	panic("inspection priced or admitted work")
}

func assertSelectionInspectionOnlyReads(t *testing.T, store *inspectionSelectionStore, catalog *inspectionSelectionCatalog) {
	t.Helper()
	if store.writes != 0 || catalog.listCalls != 0 || catalog.qualificationCalls != 0 || catalog.freePathCalls != 0 {
		t.Fatalf("inspection invoked side effects: writes=%d list=%d qualification=%d freepath=%d", store.writes, catalog.listCalls, catalog.qualificationCalls, catalog.freePathCalls)
	}
}

func TestExplicitModelForInspectionPreservesTheActiveChoiceAndNeverAdmitsWork(t *testing.T) {
	info := defaultInfo("frozen-writer", "free", provider.StageWrite)
	store := &inspectionSelectionStore{selections: []provider.Selection{{Stage: provider.StageWrite, Ref: llm.ModelRef{ProviderID: "p", ModelID: "new-active"}}}}
	catalog := &inspectionSelectionCatalog{models: map[llm.ModelRef]llm.ModelInfo{info.Ref: info}}
	credits := &inspectionSelectionCredits{tier: plan.Free}
	service := provider.NewService(store, catalog, credits).WithModelGrades()
	got, eligible, err := service.ModelForInspection(t.Context(), "alice", provider.StageWrite, info.Ref)
	if err != nil || !eligible || got.Ref != info.Ref || len(store.users) != 0 {
		t.Fatalf("explicit frozen selection changed: %+v %v %v", got, eligible, err)
	}
	assertSelectionInspectionOnlyReads(t, store, catalog)
	info.Levels["write"] = "top"
	catalog.models[info.Ref] = info
	if _, eligible, err := service.ModelForInspection(t.Context(), "alice", provider.StageWrite, info.Ref); err != nil || eligible {
		t.Fatal("unentitled model prepared", err)
	}
	if _, eligible, err := service.ModelForInspection(t.Context(), "alice", provider.StageAnalyze, info.Ref); err != nil || eligible {
		t.Fatal("wrong purpose prepared", err)
	}
	if _, eligible, err := service.ModelForInspection(t.Context(), "alice", provider.StageWrite, llm.ModelRef{}); err != nil || eligible {
		t.Fatal("missing model prepared", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := service.ModelForInspection(ctx, "alice", provider.StageWrite, info.Ref); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
	assertSelectionInspectionOnlyReads(t, store, catalog)
}

func TestSelectionForInspectionNeverQualifiesFreeEndpointsOrInitializesDefaults(t *testing.T) {
	for _, stage := range provider.Stages {
		t.Run(string(stage), func(t *testing.T) {
			info := defaultInfo("free", "free", stage)
			stored := provider.Selection{Stage: stage, Ref: info.Ref, UpdatedAt: time.Unix(123, 0)}
			store := &inspectionSelectionStore{selections: []provider.Selection{stored}}
			catalog := &inspectionSelectionCatalog{models: map[llm.ModelRef]llm.ModelInfo{info.Ref: info}}
			credits := &inspectionSelectionCredits{tier: plan.Free}
			service := provider.NewService(store, catalog, credits).WithModelGrades()
			selection, found, err := service.SelectionForInspection(t.Context(), "alice", stage)
			if err != nil || !found || selection.Ref != stored.Ref || selection.UpdatedAt != stored.UpdatedAt || selection.Slot != provider.SlotActive || selection.Missing || selection.UnavailableReason != "" || selection.RequiredPlan != plan.Free {
				t.Fatalf("selection=%+v found=%v err=%v", selection, found, err)
			}
			if !reflect.DeepEqual(store.selections, []provider.Selection{stored}) || !reflect.DeepEqual(store.users, []string{"alice"}) || !reflect.DeepEqual(credits.users, []string{"alice"}) || !reflect.DeepEqual(catalog.lookups, []llm.ModelRef{info.Ref}) {
				t.Fatal("saved identity was mutated or account/stage read was widened")
			}
			assertSelectionInspectionOnlyReads(t, store, catalog)
		})
	}
	for _, selections := range [][]provider.Selection{nil, {{Stage: provider.StageWrite, Slot: provider.SlotCandidateA, Ref: live}}, {{Stage: provider.StageObserve, Ref: live}}} {
		store := &inspectionSelectionStore{selections: selections}
		catalog := &inspectionSelectionCatalog{models: map[llm.ModelRef]llm.ModelInfo{}}
		service := provider.NewService(store, catalog, &inspectionSelectionCredits{tier: plan.Free}).WithModelGrades()
		selection, found, err := service.SelectionForInspection(t.Context(), "alice", provider.StageWrite)
		if err != nil || found || selection != (provider.Selection{}) || len(catalog.lookups) != 0 {
			t.Fatalf("selection=%+v found=%v err=%v", selection, found, err)
		}
		assertSelectionInspectionOnlyReads(t, store, catalog)
	}
}

func TestSelectionForInspectionPreservesUnavailableSavedRefsWithoutSubstitution(t *testing.T) {
	for _, tc := range []struct {
		name    string
		missing bool
		reason  string
		mutate  func(*llm.ModelInfo)
	}{
		{"missing registration", true, "", nil},
		{"unsuitable stage", true, "", func(info *llm.ModelInfo) { info.Stages = []string{llm.StageNameObserve} }},
		{"disabled provider", false, "MODEL_PROVIDER_UNAVAILABLE", func(info *llm.ModelInfo) { info.Disabled = true }},
		{"higher plan", false, "MODEL_PLAN_REQUIRED", func(info *llm.ModelInfo) { info.Levels[llm.StageNameWrite] = "top" }},
		{"unclassified", false, "MODEL_UNCLASSIFIED", func(info *llm.ModelInfo) { delete(info.Levels, llm.StageNameWrite) }},
		{"nonzero free price", false, "MODEL_FREE_PATH_UNAVAILABLE", func(info *llm.ModelInfo) { info.InputUSDPerMillion = "1" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := defaultInfo("saved-owner-choice", "free", provider.StageWrite)
			stored := provider.Selection{Stage: provider.StageWrite, Slot: provider.SlotActive, Ref: info.Ref, UpdatedAt: time.Unix(123, 0)}
			store := &inspectionSelectionStore{selections: []provider.Selection{stored}}
			catalog := &inspectionSelectionCatalog{models: map[llm.ModelRef]llm.ModelInfo{}}
			if tc.mutate != nil {
				tc.mutate(&info)
				catalog.models[info.Ref] = info
			}
			service := provider.NewService(store, catalog, &inspectionSelectionCredits{tier: plan.Free}).WithModelGrades()
			selection, found, err := service.SelectionForInspection(t.Context(), "alice", provider.StageWrite)
			if err != nil || !found || selection.Ref != stored.Ref || selection.UpdatedAt != stored.UpdatedAt || selection.Missing != tc.missing || selection.UnavailableReason != tc.reason {
				t.Fatalf("selection=%+v found=%v err=%v", selection, found, err)
			}
			if !reflect.DeepEqual(store.selections, []provider.Selection{stored}) {
				t.Fatal("unavailable choice was cleared or replaced")
			}
			assertSelectionInspectionOnlyReads(t, store, catalog)
		})
	}
}

func TestSelectionForInspectionUsesCurrentTierAndPropagatesReadFailures(t *testing.T) {
	info := defaultInfo("premium", "premium", provider.StageWrite)
	store := &inspectionSelectionStore{selections: []provider.Selection{{Stage: provider.StageWrite, Ref: info.Ref}}}
	catalog := &inspectionSelectionCatalog{models: map[llm.ModelRef]llm.ModelInfo{info.Ref: info}}
	credits := &inspectionSelectionCredits{tier: plan.Free}
	service := provider.NewService(store, catalog, credits).WithModelGrades()
	locked, _, err := service.SelectionForInspection(t.Context(), "alice", provider.StageWrite)
	if err != nil || locked.UnavailableReason != "MODEL_PLAN_REQUIRED" {
		t.Fatal(locked, err)
	}
	credits.tier = plan.Pro
	eligible, found, err := service.SelectionForInspection(t.Context(), "alice", provider.StageWrite)
	if err != nil || !found || eligible.UnavailableReason != "" || eligible.Ref != locked.Ref {
		t.Fatal(eligible, found, err)
	}
	readFailure := errors.New("read failed")
	credits.err = readFailure
	if _, _, err := service.SelectionForInspection(t.Context(), "alice", provider.StageWrite); !errors.Is(err, readFailure) {
		t.Fatal(err)
	}
	credits.err, store.err = nil, readFailure
	if _, _, err := service.SelectionForInspection(t.Context(), "alice", provider.StageWrite); !errors.Is(err, readFailure) {
		t.Fatal(err)
	}
	if _, _, err := service.SelectionForInspection(t.Context(), "alice", "unknown"); !errors.Is(err, provider.ErrUnknownStage) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := service.SelectionForInspection(ctx, "alice", provider.StageWrite); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertSelectionInspectionOnlyReads(t, store, catalog)
}

func TestSelectionForInspectionLegacyServiceStillRefusesDisabledModel(t *testing.T) {
	info := defaultInfo("disabled", "", provider.StageWrite)
	info.Disabled = true
	store := &inspectionSelectionStore{selections: []provider.Selection{{Stage: provider.StageWrite, Ref: info.Ref}}}
	catalog := &inspectionSelectionCatalog{models: map[llm.ModelRef]llm.ModelInfo{info.Ref: info}}
	credits := &inspectionSelectionCredits{err: errors.New("legacy service must not read a tier")}
	selection, found, err := provider.NewService(store, catalog, credits).SelectionForInspection(t.Context(), "alice", provider.StageWrite)
	if err != nil || !found || selection.Ref != info.Ref || selection.UnavailableReason != "MODEL_PROVIDER_UNAVAILABLE" || len(credits.users) != 0 {
		t.Fatal(selection, found, err)
	}
	assertSelectionInspectionOnlyReads(t, store, catalog)
}
