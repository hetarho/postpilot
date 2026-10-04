package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
	"github.com/postpilot/backend/internal/provider"
	providerstore "github.com/postpilot/backend/internal/provider/store"
	"github.com/postpilot/backend/internal/usage"
)

// catalogRecommendations is the provider context's recommendation-set rows as the models
// document reads and replaces them (MODEL-72, MODEL-73), bound to the document's own
// transaction. The document names models by id alone; the registry's single provider fills
// the rest of each ref (MODEL-10), and a new set gets its identity here, where provider's
// id rule lives.
type catalogRecommendations struct {
	rows       *providerstore.TxStore
	providerID string
}

func (a catalogRecommendations) List(ctx context.Context) ([]modelcatalog.StoredSet, error) {
	sets, err := a.rows.ListRecommendationSets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]modelcatalog.StoredSet, 0, len(sets))
	for _, set := range sets {
		stored := modelcatalog.StoredSet{ID: set.ID, Label: set.Label}
		for _, selection := range set.Selections {
			ids := [3]string{selection.Active.ModelID, selection.CandidateA.ModelID, selection.CandidateB.ModelID}
			switch selection.Stage {
			case provider.StageObserve:
				stored.Observe = ids
			case provider.StageAnalyze:
				stored.Analyze = selection.Active.ModelID
			case provider.StageWrite:
				stored.Write = ids
			}
		}
		out = append(out, stored)
	}
	return out, nil
}

func (a catalogRecommendations) Replace(ctx context.Context, sets []modelcatalog.StoredSet, at time.Time) error {
	ref := func(modelID string) llm.ModelRef { return llm.ModelRef{ProviderID: a.providerID, ModelID: modelID} }
	out := make([]provider.RecommendationSet, 0, len(sets))
	for _, set := range sets {
		id := set.ID
		if id == "" {
			id = provider.NewRecommendationID()
		}
		out = append(out, provider.RecommendationSet{ID: id, Label: set.Label, Selections: []provider.RecommendationStageSelection{
			{Stage: provider.StageObserve, Active: ref(set.Observe[0]), CandidateA: ref(set.Observe[1]), CandidateB: ref(set.Observe[2])},
			{Stage: provider.StageAnalyze, Active: ref(set.Analyze)},
			{Stage: provider.StageWrite, Active: ref(set.Write[0]), CandidateA: ref(set.Write[1]), CandidateB: ref(set.Write[2])},
		}})
	}
	return a.rows.ReplaceRecommendationSets(ctx, out, at)
}

type catalogReasoningSpend struct {
	ledger     *usage.Service
	providerID string
}

func (a catalogReasoningSpend) ReasoningSpendByModel(ctx context.Context, stage string) ([]modelcatalog.SpendRow, error) {
	rows, err := a.ledger.ReasoningSpendByModel(ctx, stage)
	if err != nil {
		return nil, err
	}
	out := make([]modelcatalog.SpendRow, 0, len(rows))
	for _, row := range rows {
		modelID, ok := catalogModelID(row.Model, a.providerID)
		if !ok {
			// A row recorded under another provider's ref has no catalog model to join to.
			continue
		}
		out = append(out, modelcatalog.SpendRow{
			Model: modelID, Calls: row.Calls,
			ReasoningTokens: row.ReasoningTokens, CompletionTokens: row.CompletionTokens,
			ReasoningTruncations: row.ReasoningTruncations,
		})
	}
	return out, nil
}

// catalogModelID strips the registry's provider segment from a recorded ref. A provider-local
// id itself contains slashes ("z-ai/glm-5.3-flash"), so only the FIRST segment is removed and
// only when it is this registry's own.
func catalogModelID(recorded, providerID string) (string, bool) {
	prefix := providerID + "/"
	if providerID == "" || !strings.HasPrefix(recorded, prefix) {
		return "", false
	}
	return strings.TrimPrefix(recorded, prefix), true
}

type estimatorCombos struct {
	catalog *modelcatalog.Service
	ledger  *usage.Service
	// figures and models price one post on each level's pair (QUOTA-64); without them a
	// combo carries no per-post figure.
	figures    provider.PostFigures
	models     usage.Models
	providerID string
}

