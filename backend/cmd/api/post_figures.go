package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/usage"
)

// postFigureCacheTTL bounds how stale a recent-usage figure may be. The read aggregates 30 days
// of rows and a figure may lag the window by up to a day (QUOTA-64), so an hour costs nothing a
// user can see and keeps the aggregate off every model-list read.
const postFigureCacheTTL = time.Hour

type recentPostFigures interface {
	RecentPostFigures(ctx context.Context) (map[usage.StageModel]usage.RecentPostFigure, error)
}

type estimateRates interface {
	SelectRate(ctx context.Context) (plan.RateSnapshot, error)
}

// postFigures is QUOTA-64's per-post figure: the ledger's recent real usage when its sample
// clears the floor, otherwise the catalog-based estimate at the current rate, labelled as one.
type postFigures struct {
	recent recentPostFigures
	rates  estimateRates
	now    func() time.Time

	mu     sync.Mutex
	readAt time.Time
	cached map[usage.StageModel]usage.RecentPostFigure
}

func newPostFigures(ledger *usage.Service) *postFigures {
	return &postFigures{recent: ledger, rates: ledger, now: time.Now}
}

func (p *postFigures) StageFigure(ctx context.Context, stage provider.Stage, info llm.ModelInfo) (plan.PostFigure, bool) {
	if figure, ok := p.recentFigure(ctx, string(stage), info.Ref); ok {
		return plan.PostFigure{Credits: figure.Credits, Basis: plan.PostCreditsRecentUsage}, true
	}
	return p.estimate(ctx, stage, info)
}

// recentFigure answers from the cached aggregate, refreshing it once it is an hour old. A
// failed refresh keeps the last good snapshot; with none, every figure falls to the estimate.
func (p *postFigures) recentFigure(ctx context.Context, stage string, ref llm.ModelRef) (usage.RecentPostFigure, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if now := p.now(); p.cached == nil || now.Sub(p.readAt) >= postFigureCacheTTL {
		fresh, err := p.recent.RecentPostFigures(ctx)
		if err != nil {
			slog.Warn("recent post figures unavailable; using the estimate", "err", err)
		} else {
			p.cached, p.readAt = fresh, now
		}
	}
	figure, ok := p.cached[usage.StageModel{Stage: stage, Model: ref}]
	return figure, ok && figure.Eligible()
}

// estimate prices one post from the catalog at the default item conditions. Only observe and
// write run in a post; with no eligible rate or no bounded price there is no figure.
func (p *postFigures) estimate(ctx context.Context, stage provider.Stage, info llm.ModelInfo) (plan.PostFigure, bool) {
	rate, err := p.rates.SelectRate(ctx)
	if err != nil {
		return plan.PostFigure{}, false
	}
	pricer := catalogPricer(info)
	var credits int
	var ok bool
	switch stage {
	case provider.StageObserve:
		credits, ok = plan.ObservePostCreditsAt(pricer, rate, plan.EstimatorDefaultPhotos)
	case provider.StageWrite:
		credits, ok = plan.WritePostCreditsAt(pricer, rate, plan.EstimatorDefaultChars)
	}
	if !ok {
		return plan.PostFigure{}, false
	}
	return plan.PostFigure{Credits: credits, Basis: plan.PostCreditsEstimate}, true
}

// catalogPricer prices assumed token counts at the registry's catalog prices, the way the
// estimator cards do; a price the catalog cannot bound is no estimate.
func catalogPricer(info llm.ModelInfo) plan.Pricer {
	return func(promptTokens, completionTokens int64) (int64, bool) {
		cost := llm.ResolveCost(llm.CostInput{
			PromptTokens: promptTokens, CompletionTokens: completionTokens,
			InputUSDPerMillion: info.InputUSDPerMillion, OutputUSDPerMillion: info.OutputUSDPerMillion,
		})
		return cost.Microusd, cost.Source == llm.CostEstimated
	}
}

var _ provider.PostFigures = (*postFigures)(nil)
