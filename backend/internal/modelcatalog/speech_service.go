package modelcatalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type SpeechService struct {
	store  SpeechProfileStore
	source SpeechCatalogSource
	now    func() time.Time
}

func NewSpeechService(store SpeechProfileStore, source SpeechCatalogSource) *SpeechService {
	return &SpeechService{store: store, source: source, now: time.Now}
}

func (s *SpeechService) BrowseSpeech(ctx context.Context, refresh bool) (SpeechAdminBrowse, error) {
	profiles, err := s.store.ListSpeechProfiles(ctx)
	if err != nil {
		return SpeechAdminBrowse{}, err
	}
	browse := SpeechAdminBrowse{Profiles: profiles}
	var catalog llm.SpeechCatalog
	connection := s.source.SpeechConnection()
	switch {
	case connection.ProviderID == "":
		browse.FetchError = "SPEECH_PROVIDER_NOT_CONFIGURED"
		err = llm.ErrProviderDisabled
	case connection.Disabled:
		browse.FetchError = "SPEECH_CONNECTION_UNAVAILABLE"
		if connection.DisabledReason == llm.DisabledReasonNoKey {
			browse.FetchError = "SPEECH_API_KEY_NOT_CONFIGURED"
		}
		err = llm.ErrProviderDisabled
	default:
		catalog, err = s.source.ReadSpeechCatalog(ctx, refresh)
		if err != nil {
			browse.FetchError = "SPEECH_CATALOG_UNAVAILABLE"
		}
	}
	browse.Candidates = catalog.Models
	for _, p := range profiles {
		browse.Choices = append(browse.Choices, s.choice(p, plan.Master, catalog, err))
	}
	return browse, nil
}

func (s *SpeechService) SpeechChoices(ctx context.Context, tier plan.Plan) ([]SpeechChoice, error) {
	profiles, err := s.store.ListSpeechProfiles(ctx)
	if err != nil {
		return nil, err
	}
	catalog, fetchErr := s.source.ReadSpeechCatalog(ctx, false)
	out := make([]SpeechChoice, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, s.choice(p, tier, catalog, fetchErr))
	}
	return out, nil
}

func (s *SpeechService) QualificationSpeechChoices(ctx context.Context, owner string, tier plan.Plan, session string) ([]SpeechChoice, error) {
	if tier != plan.Master {
		return nil, ErrSpeechQualificationInvalid
	}
	q, err := s.SpeechQualification(ctx, owner, session)
	if err != nil {
		return nil, err
	}
	p, err := s.ResolveSpeechProfile(ctx, owner, tier, q.ProfileID, q.Revision, session, false)
	if err != nil {
		return nil, err
	}
	catalog, fetchErr := s.source.ReadSpeechCatalog(ctx, false)
	c := s.choice(p, tier, catalog, fetchErr)
	// Only live readiness is provisional; every other compatibility/price gate
	// has already passed. The live qualification flags remain false.
	if c.UnavailableReason == "SPEECH_NOT_QUALIFIED" {
		c.Available, c.UnavailableReason = true, ""
	}
	return []SpeechChoice{c}, nil
}

func (s *SpeechService) choice(p SpeechProfile, tier plan.Plan, catalog llm.SpeechCatalog, fetchErr error) SpeechChoice {
	required, entitled := plan.AllowsModelGrade(tier, string(p.Level))
	out := SpeechChoice{ID: p.ID, Revision: p.Revision, Label: p.Label, Design: p.Binding.Design, DesignLabel: p.Binding.DesignModel.Label,
		Synthesis: p.Binding.Synthesis, SynthesisLabel: p.Binding.SpeechModel.Label, Level: p.Level, RequiredPlan: required, Entitled: entitled,
		DescriptionMax: p.Binding.DescriptionMax, PreviewMin: llm.SpeechPreviewMin, PreviewMax: p.Binding.PreviewMax, SpeechMax: p.Binding.SpeechMax,
		VoiceReady: p.VoiceEvidence != "", ExportReady: p.ExportEvidence != ""}
	out.UnavailableReason = s.readiness(p, catalog, fetchErr)
	if out.UnavailableReason == "" && !out.VoiceReady {
		out.UnavailableReason = "SPEECH_NOT_QUALIFIED"
	}
	if out.UnavailableReason == "" && !entitled {
		out.UnavailableReason = "MODEL_PLAN_REQUIRED"
	}
	out.Available = out.UnavailableReason == ""
	return out
}

