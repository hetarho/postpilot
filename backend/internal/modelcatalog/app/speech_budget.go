// Package app composes the speech catalog's qualified bindings with the usage
// ledger's product-neutral budget. Neither context reads the other's tables.
package app

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

type SpeechProfiles interface {
	ResolveSpeechProfile(context.Context, string, plan.Plan, string, int64, string, bool) (modelcatalog.SpeechProfile, error)
}

type SpeechBudgets struct{ Profiles SpeechProfiles }

// Budget is server-only. The caller has already verified ownership of the
// exact candidate/voice, then supplies its canonical request and private scope.
func (s SpeechBudgets) Budget(ctx context.Context, owner string, tier plan.Plan, id string, revision int64, authorization, scope string, input llm.SpeechInput, count int) (usage.UnitBudget, error) {
	p, err := s.Profiles.ResolveSpeechProfile(ctx, owner, tier, id, revision, authorization, false)
	if err != nil {
		return usage.UnitBudget{}, err
	}
	return speechBudget(p, authorization, scope, input, count)
}

func speechBudget(p modelcatalog.SpeechProfile, authorization, scope string, input llm.SpeechInput, count int) (usage.UnitBudget, error) {
	ref := p.Binding.Design
	if input.AuxiliaryCharacters > p.Binding.DescriptionMax {
		return usage.UnitBudget{}, usage.ErrUnitPricing
	}
	switch input.Operation {
	case "voice_design":
		if input.InputCharacters > p.Binding.PreviewMax {
			return usage.UnitBudget{}, usage.ErrUnitPricing
		}
	case "voice_confirm":
	case "speech":
		ref = p.Binding.Synthesis
		if input.ParametersDigest != p.Binding.Settings.Digest() {
			return usage.UnitBudget{}, usage.ErrUnitPricing
		}
		if input.InputCharacters > p.Binding.SpeechMax {
			return usage.UnitBudget{}, usage.ErrUnitPricing
		}
	default:
		return usage.UnitBudget{}, usage.ErrUnitPricing
	}
	if ref != input.Ref {
		return usage.UnitBudget{}, usage.ErrUnitPricing
	}
	b := usage.UnitBudget{PolicyID: p.ID, Revision: p.Revision, AuthorizationID: authorization, ScopeDigest: scope, Ref: input.Ref, Operation: input.Operation, InputDigest: input.Digest, Count: count, InputCharacters: input.InputCharacters, AuxiliaryCharacters: input.AuxiliaryCharacters, ParametersDigest: input.ParametersDigest}
	for _, price := range p.Prices {
		if string(price.Operation) != input.Operation {
			continue
		}
		b.Source, b.BoundsSource, b.CheckedAt, b.Complete = price.Source, price.BoundsSource, price.CheckedAt, price.Complete
		for _, c := range price.Charges {
			b.Tariffs = append(b.Tariffs, usage.UnitTariff{Unit: c.Unit, USDPerUnit: c.USDPerUnit, Multiplier: c.Multiplier, MaximumUnits: c.MaximumUnits, UnitsPerInputCharacter: c.UnitsPerInputCharacter})
		}
	}
	if _, err := b.MaximumUSD(); err != nil {
		return usage.UnitBudget{}, err
	}
	return b, nil
}

func (s SpeechBudgets) ValidateUnitBudget(ctx context.Context, owner string, tier plan.Plan, b usage.UnitBudget) error {
	current, err := s.Budget(ctx, owner, tier, b.PolicyID, b.Revision, b.AuthorizationID, b.ScopeDigest, llm.SpeechInput{Ref: b.Ref, Operation: b.Operation, Digest: b.InputDigest, InputCharacters: b.InputCharacters, AuxiliaryCharacters: b.AuxiliaryCharacters, ParametersDigest: b.ParametersDigest, IdentityDigest: b.InputIdentityDigest}, b.Count)
	if err != nil {
		return err
	}
	if b.BoundedInput {
		current.BoundedInput = true
		current.TotalInputCharacters = b.TotalInputCharacters
		current.InputIdentityDigest = b.InputIdentityDigest
	}
	if current.Fingerprint() != b.Fingerprint() {
		return usage.ErrUnitPricing
	}
	return nil
}
