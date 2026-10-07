package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

type pricingFixture struct {
	user, kind string
	tier       plan.Plan
	calls      []usage.PlannedCall
}

func (f *pricingFixture) PlanOf(_ context.Context, user string) (plan.Plan, error) {
	f.user = user
	return plan.Basic, nil
}
func (f *pricingFixture) QuoteTokenPlan(_ context.Context, user string, tier plan.Plan, kind string, calls []usage.PlannedCall) (usage.TokenPlanQuote, error) {
	f.user, f.kind, f.tier, f.calls = user, kind, tier, calls
	return usage.TokenPlanQuote{Credits: 42}, nil
}
func TestWritingTestPricingPreservesExactStagesCountsBudgetsAndOwner(t *testing.T) {
	f := &pricingFixture{}
	calls := []experiment.TestCall{{Ref: experiment.ModelRef{ProviderID: "p", ModelID: "observer"}, Stage: experiment.StageObserve, Count: 3, PromptTokens: 50000, CompletionTokens: 12000}, {Ref: experiment.ModelRef{ProviderID: "p", ModelID: "writer"}, Stage: experiment.StageWrite, Count: 16, PromptTokens: 30000, CompletionTokens: 8192}}
	if q, err := NewWritingTestPricing(f, f).PriceWritingTest(t.Context(), "alice", calls); err != nil || q.Credits != 42 || q.Free {
		t.Fatal(q, err)
	}
	if f.user != "alice" || f.kind != WritingTestJobKind || f.tier != plan.Basic || len(f.calls) != 2 {
		t.Fatal(f)
	}
	for i, c := range f.calls {
		if !reflect.DeepEqual([]int64{int64(c.Count), c.PromptTokens, c.CompletionTokens}, []int64{int64(calls[i].Count), int64(calls[i].PromptTokens), int64(calls[i].CompletionTokens)}) || c.Stage != string(calls[i].Stage) || c.Ref.String() != calls[i].Ref.String() {
			t.Fatal("exact plan lost", c)
		}
	}
}
