package modelcatalog

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type speechMemory struct {
	profiles map[string][]SpeechProfile
	sessions map[string]SpeechQualificationSession
}

func newSpeechMemory() *speechMemory {
	return &speechMemory{map[string][]SpeechProfile{}, map[string]SpeechQualificationSession{}}
}
func (m *speechMemory) ListSpeechProfiles(context.Context) ([]SpeechProfile, error) {
	out := []SpeechProfile{}
	for _, ps := range m.profiles {
		out = append(out, cloneSpeechProfile(ps[len(ps)-1]))
	}
	return out, nil
}
func (m *speechMemory) GetSpeechRevision(_ context.Context, id string, rev int64) (SpeechProfile, error) {
	for _, p := range m.profiles[id] {
		if p.Revision == rev {
			return cloneSpeechProfile(p), nil
		}
	}
	return SpeechProfile{}, ErrNotFound
}
func (m *speechMemory) SaveSpeechRevision(_ context.Context, p SpeechProfile, expected int64) (SpeechProfile, error) {
	ps := m.profiles[p.ID]
	if int64(len(ps)) != expected {
		return SpeechProfile{}, ErrSpeechProfileConflict
	}
	m.profiles[p.ID] = append(ps, cloneSpeechProfile(p))
	return cloneSpeechProfile(p), nil
}
func (m *speechMemory) RecordSpeechReadiness(_ context.Context, id string, rev int64, evidence string, export bool) error {
	ps := m.profiles[id]
	if len(ps) == 0 || ps[len(ps)-1].Revision != rev {
		return ErrSpeechProfileConflict
	}
	p := &ps[len(ps)-1]
	if export {
		p.ExportEvidence = evidence
	} else {
		p.VoiceEvidence = evidence
	}
	return nil
}
func (m *speechMemory) CreateSpeechQualification(_ context.Context, q SpeechQualificationSession) error {
	m.sessions[q.ID] = q
	return nil
}
func (m *speechMemory) GetSpeechQualification(_ context.Context, owner, id string) (SpeechQualificationSession, error) {
	q, ok := m.sessions[id]
	if !ok || q.OwnerID != owner {
		return SpeechQualificationSession{}, ErrSpeechQualificationInvalid
	}
	return q, nil
}
func cloneSpeechProfile(p SpeechProfile) SpeechProfile {
	p.Prices = slices.Clone(p.Prices)
	for i := range p.Prices {
		p.Prices[i].Charges = slices.Clone(p.Prices[i].Charges)
	}
	return p
}

type speechSource struct {
	catalog    llm.SpeechCatalog
	connection llm.SpeechConnection
	err        error
}

func (s *speechSource) ReadSpeechCatalog(context.Context, bool) (llm.SpeechCatalog, error) {
	c := s.catalog
	c.Models = slices.Clone(c.Models)
	return c, s.err
}
func (s *speechSource) SpeechConnection() llm.SpeechConnection { return s.connection }

func speechFixture(t *testing.T) (*SpeechService, *speechMemory, *speechSource, SpeechProfile) {
	t.Helper()
	store := newSpeechMemory()
	source := &speechSource{connection: llm.SpeechConnection{ProviderID: "speech-test"}, catalog: llm.SpeechCatalog{Models: []llm.SpeechModel{
		{Ref: llm.ModelRef{ProviderID: "speech-test", ModelID: "design"}, Label: "Design", Design: true, MaxText: 1000},
		{Ref: llm.ModelRef{ProviderID: "speech-test", ModelID: "synth"}, Label: "Synthesis", Synthesis: true, Korean: true, Style: true, SpeakerBoost: true, MaxText: 5000, TokenCostFactor: "1", CharacterCostMultiplier: "1", CostDiscountMultiplier: "1"},
	}}}
	service := NewSpeechService(store, source)
	at := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return at }
	p := SpeechProfile{Label: "Korean voice", Level: LevelValue, Enabled: true, Binding: SpeechBinding{Design: source.catalog.Models[0].Ref, Synthesis: source.catalog.Models[1].Ref, Settings: llm.SpeechSettings{Stability: 0.5, SimilarityBoost: 0.75, Speed: 1}, OutputFormat: llm.SpeechOutputFormat, DescriptionMax: 1000, PreviewMax: 1000, SpeechMax: 1000}}
	for _, op := range SpeechOperations {
		p.Prices = append(p.Prices, SpeechPrice{Operation: op, Source: "https://example.com/account-prices", BoundsSource: "https://example.com/bounds", CheckedAt: at, Complete: true, Charges: []SpeechCharge{{Unit: llm.SpeechUnitCharacterCost, USDPerUnit: "0.000100001", Multiplier: "1.25", MaximumUnits: "1000"}}})
	}
	return service, store, source, p
}

