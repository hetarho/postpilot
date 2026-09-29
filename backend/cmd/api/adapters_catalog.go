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
	"github.com/postpilot/backend/internal/usage"
)

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
}

func (e estimatorCombos) CurrentRate(ctx context.Context) (plan.RateSnapshot, error) {
	if e.ledger == nil {
		return plan.RateSnapshot{}, usage.ErrRateUnavailable
	}
	return e.ledger.SelectRate(ctx)
}

func (e estimatorCombos) ComboRates(ctx context.Context) ([]planrpc.EstimatorCombo, error) {
	var priced []modelcatalog.ComboRates
	var err error
	if e.ledger != nil {
		rate, rateErr := e.ledger.SelectRate(ctx)
		if rateErr != nil {
			return nil, nil // Plan viewing remains available while paid AI has no usable rate.
		}
		priced, err = e.catalog.ComboRatesAt(ctx, rate)
	} else {
		priced, err = e.catalog.ComboRates(ctx)
	}
	if err != nil {
		return nil, err
	}
	out := make([]planrpc.EstimatorCombo, 0, len(priced))
	for _, combo := range priced {
		out = append(out, planrpc.EstimatorCombo{
			Combo:             string(combo.Combo),
			ObserveLabel:      combo.ObserveLabel,
			WriteLabel:        combo.WriteLabel,
			PerPhotoMilli:     combo.Rates.PerPhoto,
			PerVideoMilli:     combo.Rates.PerVideo,
			Per1000CharsMilli: combo.Rates.Per1000Chars,
			PerPostBaseMilli:  combo.Rates.PerPostBase,
			ClipRates:         combo.ClipRates,
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
