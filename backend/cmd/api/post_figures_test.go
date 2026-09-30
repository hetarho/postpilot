package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/provider"
	"github.com/postpilot/backend/internal/usage"
)

type countingRecent struct {
	reads   int
	figures map[usage.StageModel]usage.RecentPostFigure
	err     error
}

func (c *countingRecent) RecentPostFigures(context.Context) (map[usage.StageModel]usage.RecentPostFigure, error) {
	c.reads++
	return c.figures, c.err
}

type fixedRate struct {
	rate plan.RateSnapshot
	err  error
}

func (f fixedRate) SelectRate(context.Context) (plan.RateSnapshot, error) { return f.rate, f.err }

var (
	figureRate  = plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29", ReferenceE4: 14_000_000, AppliedE4: 14_000_000}
	figureModel = llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "openrouter", ModelID: "vendor/model"},
		InputUSDPerMillion: "1", OutputUSDPerMillion: "5"}
)

// QUOTA-64: an eligible recent sample wins; below the floor, and for a model with no sample,
// the catalog-based estimate stands in labelled as one; a stage no post runs has no figure.
func TestPostFiguresPreferEligibleRecentUsage(t *testing.T) {
	recent := &countingRecent{figures: map[usage.StageModel]usage.RecentPostFigure{
		{Stage: "write", Model: figureModel.Ref}:   {Credits: 42, Posts: 12, Accounts: 4},
		{Stage: "observe", Model: figureModel.Ref}: {Credits: 99, Posts: 9, Accounts: 4},
	}}
	figures := &postFigures{recent: recent, rates: fixedRate{rate: figureRate}, now: time.Now}
	ctx := context.Background()

	if got, ok := figures.StageFigure(ctx, provider.StageWrite, figureModel); !ok || got != (plan.PostFigure{Credits: 42, Basis: plan.PostCreditsRecentUsage}) {
		t.Fatalf("write = %+v ok=%v, want the recent 42", got, ok)
	}
	wantObserve, _ := plan.ObservePostCreditsAt(catalogPricer(figureModel), figureRate, plan.EstimatorDefaultPhotos)
	if got, ok := figures.StageFigure(ctx, provider.StageObserve, figureModel); !ok || got != (plan.PostFigure{Credits: wantObserve, Basis: plan.PostCreditsEstimate}) {
		t.Fatalf("observe below the floor = %+v ok=%v, want the estimate %d", got, ok, wantObserve)
	}
	if _, ok := figures.StageFigure(ctx, provider.StageAnalyze, figureModel); ok {
		t.Fatal("analyze, which no post runs, got a figure")
	}
	unpriced := llm.ModelInfo{Ref: llm.ModelRef{ProviderID: "openrouter", ModelID: "vendor/unpriced"}}
	if _, ok := figures.StageFigure(ctx, provider.StageWrite, unpriced); ok {
		t.Fatal("an unpriced model with no sample got a figure")
	}
	noRate := &postFigures{recent: &countingRecent{}, rates: fixedRate{err: usage.ErrRateUnavailable}, now: time.Now}
	if _, ok := noRate.StageFigure(ctx, provider.StageWrite, figureModel); ok {
		t.Fatal("an estimate appeared without an eligible rate")
	}
}

// The 30-day aggregate is read at most once an hour; a failed refresh keeps the last good
// snapshot instead of dropping every recent figure to the estimate.
func TestPostFiguresCacheTheAggregateForAnHour(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	recent := &countingRecent{figures: map[usage.StageModel]usage.RecentPostFigure{
		{Stage: "write", Model: figureModel.Ref}: {Credits: 42, Posts: 12, Accounts: 4},
	}}
	figures := &postFigures{recent: recent, rates: fixedRate{rate: figureRate}, now: func() time.Time { return now }}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		figures.StageFigure(ctx, provider.StageWrite, figureModel)
	}
	if recent.reads != 1 {
		t.Fatalf("reads within the hour = %d, want 1", recent.reads)
	}
	now = now.Add(postFigureCacheTTL)
	recent.err = errors.New("database is locked")
	got, ok := figures.StageFigure(ctx, provider.StageWrite, figureModel)
	if recent.reads != 2 || !ok || got.Basis != plan.PostCreditsRecentUsage || got.Credits != 42 {
		t.Fatalf("after a failed refresh: reads=%d figure=%+v ok=%v, want the kept 42", recent.reads, got, ok)
	}
}

type lookupModels map[llm.ModelRef]llm.ModelInfo

func (m lookupModels) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	info, ok := m[ref]
	return info, ok
}

type stubFigures map[provider.Stage]plan.PostFigure

func (s stubFigures) StageFigure(_ context.Context, stage provider.Stage, _ llm.ModelInfo) (plan.PostFigure, bool) {
	figure, ok := s[stage]
	return figure, ok
}

// A level's per-post figure is its pair's observe and write figures summed, an estimate when
// either part is, and nothing when either part is missing.
func TestEstimatorComboSumsItsPairsFigures(t *testing.T) {
	observeRef := llm.ModelRef{ProviderID: "openrouter", ModelID: "vendor/eyes"}
	writeRef := llm.ModelRef{ProviderID: "openrouter", ModelID: "vendor/pen"}
	models := lookupModels{observeRef: {Ref: observeRef}, writeRef: {Ref: writeRef}}
	combo := modelcatalog.ComboRates{Combo: "balanced", ObserveModelID: "vendor/eyes", WriteModelID: "vendor/pen"}
	ctx := context.Background()

	both := estimatorCombos{models: models, providerID: "openrouter", figures: stubFigures{
		provider.StageObserve: {Credits: 20, Basis: plan.PostCreditsRecentUsage},
		provider.StageWrite:   {Credits: 30, Basis: plan.PostCreditsEstimate},
	}}
	if got := both.postCredits(ctx, combo); got != (plan.PostFigure{Credits: 50, Basis: plan.PostCreditsEstimate}) {
		t.Fatalf("pair figure = %+v", got)
	}
	writeOnly := estimatorCombos{models: models, providerID: "openrouter", figures: stubFigures{
		provider.StageWrite: {Credits: 30, Basis: plan.PostCreditsRecentUsage},
	}}
	if got := writeOnly.postCredits(ctx, combo); got != (plan.PostFigure{}) {
		t.Fatalf("pair with no observe figure = %+v, want none", got)
	}
	if got := (estimatorCombos{}).postCredits(ctx, combo); got != (plan.PostFigure{}) {
		t.Fatalf("unwired combos = %+v, want none", got)
	}
}