func TestSpeechProfilesPreserveRevisionsAndInvalidateReadiness(t *testing.T) {
	s, store, source, p := speechFixture(t)
	ctx := context.Background()
	first, err := s.SaveSpeechProfile(ctx, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.ID == "" {
		t.Fatalf("identity: %+v", first)
	}
	choices, err := s.SpeechChoices(ctx, plan.Master)
	if err != nil || choices[0].Available || choices[0].UnavailableReason != "SPEECH_NOT_QUALIFIED" {
		t.Fatalf("unqualified: %+v %v", choices, err)
	}
	q, err := s.StartSpeechQualification(ctx, "owner", first.ID, first.Revision, "1.5")
	if err != nil {
		t.Fatal(err)
	}
	e := SpeechQualificationEvidence{SessionID: q.ID, OwnerID: "owner", ReportID: "private-live-report", DesignRequestID: "design-1", ConfirmRequestID: "confirm-1", SpeechRequestIDs: []string{"script-1", "script-2", "script-3"}, ConfirmedVoiceID: "private-voice", AuditionAccepted: true, KoreanAccepted: true, ContinuityAccepted: true, UsageVerified: true}
	if err := s.RecordVoiceQualification(ctx, e); err != nil {
		t.Fatal(err)
	}
	choices, _ = s.SpeechChoices(ctx, plan.Light)
	if !choices[0].Available || !choices[0].VoiceReady || choices[0].ExportReady {
		t.Fatalf("voice readiness: %+v", choices)
	}
	if _, err := s.ResolveSpeechProfile(ctx, "owner", plan.Light, first.ID, 1, "", true); !errors.Is(err, ErrSpeechProfileUnavailable) {
		t.Fatalf("export unqualified: %v", err)
	}
	changed := first
	changed.Level = LevelPremium
	changed.Prices = cloneSpeechProfile(first).Prices
	changed.Prices[0].Charges[0].USDPerUnit = "0.0002"
	second, err := s.SaveSpeechProfile(ctx, changed, 1)
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != 2 || second.VoiceEvidence != "" {
		t.Fatalf("new revision: %+v", second)
	}
	old, _ := store.GetSpeechRevision(ctx, first.ID, 1)
	if old.Prices[0].Charges[0].USDPerUnit != "0.000100001" || old.VoiceEvidence == "" {
		t.Fatalf("history changed: %+v", old)
	}
	if _, err := s.SaveSpeechProfile(ctx, first, 1); !errors.Is(err, ErrSpeechProfileConflict) {
		t.Fatalf("stale write: %v", err)
	}
	if _, err := s.ResolveSpeechProfile(ctx, "owner", plan.Master, first.ID, 1, q.ID, false); !errors.Is(err, ErrSpeechProfileConflict) {
		t.Fatalf("old quote: %v", err)
	}
	source.connection.Disabled = true
	choices, _ = s.SpeechChoices(ctx, plan.Master)
	if choices[0].UnavailableReason != "SPEECH_CONNECTION_UNAVAILABLE" {
		t.Fatalf("disabled: %+v", choices)
	}
	if _, err := store.GetSpeechRevision(ctx, first.ID, 1); err != nil {
		t.Fatal("missing key erased historical binding")
	}
}

func TestSpeechCapabilityAndPriceDrift(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*speechSource)
		reason string
	}{
		{"Korean lost", func(s *speechSource) { s.catalog.Models[1].Korean = false }, "SPEECH_PATH_UNSUPPORTED"},
		{"limit lost", func(s *speechSource) { s.catalog.Models[1].MaxText = 0 }, "SPEECH_PATH_UNSUPPORTED"},
		{"factor changed", func(s *speechSource) { s.catalog.Models[1].CharacterCostMultiplier = "2" }, "SPEECH_PROFILE_CHANGED"},
		{"model removed", func(s *speechSource) { s.catalog.Models = s.catalog.Models[:1] }, "SPEECH_PATH_UNSUPPORTED"},
		{"fetch failed", func(s *speechSource) { s.err = errors.New("supplier private balance") }, "SPEECH_CATALOG_UNAVAILABLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, source, p := speechFixture(t)
			ctx := context.Background()
			saved, err := s.SaveSpeechProfile(ctx, p, 0)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(source)
			choices, err := s.SpeechChoices(ctx, plan.Master)
			if err != nil || choices[0].Available || choices[0].UnavailableReason != tc.reason {
				t.Fatalf("%+v %v", choices, err)
			}
			if _, err := s.StartSpeechQualification(ctx, "owner", saved.ID, 1, "5"); err == nil {
				t.Fatal("unsafe qualification allowed")
			}
		})
	}
	s, _, source, p := speechFixture(t)
	ctx := context.Background()
	saved, _ := s.SaveSpeechProfile(ctx, p, 0)
	source.catalog.Models[1].CharacterCostMultiplier = "2"
	revised, err := s.SaveSpeechProfile(ctx, saved, 1)
	if err != nil || revised.Binding.SpeechModel.CharacterCostMultiplier != "2" {
		t.Fatalf("explicit drift adoption: %+v %v", revised, err)
	}
}