// postCredits is one post with photos on the combo's pair: both stage figures summed, and no
// figure when either stage has none.
func (e estimatorCombos) postCredits(ctx context.Context, combo modelcatalog.ComboRates) plan.PostFigure {
	if e.figures == nil || e.models == nil {
		return plan.PostFigure{}
	}
	observe, ok := e.models.Lookup(llm.ModelRef{ProviderID: e.providerID, ModelID: combo.ObserveModelID})
	if !ok {
		return plan.PostFigure{}
	}
	write, ok := e.models.Lookup(llm.ModelRef{ProviderID: e.providerID, ModelID: combo.WriteModelID})
	if !ok {
		return plan.PostFigure{}
	}
	observeFigure, ok := e.figures.StageFigure(ctx, provider.StageObserve, observe)
	if !ok {
		return plan.PostFigure{}
	}
	writeFigure, ok := e.figures.StageFigure(ctx, provider.StageWrite, write)
	if !ok {
		return plan.PostFigure{}
	}
	return observeFigure.Plus(writeFigure)
}

// CurrentRate is the rate a new job would select; the ledger it reads is always wired, so a
// comparison without a rate is the official source being down, never a missing selector.
func (e estimatorCombos) CurrentRate(ctx context.Context) (plan.RateSnapshot, error) {
	return e.ledger.SelectRate(ctx)
}

// ComboRatesAt uses the exact rate already disclosed on GetMyPlan.
func (e estimatorCombos) ComboRatesAt(ctx context.Context, rate plan.RateSnapshot) ([]planrpc.EstimatorCombo, error) {
	priced, err := e.catalog.ComboRatesAt(ctx, rate)
	if err != nil {
		return nil, err
	}
	out := make([]planrpc.EstimatorCombo, 0, len(priced))
	for _, combo := range priced {
		out = append(out, planrpc.EstimatorCombo{
			Combo: string(combo.Combo), ClipRates: combo.ClipRates, PostCredits: e.postCredits(ctx, combo),
		})
	}
	return out, nil
}

// comboAssigner lets the admin edge assign a combo, translating the catalog's refusals into
// the sentinels that edge declared. Without the translation the auth context would have to
// import the catalog to recognise its own error cases.
type comboAssigner struct{ catalog *modelcatalog.Service }

func (c comboAssigner) AssignCombo(ctx context.Context, combo, observeModelID, writeModelID string) error {
	err := c.catalog.AssignCombo(ctx, modelcatalog.Combo(combo), observeModelID, writeModelID)
	switch {
	case errors.Is(err, modelcatalog.ErrUnknownCombo):
		return fmt.Errorf("%w: %s", authrpc.ErrComboUnknown, combo)
	case errors.Is(err, modelcatalog.ErrComboModelUnusable):
		return fmt.Errorf("%w: %v", authrpc.ErrComboModelUnusable, err)
	}
	return err
}

// emptyModels satisfies the ledger's registry port for the provisioning paths, which only
// grant credits and never price a call. Building the real registry there would make
// account creation depend on a reachable provider catalog.
type emptyModels struct{}

func (emptyModels) Lookup(llm.ModelRef) (llm.ModelInfo, bool) { return llm.ModelInfo{}, false }

// planBalance is the ledger as the plan edge asks for it: the translation ARCH-7 wants at the
// boundary, so `plan/rpc` publishes its own shape and the ledger's lot row stops here.
type planBalance struct{ ledger *usage.Service }

type planExports struct{ windows clip.ExportWindows }

func (p planExports) Current(ctx context.Context, user string, at time.Time) (planrpc.ExportBalance, bool, error) {
	w, ok, err := p.windows.CurrentExportWindow(ctx, user, at)
	return planrpc.ExportBalance{CoverageID: w.CoverageID, StartsAt: w.Start, EndsAt: w.End,
		Allowance: w.Allowance, Used: w.Used, Reserved: w.Reserved}, ok, err
}

func (b planBalance) BalanceFor(ctx context.Context, userID string, acting plan.Plan) (planrpc.Balance, error) {
	found, err := b.ledger.BalanceFor(ctx, userID, acting)
	if err != nil {
		return planrpc.Balance{}, err
	}
	lots := make([]planrpc.Lot, 0, len(found.Lots))
	for _, lot := range found.Lots {
		lots = append(lots, planrpc.Lot{
			Kind: string(lot.Kind), Granted: lot.Granted, Remaining: lot.Remaining,
			ExpiresAt: lot.ExpiresAt, CoverageID: lot.CoverageID,
			WindowStart: lot.WindowStart, IssuanceCause: lot.IssuanceCause,
		})
	}
	return planrpc.Balance{
		Credits: found.Credits, Unlimited: found.Unlimited, Lots: lots, RenewsAt: found.RenewsAt,
		DailyGrant: found.DailyGrant, MonthlyBonus: found.MonthlyBonus,
		DailyResetsAt: found.DailyResetsAt, BonusResetsAt: found.BonusResetsAt,
		CoverageID: found.CoverageID, CoverageEnd: found.CoverageEnd,
		BenefitStart: found.BenefitStart, BenefitEnd: found.BenefitEnd,
	}, nil
}