func (s *SpeechService) readiness(p SpeechProfile, catalog llm.SpeechCatalog, fetchErr error) string {
	c := s.source.SpeechConnection()
	if c.Disabled || c.ProviderID == "" {
		return "SPEECH_CONNECTION_UNAVAILABLE"
	}
	if !p.Enabled {
		return "SPEECH_PROFILE_UNAVAILABLE"
	}
	if c.ProviderID != p.Binding.Design.ProviderID || c.ProviderID != p.Binding.Synthesis.ProviderID {
		return "SPEECH_BINDING_INCOMPATIBLE"
	}
	if fetchErr != nil {
		return "SPEECH_CATALOG_UNAVAILABLE"
	}
	if len(catalog.ConnectionScope) != 64 || p.Binding.ConnectionScope != catalog.ConnectionScope {
		return "SPEECH_BINDING_INCOMPATIBLE"
	}
	d, dok := speechCandidate(catalog, p.Binding.Design, true)
	v, vok := speechCandidate(catalog, p.Binding.Synthesis, false)
	if !dok || !vok || !v.Korean || v.RequiresAlpha || !validSpeechCapabilities(p.Binding, d, v) {
		return "SPEECH_PATH_UNSUPPORTED"
	}
	if d != p.Binding.DesignModel || v != p.Binding.SpeechModel {
		return "SPEECH_PROFILE_CHANGED"
	}
	if !p.PricingReady(s.now()) {
		return "SPEECH_PRICE_UNAVAILABLE"
	}
	if p.Level == LevelFree && !p.ZeroPriced(s.now()) {
		return "SPEECH_PRICE_UNAVAILABLE"
	}
	return ""
}

func speechCandidate(c llm.SpeechCatalog, ref llm.ModelRef, design bool) (llm.SpeechModel, bool) {
	for _, m := range c.Models {
		if m.Ref == ref && ((design && m.Design) || (!design && m.Synthesis)) {
			return m, true
		}
	}
	return llm.SpeechModel{}, false
}

func validSpeechCapabilities(b SpeechBinding, d, v llm.SpeechModel) bool {
	return d.Design && v.Synthesis && v.Korean && !v.RequiresAlpha && v.MaxText > 0 &&
		b.OutputFormat == llm.SpeechOutputFormat && b.DescriptionMax >= llm.SpeechDescriptionMin && b.DescriptionMax <= llm.SpeechDescriptionMax &&
		b.PreviewMax >= llm.SpeechPreviewMin && b.PreviewMax <= llm.SpeechPreviewMax &&
		b.SpeechMax > 0 && b.SpeechMax <= llm.SpeechMaxText && b.SpeechMax <= v.MaxText &&
		(b.Settings.Style == 0 || v.Style) && (!b.Settings.SpeakerBoost || v.SpeakerBoost) && b.Settings.Validate() == nil
}

