package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
)

type tierCredits struct {
	fakeCredits
	tier plan.Plan
}

func (c tierCredits) Tier(context.Context, string) (plan.Plan, error) { return c.tier, nil }

func TestModelAccessTierMatrixAndSavedLockedRef(t *testing.T) {
	grades := []struct {
		grade    string
		required plan.Plan
	}{{"free", plan.Free}, {"value", plan.Light}, {"balanced", plan.Basic}, {"premium", plan.Pro}, {"top", plan.Max}}
	for _, tier := range []plan.Plan{plan.Free, plan.Light, plan.Basic, plan.Pro, plan.Max, plan.Master} {
		for _, row := range grades {
			ref := llm.ModelRef{ProviderID: "openrouter", ModelID: row.grade}
			price := "1"
			if row.grade == "free" {
				price = "0"
			}
			store := &fakeStore{rows: map[string]provider.Selection{}}
			svc := provider.NewService(store, fakeCatalog{ref: {Ref: ref, Stages: []string{"write"}, Levels: map[string]string{"write": row.grade}, InputUSDPerMillion: price, OutputUSDPerMillion: price}}, tierCredits{tier: tier}).WithModelGrades()
			models, err := svc.ListModels(context.Background(), "alice")
			if err != nil {
				t.Fatal(err)
			}
			want := tier == plan.Master || tier.Rank() >= row.required.Rank()
			if len(models) != 1 || len(models[0].Access) != 1 || models[0].Access[0].Entitled != want || models[0].Access[0].RequiredPlan != row.required {
				t.Fatalf("tier=%s grade=%s access=%+v", tier, row.grade, models)
			}
			_, err = svc.SaveSelection(context.Background(), "alice", provider.StageWrite, ref)
			var locked *provider.ModelAccessError
			if want && err != nil || !want && (!errors.As(err, &locked) || locked.Required != row.required) {
				t.Fatalf("tier=%s grade=%s save=%v", tier, row.grade, err)
			}
			if !want {
				store.rows["write"] = provider.Selection{Stage: provider.StageWrite, Ref: ref}
				saved, err := svc.GetSelections(context.Background(), "alice")
				if err != nil || len(saved) != 1 || saved[0].Missing || saved[0].UnavailableReason != "MODEL_PLAN_REQUIRED" {
					t.Fatalf("retained tier=%s grade=%s selection=%+v err=%v", tier, row.grade, saved, err)
				}
			}
		}
	}
}

