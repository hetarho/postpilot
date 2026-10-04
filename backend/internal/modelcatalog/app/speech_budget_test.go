package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
)

type budgetProfiles struct {
	profile modelcatalog.SpeechProfile
	owner   string
	tier    plan.Plan
	auth    string
}

func (p *budgetProfiles) ResolveSpeechProfile(_ context.Context, owner string, tier plan.Plan, id string, rev int64, authorization string, _ bool) (modelcatalog.SpeechProfile, error) {
	p.owner, p.tier, p.auth = owner, tier, authorization
	if id != p.profile.ID || rev != p.profile.Revision {
		return modelcatalog.SpeechProfile{}, modelcatalog.ErrSpeechProfileConflict
	}
	return p.profile, nil
}
func budgetProfile() modelcatalog.SpeechProfile {
	return modelcatalog.SpeechProfile{ID: "profile", Revision: 1, Binding: modelcatalog.SpeechBinding{Design: llm.ModelRef{ProviderID: "p", ModelID: "d"}, Synthesis: llm.ModelRef{ProviderID: "p", ModelID: "s"}, DescriptionMax: 100, PreviewMax: 1000, SpeechMax: 1000, Settings: llm.SpeechSettings{Stability: .5, SimilarityBoost: .75, Speed: 1}},
		Prices: []modelcatalog.SpeechPrice{{Operation: modelcatalog.SpeechSynthesize, Source: "https://example.com/prices", BoundsSource: "https://example.com/bounds", CheckedAt: time.Now().Add(-time.Minute), Complete: true,
			Charges: []modelcatalog.SpeechCharge{{Unit: llm.SpeechUnitCharacterCost, USDPerUnit: "0.0001", Multiplier: "1.25", MaximumUnits: "1000", UnitsPerInputCharacter: "1"}}}}}
}
func TestSpeechBudgetMapsQualifiedScheduleAndRefusesBindingOrRateDrift(t *testing.T) {
	p := &budgetProfiles{profile: budgetProfile()}
	maker := SpeechBudgets{Profiles: p}
	r := llm.SpeechRequest{Model: p.profile.Binding.Synthesis, Voice: "private-owner-voice", Text: strings.Repeat("a", 100), Settings: p.profile.Binding.Settings}
	input, _ := r.Input()
	b, err := maker.Budget(t.Context(), "owner", plan.Master, "profile", 1, "qualification", usage.UnitDigest("owner-voice"), input, 1)
	if err != nil || p.owner != "owner" || p.tier != plan.Master || p.auth != "qualification" || b.Tariffs[0].UnitsPerInputCharacter != "1" {
		t.Fatal(b, err, p)
	}
	if err := maker.ValidateUnitBudget(t.Context(), "owner", plan.Master, b); err != nil {
		t.Fatal(err)
	}
	p.profile.Prices[0].Charges[0].Multiplier = "2"
	if err := maker.ValidateUnitBudget(t.Context(), "owner", plan.Master, b); err == nil {
		t.Fatal("changed account schedule accepted")
	}
	p.profile = budgetProfile()
	r.Settings.Style = .2
	wrong, _ := r.Input()
	if _, err := maker.Budget(t.Context(), "owner", plan.Master, "profile", 1, "", b.ScopeDigest, wrong, 1); err == nil {
		t.Fatal("settings changed immutable binding")
	}
	p.profile.Prices[0].Charges[0].UnitsPerInputCharacter = ""
	r.Settings = p.profile.Binding.Settings
	missing, _ := r.Input()
	if _, err := maker.Budget(t.Context(), "owner", plan.Master, "profile", 1, "", b.ScopeDigest, missing, 1); err == nil {
		t.Fatal("unknown conversion accepted")
	}
}