// Every curation change produces a revision. Historical snapshots remain intact;
// an old voice can read its binding but cannot bypass current live safety gates.
func (s *SpeechService) SaveSpeechProfile(ctx context.Context, p SpeechProfile, expected int64) (SpeechProfile, error) {
	if strings.TrimSpace(p.Label) == "" || !utf8.ValidString(p.Label) || utf8.RuneCountInString(p.Label) > SpeechProfileLabelMax {
		return SpeechProfile{}, ErrSpeechProfileInvalid
	}
	if _, ok := plan.ModelGradeRequired(string(p.Level)); !ok {
		return SpeechProfile{}, ErrInvalidLevel
	}
	if p.Binding.Design.ProviderID == "" || p.Binding.Design.ProviderID != p.Binding.Synthesis.ProviderID || p.Binding.Design.ModelID == "" || p.Binding.Synthesis.ModelID == "" {
		return SpeechProfile{}, ErrSpeechProfileInvalid
	}
	if p.Binding.Settings.Validate() != nil {
		return SpeechProfile{}, ErrSpeechProfileInvalid
	}
	if len(p.Prices) > len(SpeechOperations) {
		return SpeechProfile{}, ErrSpeechProfileInvalid
	}
	seen := map[SpeechOperation]bool{}
	for _, price := range p.Prices {
		if !slices.Contains(SpeechOperations, price.Operation) || seen[price.Operation] || !price.Valid(s.now()) {
			return SpeechProfile{}, ErrSpeechProfileInvalid
		}
		seen[price.Operation] = true
	}
	if p.Level == LevelFree && !p.ZeroPriced(s.now()) {
		return SpeechProfile{}, ErrFreeIneligible
	}
	var previous SpeechProfile
	if p.ID != "" {
		var err error
		previous, err = s.store.GetSpeechRevision(ctx, p.ID, expected)
		if err != nil {
			return SpeechProfile{}, err
		}
	} else {
		if expected != 0 {
			return SpeechProfile{}, ErrSpeechProfileConflict
		}
		p.ID = speechID()
	}
	// Reuse the frozen capability snapshot only for an unchanged binding edit.
	// This lets the operator amend prices/withdraw a profile during an outage.
	b := p.Binding
	b.DesignModel, b.SpeechModel = previous.Binding.DesignModel, previous.Binding.SpeechModel
	b.ConnectionScope = previous.Binding.ConnectionScope
	if previous.ID != "" && b == previous.Binding {
		p.Binding = b
		// A successful read can explicitly adopt changed capability/rate metadata
		// into this new revision. An outage leaves the historical snapshot intact.
		if catalog, err := s.source.ReadSpeechCatalog(ctx, false); err == nil {
			d, dok := speechCandidate(catalog, b.Design, true)
			v, vok := speechCandidate(catalog, b.Synthesis, false)
			if dok && vok && validSpeechCapabilities(b, d, v) {
				p.Binding.DesignModel, p.Binding.SpeechModel = d, v
				p.Binding.ConnectionScope = catalog.ConnectionScope
			}
		}
	} else {
		catalog, err := s.source.ReadSpeechCatalog(ctx, false)
		if err != nil {
			return SpeechProfile{}, ErrSpeechProfileUnavailable
		}
		d, dok := speechCandidate(catalog, p.Binding.Design, true)
		v, vok := speechCandidate(catalog, p.Binding.Synthesis, false)
		if !dok || !vok || !validSpeechCapabilities(p.Binding, d, v) {
			return SpeechProfile{}, ErrSpeechProfileInvalid
		}
		p.Binding.DesignModel, p.Binding.SpeechModel = d, v
		p.Binding.ConnectionScope = catalog.ConnectionScope
	}
	// Binding, tariff or grade edits never inherit live evidence automatically.
	p.Revision = expected + 1
	p.CreatedAt = s.now().UTC()
	p.VoiceEvidence, p.ExportEvidence = "", ""
	return s.store.SaveSpeechRevision(ctx, p, expected)
}

func (s *SpeechService) StartSpeechQualification(ctx context.Context, owner, id string, rev int64, maxUSD string) (SpeechQualificationSession, error) {
	cap, ok := SpeechDecimal(maxUSD)
	if !ok || owner == "" {
		return SpeechQualificationSession{}, ErrSpeechQualificationInvalid
	}
	p, err := s.store.GetSpeechRevision(ctx, id, rev)
	if err != nil {
		return SpeechQualificationSession{}, err
	}
	current, err := s.currentSpeechProfile(ctx, id)
	if err != nil || current.Revision != rev {
		return SpeechQualificationSession{}, ErrSpeechProfileConflict
	}
	catalog, fetchErr := s.source.ReadSpeechCatalog(ctx, false)
	if s.readiness(p, catalog, fetchErr) != "" {
		return SpeechQualificationSession{}, ErrSpeechProfileUnavailable
	}
	if cap.Sign() == 0 && !p.ZeroPriced(s.now()) {
		return SpeechQualificationSession{}, ErrSpeechQualificationInvalid
	}
	q := SpeechQualificationSession{ID: speechID(), OwnerID: owner, ProfileID: id, Revision: rev, MaximumUSD: maxUSD, ExpiresAt: s.now().Add(SpeechQualificationTTL).UTC()}
	return q, s.store.CreateSpeechQualification(ctx, q)
}

