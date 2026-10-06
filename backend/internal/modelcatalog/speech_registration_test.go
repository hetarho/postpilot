package modelcatalog

import (
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"reflect"
	"testing"
)

func catalogFixture(t *testing.T) (*SpeechService, *speechMemory, *speechSource, SpeechRegistration) {
	t.Helper()
	s, m, source, p := speechFixture(t)
	source.catalog.BillingRules = []llm.SpeechBillingRule{
		{Ref: p.Binding.Design, Operation: "voice_design", Unit: llm.SpeechUnitCharacterCost, UnitsPerInputCharacter: "1", Source: "https://example.com/design-bounds"},
		{Ref: p.Binding.Synthesis, Operation: "speech", Unit: llm.SpeechUnitCharacterCost, UnitsPerInputCharacter: "1", Source: "https://example.com/speech-bounds"},
	}
	return s, m, source, SpeechRegistration{Design: p.Binding.Design, Synthesis: p.Binding.Synthesis, Enabled: true}
}
func accountTariff() SpeechAccountTariff {
	return SpeechAccountTariff{DesignUSDPerUnit: "0.000100001", SpeechUSDPerUnit: "0.000000001", ConfirmationUSD: "0", Source: "https://example.com/account-prices", Complete: true}
}
func TestSpeechCatalogRegistrationResolvesDefaultsAndRequiresNoPriceOrGrade(t *testing.T) {
	s, m, source, r := catalogFixture(t)
	source.catalog.Models[1].MaxText = 600
	p, err := s.RegisterSpeechCombination(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Level != "" || !p.CatalogManaged || len(p.Prices) != 0 || p.Label != "Design → Synthesis" || p.Binding.SpeechMax != 600 || p.Binding.Settings.Speed != 1 || p.Binding.Settings.SpeakerBoost || p.Binding.Settings.Stability != SpeechDefaultStability {
		t.Fatalf("defaults: %+v", p)
	}
	choices, err := s.SpeechChoices(t.Context(), plan.Master)
	if err != nil || choices[0].Available || choices[0].UnavailableReason != "MODEL_UNCLASSIFIED" {
		t.Fatal(choices, err)
	}
	if _, err = s.RegisterSpeechCombination(t.Context(), r); !errors.Is(err, ErrSpeechProfileConflict) {
		t.Fatal("duplicate pair accepted", err)
	}
	r.ID, r.ExpectedRevision, r.Level = p.ID, p.Revision, LevelValue
	second, err := s.RegisterSpeechCombination(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RegisterSpeechCombination(t.Context(), r); !errors.Is(err, ErrSpeechProfileConflict) {
		t.Fatal("stale revision accepted", err)
	}
	old, _ := m.GetSpeechRevision(t.Context(), p.ID, p.Revision)
	if !reflect.DeepEqual(old, p) || second.Revision != 2 {
		t.Fatal("historical registration changed")
	}
	choices, _ = s.SpeechChoices(t.Context(), plan.Master)
	if choices[0].UnavailableReason != "SPEECH_PRICE_UNAVAILABLE" {
		t.Fatal(choices)
	}
}
func TestCommonSpeechTariffRefreshesPricesWithoutMutatingSoundOrHistory(t *testing.T) {
	s, m, _, r := catalogFixture(t)
	r.Level = LevelValue
	p, err := s.RegisterSpeechCombination(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	tariff, err := s.SaveSpeechAccountTariff(t.Context(), accountTariff(), 0)
	if err != nil {
		t.Fatal(err)
	}
	current, _ := s.currentSpeechProfile(t.Context(), p.ID)
	if current.Revision != 2 || current.TariffRevision != 1 || current.Binding != p.Binding || !current.PricingReady(s.now()) {
		t.Fatalf("common projection: %+v", current)
	}
	for _, price := range current.Prices {
		if price.Operation == SpeechSynthesize && (price.Charges[0].USDPerUnit != "0.000000001" || price.Charges[0].MaximumUnits != "1000" || price.Charges[0].Multiplier != "1") {
			t.Fatal(price)
		}
	}
	if _, err = s.SaveSpeechAccountTariff(t.Context(), accountTariff(), 0); !errors.Is(err, ErrSpeechProfileConflict) {
		t.Fatal(err)
	}
	if _, err = s.ResolveSpeechProfile(t.Context(), "owner", plan.Master, p.ID, p.Revision, "", false); !errors.Is(err, ErrSpeechProfileConflict) {
		t.Fatal("old profile still admitted", err)
	}
	old, _ := m.GetSpeechRevision(t.Context(), p.ID, p.Revision)
	if !reflect.DeepEqual(old, p) {
		t.Fatal("history changed")
	}
	m.RecordSpeechReadiness(t.Context(), current.ID, current.Revision, "reviewed", false)
	changed := accountTariff()
	changed.SpeechUSDPerUnit = "0.0002"
	if _, err = s.SaveSpeechAccountTariff(t.Context(), changed, tariff.Revision); err != nil {
		t.Fatal(err)
	}
	current, _ = s.currentSpeechProfile(t.Context(), p.ID)
	if current.Revision != 3 || current.VoiceEvidence != "" || current.ExportEvidence != "" {
		t.Fatal("tariff kept readiness", current)
	}
}
func TestSpeechTariffRequiresExplicitConfirmationAndAccountEvidence(t *testing.T) {
	for _, change := range []func(*SpeechAccountTariff){func(t *SpeechAccountTariff) { t.ConfirmationUSD = "" }, func(t *SpeechAccountTariff) { t.Complete = false }, func(t *SpeechAccountTariff) { t.Source = "http://example.com" }, func(t *SpeechAccountTariff) { t.SpeechUSDPerUnit = "NaN" }} {
		s, m, _, _ := catalogFixture(t)
		draft := accountTariff()
		change(&draft)
		if _, err := s.SaveSpeechAccountTariff(t.Context(), draft, 0); !errors.Is(err, ErrSpeechProfileInvalid) {
			t.Fatal(err)
		}
		if m.tariff.Revision != 0 {
			t.Fatal("invalid evidence written")
		}
	}
}
func TestMissingBillingRulesAndAccountRotationCannotAuthorizePaidSpeech(t *testing.T) {
	s, m, source, r := catalogFixture(t)
	r.Level = LevelValue
	source.catalog.BillingRules = source.catalog.BillingRules[:1]
	s.SaveSpeechAccountTariff(t.Context(), accountTariff(), 0)
	p, err := s.RegisterSpeechCombination(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p.PricingReady(s.now()) {
		t.Fatal("unknown synthesis units priced")
	}
	source.catalog.ConnectionScope = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	r.ID, r.ExpectedRevision = p.ID, p.Revision
	next, err := s.RegisterSpeechCombination(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Prices) != 0 || next.PricingReady(s.now()) {
		t.Fatal("another account tariff inherited")
	}
	m.tariff.Revision++
	if reason := s.readiness(t.Context(), next, source.catalog, nil); reason != "SPEECH_PRICE_UNAVAILABLE" {
		t.Fatal(reason)
	}
}
func TestSpeechCatalogUnsupportedStyleAndOfflineWithdrawal(t *testing.T) {
	s, _, source, r := catalogFixture(t)
	source.catalog.Models[1].Style = false
	r.Adjustments = &llm.SpeechSettings{Stability: .4, SimilarityBoost: .8, Style: .2}
	if _, err := s.RegisterSpeechCombination(t.Context(), r); !errors.Is(err, ErrSpeechProfileInvalid) {
		t.Fatal("unsupported style accepted", err)
	}
	r.Adjustments = nil
	p, err := s.RegisterSpeechCombination(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	source.err = errors.New("supplier offline")
	r.ID, r.ExpectedRevision, r.Enabled = p.ID, p.Revision, false
	updated, err := s.RegisterSpeechCombination(t.Context(), r)
	if err != nil || updated.Enabled || updated.Binding != p.Binding {
		t.Fatal(updated, err)
	}
	browse, err := s.BrowseSpeech(t.Context(), false)
	if err != nil || len(browse.Profiles) != 1 || len(browse.Combinations) != 0 || browse.FetchError != "SPEECH_CATALOG_UNAVAILABLE" {
		t.Fatal(browse, err)
	}
}

func TestCommonSpeechPricingAdoptsLegacyRegistrationWithoutRebindingItsVoice(t *testing.T) {
	s, m, _, r := catalogFixture(t)
	_, _, _, p := speechFixture(t)
	p.Binding.Design, p.Binding.Synthesis = r.Design, r.Synthesis
	saved, err := s.SaveSpeechProfile(t.Context(), p, 0)
	if err != nil {
		t.Fatal(err)
	}
	m.RecordSpeechReadiness(t.Context(), saved.ID, 1, "original-listening-proof", false)
	if _, err := s.SaveSpeechAccountTariff(t.Context(), accountTariff(), 0); err != nil {
		t.Fatal(err)
	}
	current, _ := s.currentSpeechProfile(t.Context(), saved.ID)
	if !current.CatalogManaged || current.TariffRevision != 1 || current.Binding != saved.Binding || current.Label != saved.Label || current.Level != saved.Level {
		t.Fatalf("legacy adoption: %+v", current)
	}
	original, _ := m.GetSpeechRevision(t.Context(), saved.ID, 1)
	if original.VoiceEvidence != "original-listening-proof" || original.Binding != saved.Binding {
		t.Fatal("legacy history changed")
	}
}
