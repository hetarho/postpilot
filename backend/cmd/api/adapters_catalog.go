package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authrpc "github.com/postpilot/backend/internal/auth/rpc"
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

type estimatorCombos struct{ catalog *modelcatalog.Service }

func (e estimatorCombos) ComboRates(ctx context.Context) ([]planrpc.EstimatorCombo, error) {
	priced, err := e.catalog.ComboRates(ctx)
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

// defaultVoiceBootstrap gives a freshly provisioned account its `기본 말투` before it can
// create a post. It is idempotent, so `adduser` may be rerun to repair an account that was
// left without a voice; a failure exits non-zero because the invariant is not established.

// planBalance is the ledger as the plan edge asks for it: the translation ARCH-7 wants at the
// boundary, so `plan/rpc` publishes its own shape and the ledger's lot row stops here.
type planBalance struct{ ledger *usage.Service }

func (b planBalance) BalanceFor(ctx context.Context, userID string, acting plan.Plan) (planrpc.Balance, error) {
	found, err := b.ledger.BalanceFor(ctx, userID, acting)
	if err != nil {
		return planrpc.Balance{}, err
	}
	lots := make([]planrpc.Lot, 0, len(found.Lots))
	for _, lot := range found.Lots {
		lots = append(lots, planrpc.Lot{
			Kind: string(lot.Kind), Granted: lot.Granted, Remaining: lot.Remaining,
			ExpiresAt: lot.ExpiresAt,
		})
	}
	return planrpc.Balance{
		Credits: found.Credits, Unlimited: found.Unlimited, Lots: lots, RenewsAt: found.RenewsAt,
	}, nil
}
