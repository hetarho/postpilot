package store

import (
	"encoding/json"
	"errors"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
)

func encodeSpeech(p modelcatalog.SpeechProfile) (string, string, error) {
	b := p.Binding
	v := b.Settings
	binding, err := json.Marshal(speechBindingRecord{ConnectionScope: b.ConnectionScope, Version: 1, Provider: b.Design.ProviderID, Design: b.Design.ModelID, Synthesis: b.Synthesis.ModelID, Settings: speechSettingsRecord{v.Stability, v.SimilarityBoost, v.Style, v.SpeakerBoost, v.Speed}, Format: b.OutputFormat, DescriptionMax: b.DescriptionMax, PreviewMax: b.PreviewMax, SpeechMax: b.SpeechMax, DesignModel: encodeSpeechModel(b.DesignModel), SpeechModel: encodeSpeechModel(b.SpeechModel)})
	if err != nil {
		return "", "", err
	}
	prices := speechPricesRecord{Version: 1}
	for _, price := range p.Prices {
		r := speechPriceRecord{Operation: string(price.Operation), Source: price.Source, BoundsSource: price.BoundsSource, CheckedAt: formatTime(price.CheckedAt), Complete: price.Complete}
		for _, c := range price.Charges {
			r.Charges = append(r.Charges, speechChargeRecord{string(c.Unit), c.USDPerUnit, c.Multiplier, c.MaximumUnits, c.UnitsPerInputCharacter})
		}
		prices.Prices = append(prices.Prices, r)
	}
	encoded, err := json.Marshal(prices)
	return string(binding), string(encoded), err
}

func encodeSpeechModel(m llm.SpeechModel) speechModelRecord {
	return speechModelRecord{m.Label, m.Design, m.Synthesis, m.Korean, m.Style, m.SpeakerBoost, m.RequiresAlpha, m.MaxText, m.TokenCostFactor, m.CharacterCostMultiplier, m.CostDiscountMultiplier}
}
func decodeSpeechModel(m speechModelRecord, ref llm.ModelRef) llm.SpeechModel {
	return llm.SpeechModel{Ref: ref, Label: m.Label, Design: m.Design, Synthesis: m.Synthesis, Korean: m.Korean, Style: m.Style, SpeakerBoost: m.SpeakerBoost, RequiresAlpha: m.Alpha, MaxText: m.Max, TokenCostFactor: m.Factor, CharacterCostMultiplier: m.Character, CostDiscountMultiplier: m.Discount}
}

func decodeSpeech(bindingJSON, pricesJSON string) (modelcatalog.SpeechBinding, []modelcatalog.SpeechPrice, error) {
	var b speechBindingRecord
	var prices speechPricesRecord
	if err := json.Unmarshal([]byte(bindingJSON), &b); err != nil {
		return modelcatalog.SpeechBinding{}, nil, err
	}
	if err := json.Unmarshal([]byte(pricesJSON), &prices); err != nil {
		return modelcatalog.SpeechBinding{}, nil, err
	}
	if b.Version != 1 || prices.Version != 1 {
		return modelcatalog.SpeechBinding{}, nil, errors.New("unsupported speech snapshot version")
	}
	design, speech := llm.ModelRef{ProviderID: b.Provider, ModelID: b.Design}, llm.ModelRef{ProviderID: b.Provider, ModelID: b.Synthesis}
	v := b.Settings
	bound := modelcatalog.SpeechBinding{ConnectionScope: b.ConnectionScope, Design: design, Synthesis: speech, Settings: llm.SpeechSettings{Stability: v.Stability, SimilarityBoost: v.Similarity, Style: v.Style, SpeakerBoost: v.SpeakerBoost, Speed: v.Speed}, OutputFormat: b.Format, DescriptionMax: b.DescriptionMax, PreviewMax: b.PreviewMax, SpeechMax: b.SpeechMax, DesignModel: decodeSpeechModel(b.DesignModel, design), SpeechModel: decodeSpeechModel(b.SpeechModel, speech)}
	out := make([]modelcatalog.SpeechPrice, 0, len(prices.Prices))
	for _, price := range prices.Prices {
		at, err := parseTime(price.CheckedAt)
		if err != nil {
			return modelcatalog.SpeechBinding{}, nil, err
		}
		p := modelcatalog.SpeechPrice{Operation: modelcatalog.SpeechOperation(price.Operation), Source: price.Source, BoundsSource: price.BoundsSource, CheckedAt: at, Complete: price.Complete}
		for _, c := range price.Charges {
			p.Charges = append(p.Charges, modelcatalog.SpeechCharge{Unit: llm.SpeechUnit(c.Unit), USDPerUnit: c.USD, Multiplier: c.Multiplier, MaximumUnits: c.Maximum, UnitsPerInputCharacter: c.PerCharacter})
		}
		out = append(out, p)
	}
	return bound, out, nil
}
