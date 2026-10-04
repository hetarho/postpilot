package usage

import (
	"math/big"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

func validUnitBudget() UnitBudget {
	return UnitBudget{PolicyID: "profile", Revision: 1, ScopeDigest: UnitDigest("scope"), Ref: llm.ModelRef{ProviderID: "p", ModelID: "d"}, Operation: "voice_design", InputDigest: UnitDigest("input"), Count: 1, InputCharacters: 100,
		Tariffs: []UnitTariff{{Unit: llm.SpeechUnitCharacterCost, USDPerUnit: "0.0001", Multiplier: "1.25", MaximumUnits: "1000", UnitsPerInputCharacter: "1"}}, Source: "https://example.com/prices", BoundsSource: "https://example.com/bounds", CheckedAt: time.Now().Add(-time.Minute), Complete: true}
}

func TestUnitBudgetRequiresVerifiedConversionAndRefusesUnenforceableOutputCost(t *testing.T) {
	for _, mutate := range []func(*UnitBudget){
		func(b *UnitBudget) { b.Tariffs[0].UnitsPerInputCharacter = "" },
		func(b *UnitBudget) { b.Tariffs[0].MaximumUnits = "99" },
		func(b *UnitBudget) { b.Tariffs[0].Unit = llm.SpeechUnitSeconds },
		func(b *UnitBudget) { b.Complete = false },
		func(b *UnitBudget) { b.Count = 101 },
		func(b *UnitBudget) {
			b.Tariffs = append(b.Tariffs, UnitTariff{Unit: llm.SpeechUnitCredits, USDPerUnit: "1", Multiplier: "1", MaximumUnits: "1000", UnitsPerInputCharacter: "1"})
		},
	} {
		b := validUnitBudget()
		mutate(&b)
		if _, err := b.MaximumUSD(); err == nil {
			t.Fatal("unbounded budget accepted", b)
		}
	}
	b := validUnitBudget()
	usd, err := b.MaximumUSD()
	if err != nil || usd.Cmp(big.NewRat(1, 80)) != 0 {
		t.Fatal(usd, err)
	}
	b.Count = 3
	total, err := b.MaximumUSD()
	if err != nil || total.Cmp(big.NewRat(3, 80)) != 0 {
		t.Fatal(total, err)
	}
}

func TestUnitCostUsesOnlyApplicableReportedEvidenceWithoutDoubleCounting(t *testing.T) {
	b := validUnitBudget()
	for _, e := range []llm.SpeechEvidence{{}, {Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacters, Quantity: "100"}}}, {ReportedUSD: "invalid"}} {
		if usd, source := UnitCost(b, e); usd != "" || source != llm.CostUnavailable {
			t.Fatal(usd, source)
		}
	}
	usd, source := UnitCost(b, llm.SpeechEvidence{Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "0.125"}}})
	if usd != "1/64000" || source != llm.CostEstimated {
		t.Fatal(usd, source)
	}
	usd, source = UnitCost(b, llm.SpeechEvidence{ReportedUSD: "0.000000001", Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "100"}}})
	if usd != "1/1000000000" || source != llm.CostReported {
		t.Fatal(usd, source)
	}
	usd, source = UnitCost(b, llm.SpeechEvidence{Units: []llm.SpeechUnitEvidence{{Unit: llm.SpeechUnitCharacterCost, Quantity: "0"}}})
	if usd != "0" || source != llm.CostEstimated {
		t.Fatal(usd, source)
	}
}

func TestExactUnitRoundingAggregatesBeforeProductCredits(t *testing.T) {
	rate := plan.RateSnapshot{Source: "official", PublicationDate: "2026-10-02", ReferenceE4: 14_800_000, AppliedE4: 14_800_000}
	one, _ := new(big.Rat).SetString("0.0003378375")
	combined := new(big.Rat).Mul(one, big.NewRat(2, 1))
	credits, err := exactCredits(combined, rate)
	if err != nil || credits != 1 {
		t.Fatal(credits, err)
	}
	if _, err := exactCredits(big.NewRat(1, 1), plan.RateSnapshot{}); err == nil {
		t.Fatal("missing rate accepted")
	}
}
