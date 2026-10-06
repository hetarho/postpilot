package modelcatalog

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const (
	SpeechDefaultStability  = 0.5
	SpeechDefaultSimilarity = 0.75
)

func productSpeechBinding(d, v llm.SpeechModel) SpeechBinding {
	return SpeechBinding{Design: d.Ref, Synthesis: v.Ref,
		Settings:     llm.SpeechSettings{Stability: SpeechDefaultStability, SimilarityBoost: SpeechDefaultSimilarity, Speed: 1},
		OutputFormat: llm.SpeechOutputFormat, DescriptionMax: llm.SpeechDescriptionMax,
		PreviewMax: min(llm.SpeechPreviewMax, d.MaxText), SpeechMax: min(llm.SpeechMaxText, v.MaxText),
		DesignModel: d, SpeechModel: v}
}

func (s *SpeechService) RegisterSpeechCombination(ctx context.Context, r SpeechRegistration) (SpeechProfile, error) {
	if _, err := ParseLevel(string(r.Level)); err != nil {
		return SpeechProfile{}, err
	}
	if r.Design.ProviderID == "" || r.Design.ProviderID != r.Synthesis.ProviderID {
		return SpeechProfile{}, ErrSpeechProfileInvalid
	}
	id, err := s.store.GetSpeechCombination(ctx, r.Design, r.Synthesis)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return SpeechProfile{}, err
	}
	if id != r.ID || (id == "" && r.ExpectedRevision != 0) {
		return SpeechProfile{}, ErrSpeechProfileConflict
	}
	var p SpeechProfile
	if id != "" {
		p, err = s.store.GetSpeechRevision(ctx, id, r.ExpectedRevision)
		if err != nil {
			return SpeechProfile{}, err
		}
		if p.Binding.Design != r.Design || p.Binding.Synthesis != r.Synthesis {
			return SpeechProfile{}, ErrSpeechProfileInvalid
		}
	}
	catalog, fetchErr := s.source.ReadSpeechCatalog(ctx, false)
	if fetchErr == nil {
		d, dok := speechCandidate(catalog, r.Design, true)
		v, vok := speechCandidate(catalog, r.Synthesis, false)
		if !dok || !vok {
			return SpeechProfile{}, ErrSpeechProfileInvalid
		}
		b := productSpeechBinding(d, v)
		if !validSpeechCapabilities(b, d, v) {
			return SpeechProfile{}, ErrSpeechProfileInvalid
		}
		if id == "" {
			p.Label = speechCombinationLabel(d, v)
		} else {
			b.Settings = p.Binding.Settings
		}
		if !v.Style {
			b.Settings.Style = 0
		}
		// Non-adjustable product settings are resolved on every successful curation.
		b.Settings.SpeakerBoost, b.Settings.Speed = false, 1
		p.Binding = b
	} else if id == "" {
		return SpeechProfile{}, ErrSpeechProfileUnavailable
	}
	if r.Adjustments != nil {
		p.Binding.Settings.Stability = r.Adjustments.Stability
		p.Binding.Settings.SimilarityBoost = r.Adjustments.SimilarityBoost
		p.Binding.Settings.Style = r.Adjustments.Style
	}
	p.ID, p.Level, p.Enabled = id, r.Level, r.Enabled
	if fetchErr == nil {
		p.CatalogManaged = true
	}
	if fetchErr == nil {
		tariff, err := s.store.GetSpeechTariff(ctx)
		if err != nil {
			return SpeechProfile{}, err
		}
		p.Prices = resolveSpeechPrices(tariff, catalog, p.Binding)
		p.TariffRevision = tariff.Revision
	}
	return s.saveSpeechProfile(ctx, p, r.ExpectedRevision)
}

func speechCombinationLabel(d, v llm.SpeechModel) string {
	// Labels are metadata; ensure a supplier label cannot exceed the profile cap.
	r := []rune(d.Label + " → " + v.Label)
	return string(r[:min(len(r), SpeechProfileLabelMax)])
}

func speechCombinations(c llm.SpeechCatalog, t SpeechAccountTariff) []SpeechProfile {
	var out []SpeechProfile
	for _, d := range c.Models {
		if !d.Design {
			continue
		}
		for _, v := range c.Models {
			if d.Ref.ProviderID != v.Ref.ProviderID {
				continue
			}
			b := productSpeechBinding(d, v)
			if !validSpeechCapabilities(b, d, v) {
				continue
			}
			b.ConnectionScope = c.ConnectionScope
			out = append(out, SpeechProfile{Label: speechCombinationLabel(d, v), Binding: b, CatalogManaged: true,
				TariffRevision: t.Revision, Prices: resolveSpeechPrices(t, c, b)})
		}
	}
	return out
}

