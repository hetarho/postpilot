package app

import (
	"context"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

type WritingTestPlans interface {
	PlanOf(context.Context, string) (plan.Plan, error)
}
type WritingTestTokenQuotes interface {
	QuoteTokenPlan(context.Context, string, plan.Plan, string, []usage.PlannedCall) (usage.TokenPlanQuote, error)
}
type WritingTestPricing struct {
	plans  WritingTestPlans
	quotes WritingTestTokenQuotes
}

func NewWritingTestPricing(plans WritingTestPlans, quotes WritingTestTokenQuotes) *WritingTestPricing {
	if plans == nil || quotes == nil {
		panic("experiment/app: writing test owner plans and exact token quotes are required")
	}
	return &WritingTestPricing{plans: plans, quotes: quotes}
}
func (p *WritingTestPricing) PriceWritingTest(ctx context.Context, owner string, calls []experiment.TestCall) (experiment.TestCost, error) {
	tier, err := p.plans.PlanOf(ctx, owner)
	if err != nil {
		return experiment.TestCost{}, err
	}
	priced := make([]usage.PlannedCall, len(calls))
	for i, c := range calls {
		priced[i] = usage.PlannedCall{Ref: llm.ModelRef{ProviderID: c.Ref.ProviderID, ModelID: c.Ref.ModelID}, Stage: string(c.Stage), Count: c.Count, PromptTokens: int64(c.PromptTokens), CompletionTokens: int64(c.CompletionTokens)}
	}
	quote, err := p.quotes.QuoteTokenPlan(ctx, owner, tier, WritingTestJobKind, priced)
	return experiment.TestCost{Credits: quote.Credits, Free: quote.Free}, WritingTestPreparationError(err)
}