func TestSpeechFreeRequiresEveryKnownZeroPrice(t *testing.T) {
	s, _, _, p := speechFixture(t)
	ctx := context.Background()
	p.Level = LevelFree
	if _, err := s.SaveSpeechProfile(ctx, p, 0); !errors.Is(err, ErrFreeIneligible) {
		t.Fatalf("paid free classification: %v", err)
	}
	for i := range p.Prices {
		p.Prices[i].Charges[0].USDPerUnit = "0"
	}
	missing := cloneSpeechProfile(p)
	missing.Prices = missing.Prices[:2]
	if _, err := s.SaveSpeechProfile(ctx, missing, 0); !errors.Is(err, ErrFreeIneligible) {
		t.Fatalf("unknown confirmation price accepted: %v", err)
	}
	saved, err := s.SaveSpeechProfile(ctx, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartSpeechQualification(ctx, "owner", saved.ID, 1, "0"); err != nil {
		t.Fatal(err)
	}
	paid := p
	paid.Level = LevelValue
	paid.Prices = nil
	saved, err = s.SaveSpeechProfile(ctx, paid, 0)
	if err != nil {
		t.Fatal("unpriced paid draft should be curatable:", err)
	}
	choices, _ := s.SpeechChoices(ctx, plan.Master)
	var found SpeechChoice
	for _, c := range choices {
		if c.ID == saved.ID {
			found = c
		}
	}
	if found.UnavailableReason != "SPEECH_PRICE_UNAVAILABLE" {
		t.Fatalf("missing price: %+v", found)
	}
}

func TestSpeechQualificationIsOwnerScopedFiniteAndNeverAQueryFlag(t *testing.T) {
	s, _, _, p := speechFixture(t)
	ctx := context.Background()
	saved, _ := s.SaveSpeechProfile(ctx, p, 0)
	if _, err := s.ResolveSpeechProfile(ctx, "owner", plan.Master, saved.ID, 1, "true", false); !errors.Is(err, ErrSpeechQualificationInvalid) {
		t.Fatalf("flag bypass: %v", err)
	}
	q, err := s.StartSpeechQualification(ctx, "owner", saved.ID, 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		owner string
		tier  plan.Plan
	}{{"other", plan.Master}, {"owner", plan.Pro}} {
		if _, err := s.ResolveSpeechProfile(ctx, tc.owner, tc.tier, saved.ID, 1, q.ID, false); !errors.Is(err, ErrSpeechQualificationInvalid) {
			t.Fatalf("scope bypass: %v", err)
		}
	}
	if _, err := s.ResolveSpeechProfile(ctx, "owner", plan.Master, saved.ID, 1, q.ID, false); err != nil {
		t.Fatal(err)
	}
	choices, err := s.QualificationSpeechChoices(ctx, "owner", plan.Master, q.ID)
	if err != nil || len(choices) != 1 || !choices[0].Available || choices[0].VoiceReady || choices[0].ExportReady {
		t.Fatalf("provisional owner picker: %+v %v", choices, err)
	}
	if _, err := s.QualificationSpeechChoices(ctx, "owner", plan.Pro, q.ID); !errors.Is(err, ErrSpeechQualificationInvalid) {
		t.Fatalf("ordinary provisional picker: %v", err)
	}
	if err := s.RecordVoiceQualification(ctx, SpeechQualificationEvidence{SessionID: q.ID, OwnerID: "owner", ReportID: "fixture"}); !errors.Is(err, ErrSpeechQualificationInvalid) {
		t.Fatalf("fixture promoted readiness: %v", err)
	}
	later := s.now().Add(SpeechQualificationTTL)
	s.now = func() time.Time { return later }
	if _, err := s.ResolveSpeechProfile(ctx, "owner", plan.Master, saved.ID, 1, q.ID, false); !errors.Is(err, ErrSpeechQualificationInvalid) {
		t.Fatalf("expiry bypass: %v", err)
	}
}

func TestSpeechPriceRejectsUnknownAndOverlappingEvidence(t *testing.T) {
	s, _, _, p := speechFixture(t)
	base := p.Prices[0]
	for _, value := range []string{"", "-1", "NaN", "Infinity", "1e-3", "0.0000000001"} {
		v := base
		v.Charges = slices.Clone(base.Charges)
		v.Charges[0].USDPerUnit = value
		if v.Valid(s.now()) {
			t.Fatalf("invalid decimal accepted: %q", value)
		}
	}
	v := base
	v.Charges = append(slices.Clone(base.Charges), SpeechCharge{Unit: llm.SpeechUnitCharacters, USDPerUnit: "1", Multiplier: "1", MaximumUnits: "1000"})
	if v.Valid(s.now()) {
		t.Fatal("overlapping variable tariff accepted")
	}
	v.Charges[1].Unit = llm.SpeechUnitRequests
	if !v.Valid(s.now()) {
		t.Fatal("documented extra request charge refused")
	}
	before := cloneSpeechProfile(p)
	if !reflect.DeepEqual(before, p) {
		t.Fatal("fixture clone changed price spelling")
	}
}