func (s *SpeechService) currentSpeechProfile(ctx context.Context, id string) (SpeechProfile, error) {
	profiles, err := s.store.ListSpeechProfiles(ctx)
	if err != nil {
		return SpeechProfile{}, err
	}
	for _, p := range profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return SpeechProfile{}, ErrNotFound
}

// Used by admission, including master qualification work. An opaque query flag
// alone provides no access; owner, expiry, current revision and live safety match.
func (s *SpeechService) ResolveSpeechProfile(ctx context.Context, owner string, tier plan.Plan, id string, rev int64, sessionID string, requireExport bool) (SpeechProfile, error) {
	p, err := s.store.GetSpeechRevision(ctx, id, rev)
	if err != nil {
		return SpeechProfile{}, err
	}
	current, err := s.currentSpeechProfile(ctx, id)
	if err != nil || current.Revision != rev {
		return SpeechProfile{}, ErrSpeechProfileConflict
	}
	_, entitled := plan.AllowsModelGrade(tier, string(p.Level))
	if !entitled {
		return SpeechProfile{}, ErrSpeechProfileUnavailable
	}
	catalog, fetchErr := s.source.ReadSpeechCatalog(ctx, false)
	if s.readiness(p, catalog, fetchErr) != "" {
		return SpeechProfile{}, ErrSpeechProfileUnavailable
	}
	if sessionID != "" {
		if tier != plan.Master {
			return SpeechProfile{}, ErrSpeechQualificationInvalid
		}
		q, err := s.SpeechQualification(ctx, owner, sessionID)
		if err != nil || q.ProfileID != id || q.Revision != rev {
			return SpeechProfile{}, ErrSpeechQualificationInvalid
		}
	} else if p.VoiceEvidence == "" || (requireExport && p.ExportEvidence == "") {
		return SpeechProfile{}, ErrSpeechProfileUnavailable
	}
	return p, nil
}

func (s *SpeechService) SpeechQualification(ctx context.Context, owner, id string) (SpeechQualificationSession, error) {
	q, err := s.store.GetSpeechQualification(ctx, owner, id)
	if err != nil || !q.ExpiresAt.After(s.now()) {
		return SpeechQualificationSession{}, ErrSpeechQualificationInvalid
	}
	return q, nil
}

func (s *SpeechService) RecordVoiceQualification(ctx context.Context, e SpeechQualificationEvidence) error {
	q, err := s.SpeechQualification(ctx, e.OwnerID, e.SessionID)
	if err != nil {
		return err
	}
	if e.ReportID == "" || e.DesignRequestID == "" || e.ConfirmRequestID == "" || e.ConfirmedVoiceID == "" || len(e.SpeechRequestIDs) < 3 || !e.AuditionAccepted || !e.KoreanAccepted || !e.ContinuityAccepted || !e.UsageVerified {
		return ErrSpeechQualificationInvalid
	}
	seen := map[string]bool{}
	for _, id := range append([]string{e.DesignRequestID, e.ConfirmRequestID}, e.SpeechRequestIDs...) {
		if id == "" || seen[id] {
			return ErrSpeechQualificationInvalid
		}
		seen[id] = true
	}
	if _, err := s.ResolveSpeechProfile(ctx, e.OwnerID, plan.Master, q.ProfileID, q.Revision, q.ID, false); err != nil {
		return err
	}
	return s.store.RecordSpeechReadiness(ctx, q.ProfileID, q.Revision, e.ReportID, false)
}

func speechID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("secure speech identity unavailable")
	}
	return hex.EncodeToString(b[:])
}