func (s *SpeechService) SaveSpeechAccountTariff(ctx context.Context, t SpeechAccountTariff, expected int64) (SpeechAccountTariff, error) {
	if !t.Complete || !validSpeechSource(t.Source) {
		return SpeechAccountTariff{}, ErrSpeechProfileInvalid
	}
	for _, v := range []string{t.DesignUSDPerUnit, t.SpeechUSDPerUnit, t.ConfirmationUSD} {
		if _, ok := SpeechDecimal(v); !ok {
			return SpeechAccountTariff{}, ErrSpeechProfileInvalid
		}
	}
	c, err := s.source.ReadSpeechCatalog(ctx, false)
	if err != nil || len(c.ConnectionScope) != 64 {
		return SpeechAccountTariff{}, ErrSpeechProfileUnavailable
	}
	current, err := s.store.GetSpeechTariff(ctx)
	if err != nil {
		return SpeechAccountTariff{}, err
	}
	if current.Revision != expected {
		return SpeechAccountTariff{}, ErrSpeechProfileConflict
	}
	t.Revision, t.ConnectionScope, t.CheckedAt = expected+1, c.ConnectionScope, s.now().UTC()
	profiles, err := s.store.ListSpeechProfiles(ctx)
	if err != nil {
		return SpeechAccountTariff{}, err
	}
	var updates []SpeechProfile
	for _, p := range profiles {
		owner, err := s.store.GetSpeechCombination(ctx, p.Binding.Design, p.Binding.Synthesis)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return SpeechAccountTariff{}, err
		}
		if owner != p.ID {
			continue
		}
		p.CatalogManaged = true
		p.Prices = resolveSpeechPrices(t, c, p.Binding)
		if p.Level == LevelFree && !p.ZeroPriced(s.now()) {
			return SpeechAccountTariff{}, ErrFreeIneligible
		}
		p.Revision++
		p.TariffRevision, p.CreatedAt = t.Revision, t.CheckedAt
		p.VoiceEvidence, p.ExportEvidence = "", ""
		updates = append(updates, p)
	}
	if err := s.store.SaveSpeechTariff(ctx, t, expected, updates); err != nil {
		return SpeechAccountTariff{}, err
	}
	return t, nil
}

func resolveSpeechPrices(t SpeechAccountTariff, c llm.SpeechCatalog, b SpeechBinding) []SpeechPrice {
	if t.Revision == 0 || !t.Complete || t.ConnectionScope != c.ConnectionScope || b.ConnectionScope != "" && b.ConnectionScope != c.ConnectionScope {
		return nil
	}
	var out []SpeechPrice
	for _, rule := range c.BillingRules {
		var usd string
		var maximum int
		switch {
		case rule.Operation == string(SpeechDesign) && rule.Ref == b.Design:
			usd, maximum = t.DesignUSDPerUnit, b.PreviewMax
		case rule.Operation == string(SpeechSynthesize) && rule.Ref == b.Synthesis:
			usd, maximum = t.SpeechUSDPerUnit, b.SpeechMax
		default:
			continue
		}
		ratio, ok := SpeechDecimal(rule.UnitsPerInputCharacter)
		if !ok || ratio.Sign() <= 0 || !validSpeechSource(rule.Source) || rule.Unit != llm.SpeechUnitCharacterCost {
			continue
		}
		maxUnits := new(big.Rat).Mul(ratio, new(big.Rat).SetInt64(int64(maximum)))
		// Existing pricing accepts decimal strings, so keep an exact finite value.
		maxDecimal := strings.TrimRight(strings.TrimRight(maxUnits.FloatString(9), "0"), ".")
		out = append(out, SpeechPrice{Operation: SpeechOperation(rule.Operation), Source: t.Source, BoundsSource: rule.Source,
			CheckedAt: t.CheckedAt, Complete: true, Charges: []SpeechCharge{{Unit: rule.Unit, USDPerUnit: usd, Multiplier: "1", MaximumUnits: maxDecimal, UnitsPerInputCharacter: rule.UnitsPerInputCharacter}}})
	}
	// Confirmation is never assumed free: its explicit, verified account term is required.
	out = append(out, SpeechPrice{Operation: SpeechConfirm, Source: t.Source, BoundsSource: t.Source,
		CheckedAt: t.CheckedAt, Complete: true, Charges: []SpeechCharge{{Unit: llm.SpeechUnitRequests, USDPerUnit: t.ConfirmationUSD, Multiplier: "1", MaximumUnits: "1"}}})
	return out
}
