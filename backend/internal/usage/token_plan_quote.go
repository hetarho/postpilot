package usage

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type TokenPlanQuote struct {
	Credits int
	Free    bool
}

// QuoteTokenPlan uses the same bounded costs and final currency rounding as
// ordinary Hold, without opening benefits, reserving lots or invoking models.
func (s *Service) QuoteTokenPlan(ctx context.Context, owner string, tier plan.Plan, kind string, calls []PlannedCall) (TokenPlanQuote, error) {
	if owner == "" || !tier.Valid() || kind == "" {
		return TokenPlanQuote{}, ErrPricingUnavailable
	}
	if len(calls) == 0 {
		return TokenPlanQuote{Free: true}, nil
	}
	for _, call := range calls {
		if call.Units != nil || call.Count <= 0 || call.CompletionTokens <= 0 || call.PromptTokens <= 0 || (call.Stage != llm.StageNameObserve && call.Stage != llm.StageNameWrite) {
			return TokenPlanQuote{}, ErrPricingUnavailable
		}
		info, found := s.models.Lookup(call.Ref)
		if !found || info.Disabled || !info.ServesStage(call.Stage) || !llm.ValidUnitPrice(info.InputUSDPerMillion) || !llm.ValidUnitPrice(info.OutputUSDPerMillion) {
			return TokenPlanQuote{}, ErrPricingUnavailable
		}
		cost := llm.ResolveCost(llm.CostInput{PromptTokens: HoldPromptTokenBound(call.PromptTokens), CompletionTokens: call.CompletionTokens, InputUSDPerMillion: info.InputUSDPerMillion, OutputUSDPerMillion: info.OutputUSDPerMillion})
		if cost.Source == llm.CostUnavailable {
			return TokenPlanQuote{}, ErrPricingUnavailable
		}
	}
	if err := s.CheckModelAccess(ctx, tier, kind, calls); err != nil {
		return TokenPlanQuote{}, err
	}
	cost, err := s.worstCaseMicrousd(calls)
	if err != nil {
		return TokenPlanQuote{}, err
	}
	if cost == 0 {
		return TokenPlanQuote{Free: true}, nil
	}
	rate, err := s.SelectRate(ctx)
	if err != nil {
		return TokenPlanQuote{}, err
	}
	credits, err := plan.ChargeAt(cost, rate)
	if err != nil {
		return TokenPlanQuote{}, err
	}
	return TokenPlanQuote{Credits: credits}, nil
}