func TestRecommendationReportsPlanAndUnclassifiedRefsBeforeAnyWrite(t *testing.T) {
	free := llm.ModelRef{ProviderID: "openrouter", ModelID: "free"}
	paid := llm.ModelRef{ProviderID: "openrouter", ModelID: "paid"}
	unset := llm.ModelRef{ProviderID: "openrouter", ModelID: "unset"}
	store := &fakeStore{rows: map[string]provider.Selection{}, sets: []provider.RecommendationSet{{ID: "mixed", Selections: []provider.RecommendationStageSelection{
		{Stage: provider.StageObserve, Active: free, CandidateA: paid, CandidateB: unset},
		{Stage: provider.StageWrite, Active: free, CandidateA: paid, CandidateB: unset},
		{Stage: provider.StageAnalyze, Active: free},
	}}}}
	catalog := fakeCatalog{
		free:  {Ref: free, Stages: allStages, Levels: map[string]string{"observe": "free", "write": "free", "analyze": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"},
		paid:  {Ref: paid, Stages: allStages, Levels: map[string]string{"observe": "top", "write": "top"}, InputUSDPerMillion: "1", OutputUSDPerMillion: "1"},
		unset: {Ref: unset, Stages: allStages},
	}
	svc := provider.NewService(store, catalog, tierCredits{tier: plan.Free}).WithModelGrades()
	_, _, _, err := svc.ApplyRecommendationSet(context.Background(), "alice", "mixed")
	var refusal *provider.SetRefusal
	if !errors.As(err, &refusal) || len(refusal.PlanLocked) != 2 || len(refusal.Unclassified) != 2 || len(store.lastBatch) != 0 {
		t.Fatalf("refusal=%+v batch=%+v err=%v", refusal, store.lastBatch, err)
	}
}

func TestDisabledComparisonRefRemainsSavedWithProviderReason(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "temporarily-disabled"}
	store := &fakeStore{rows: map[string]provider.Selection{}}
	store.rows["write"] = provider.Selection{Stage: provider.StageWrite, Slot: provider.SlotCandidateA, Ref: ref}
	svc := provider.NewService(store, fakeCatalog{ref: {
		Ref: ref, Stages: []string{"write"}, Levels: map[string]string{"write": "free"},
		InputUSDPerMillion: "0", OutputUSDPerMillion: "0", Disabled: true,
	}}, tierCredits{tier: plan.Free}).WithModelGrades()
	pairs, err := svc.GetComparisonPairs(context.Background(), "alice")
	if err != nil || len(pairs) != 1 || pairs[0].CandidateA.Missing || pairs[0].CandidateA.UnavailableReason != "MODEL_PROVIDER_UNAVAILABLE" || len(store.deleted) != 0 {
		t.Fatalf("pair=%+v deleted=%v err=%v", pairs, store.deleted, err)
	}
}

func TestLabExtraProjectionKeepsLockedAndDisabledAndClearsMissingOnce(t *testing.T) {
	locked := llm.ModelRef{ProviderID: "openrouter", ModelID: "locked"}
	disabled := llm.ModelRef{ProviderID: "openrouter", ModelID: "disabled"}
	missing := llm.ModelRef{ProviderID: "openrouter", ModelID: "missing"}
	store := &fakeStore{rows: map[string]provider.Selection{
		"write/candidate_a": {Stage: provider.StageWrite, Slot: provider.SlotCandidateA, Ref: live},
		"write/candidate_b": {Stage: provider.StageWrite, Slot: provider.SlotCandidateB, Ref: seeing},
		"write/candidate_c": {Stage: provider.StageWrite, Slot: provider.SlotCandidateC, Ref: locked},
		"write/candidate_d": {Stage: provider.StageWrite, Slot: provider.SlotCandidateD, Ref: disabled},
		"write/candidate_e": {Stage: provider.StageWrite, Slot: provider.SlotCandidateE, Ref: missing},
	}}
	catalog := fakeCatalog{
		live:     {Ref: live, Stages: textStages, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"},
		seeing:   {Ref: seeing, Stages: allStages, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0"},
		locked:   {Ref: locked, Stages: textStages, Levels: map[string]string{"write": "top"}},
		disabled: {Ref: disabled, Stages: textStages, Levels: map[string]string{"write": "free"}, InputUSDPerMillion: "0", OutputUSDPerMillion: "0", Disabled: true},
	}
	svc := provider.NewService(store, catalog, tierCredits{tier: plan.Free}).WithModelGrades()
	pairs, err := svc.GetComparisonPairs(context.Background(), "alice")
	if err != nil || len(pairs) != 1 || len(pairs[0].ExtraCandidates) != 3 {
		t.Fatalf("first read: %+v %v", pairs, err)
	}
	extras := pairs[0].ExtraCandidates
	if extras[0].UnavailableReason != "MODEL_PLAN_REQUIRED" || extras[1].UnavailableReason != "MODEL_PROVIDER_UNAVAILABLE" || !extras[2].Missing {
		t.Fatalf("extra access: %+v", extras)
	}
	pairs, err = svc.GetComparisonPairs(context.Background(), "alice")
	if err != nil || len(pairs[0].ExtraCandidates) != 2 {
		t.Fatalf("missing was not cleared once: %+v %v", pairs, err)
	}
	if pairs[0].ExtraCandidates[0].Slot != provider.SlotCandidateC || pairs[0].ExtraCandidates[1].Slot != provider.SlotCandidateD {
		t.Fatalf("read compacted before save: %+v", pairs[0].ExtraCandidates)
	}
	if _, err := svc.SaveLabExtraCandidates(context.Background(), "alice", provider.StageWrite, []llm.ModelRef{locked}); !errors.Is(err, provider.ErrModelPlanRequired) {
		t.Fatalf("locked extra was saved: %v", err)
	}
	pairs, err = svc.GetComparisonPairs(context.Background(), "alice")
	if err != nil || len(pairs[0].ExtraCandidates) != 2 {
		t.Fatalf("refused replacement changed extras: %+v %v", pairs, err)
	}
}
