package rpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/modelcatalog"
	"github.com/postpilot/backend/internal/plan"
)

type speechProfileService interface {
	SpeechChoices(context.Context, plan.Plan) ([]modelcatalog.SpeechChoice, error)
	QualificationSpeechChoices(context.Context, string, plan.Plan, string) ([]modelcatalog.SpeechChoice, error)
	BrowseSpeech(context.Context, bool) (modelcatalog.SpeechAdminBrowse, error)
	SaveSpeechProfile(context.Context, modelcatalog.SpeechProfile, int64) (modelcatalog.SpeechProfile, error)
	StartSpeechQualification(context.Context, string, string, int64, string) (modelcatalog.SpeechQualificationSession, error)
}
type SpeechHandler struct{ svc speechProfileService }

func NewSpeechHandler(svc speechProfileService) *SpeechHandler {
	return &SpeechHandler{svc: svc}
}

func (h *SpeechHandler) ListSpeechProfiles(ctx context.Context, req *connect.Request[v1.ListSpeechProfilesRequest]) (*connect.Response[v1.ListSpeechProfilesResponse], error) {
	tier, ok := auth.PlanFromContext(ctx)
	owner, identified := auth.UserFromContext(ctx)
	if !ok || !identified {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	var choices []modelcatalog.SpeechChoice
	var err error
	if session := req.Msg.GetQualificationSessionId(); session != "" {
		if tier != plan.Master {
			return nil, speechError(modelcatalog.ErrSpeechQualificationInvalid)
		}
		choices, err = h.svc.QualificationSpeechChoices(ctx, owner, tier, session)
	} else {
		choices, err = h.svc.SpeechChoices(ctx, tier)
	}
	if err != nil {
		return nil, speechError(err)
	}
	response := &v1.ListSpeechProfilesResponse{}
	for _, c := range choices {
		response.Profiles = append(response.Profiles, speechChoice(c))
	}
	return connect.NewResponse(response), nil
}

func requireSpeechMaster(ctx context.Context) error {
	if !auth.ActsAsMaster(ctx) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("master required"))
	}
	return nil
}

func (h *SpeechHandler) AdminListSpeechProfiles(ctx context.Context, req *connect.Request[v1.AdminListSpeechProfilesRequest]) (*connect.Response[v1.AdminListSpeechProfilesResponse], error) {
	if err := requireSpeechMaster(ctx); err != nil {
		return nil, err
	}
	browse, err := h.svc.BrowseSpeech(ctx, req.Msg.GetRefresh())
	if err != nil {
		return nil, speechError(err)
	}
	response := &v1.AdminListSpeechProfilesResponse{FetchError: browse.FetchError}
	for _, p := range browse.Profiles {
		response.Profiles = append(response.Profiles, speechAdmin(p))
	}
	for _, c := range browse.Choices {
		response.Choices = append(response.Choices, speechChoice(c))
	}
	for _, m := range browse.Candidates {
		response.Candidates = append(response.Candidates, &v1.SpeechModelCandidate{Ref: speechRef(m.Ref), Label: m.Label, Design: m.Design, Synthesis: m.Synthesis, Korean: m.Korean, Style: m.Style, SpeakerBoost: m.SpeakerBoost, RequiresAlpha: m.RequiresAlpha, MaxText: int32(m.MaxText), TokenCostFactor: m.TokenCostFactor, CharacterCostMultiplier: m.CharacterCostMultiplier, CostDiscountMultiplier: m.CostDiscountMultiplier})
	}
	return connect.NewResponse(response), nil
}

func (h *SpeechHandler) SaveSpeechProfile(ctx context.Context, req *connect.Request[v1.SaveSpeechProfileRequest]) (*connect.Response[v1.SaveSpeechProfileResponse], error) {
	if err := requireSpeechMaster(ctx); err != nil {
		return nil, err
	}
	p, err := speechDraft(req.Msg.GetProfile())
	if err != nil {
		return nil, speechError(err)
	}
	saved, err := h.svc.SaveSpeechProfile(ctx, p, req.Msg.GetExpectedRevision())
	if err != nil {
		return nil, speechError(err)
	}
	return connect.NewResponse(&v1.SaveSpeechProfileResponse{Profile: speechAdmin(saved)}), nil
}

func (h *SpeechHandler) StartSpeechQualification(ctx context.Context, req *connect.Request[v1.StartSpeechQualificationRequest]) (*connect.Response[v1.StartSpeechQualificationResponse], error) {
	if err := requireSpeechMaster(ctx); err != nil {
		return nil, err
	}
	owner, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	q, err := h.svc.StartSpeechQualification(ctx, owner, req.Msg.GetProfileId(), req.Msg.GetRevision(), req.Msg.GetMaximumUsd())
	if err != nil {
		return nil, speechError(err)
	}
	return connect.NewResponse(&v1.StartSpeechQualificationResponse{SessionId: q.ID, ProfileId: q.ProfileID, Revision: q.Revision, ExpiresAt: q.ExpiresAt.Format(time.RFC3339Nano)}), nil
}

func speechRef(r llm.ModelRef) *v1.ModelRef {
	return &v1.ModelRef{ProviderId: r.ProviderID, ModelId: r.ModelID}
}
func speechDomainRef(r *v1.ModelRef) llm.ModelRef {
	return llm.ModelRef{ProviderID: r.GetProviderId(), ModelID: r.GetModelId()}
}

func speechChoice(c modelcatalog.SpeechChoice) *v1.SpeechProfileChoice {
	return &v1.SpeechProfileChoice{Id: c.ID, Revision: c.Revision, Label: c.Label, DesignModel: speechRef(c.Design), DesignLabel: c.DesignLabel, SpeechModel: speechRef(c.Synthesis), SpeechLabel: c.SynthesisLabel,
		Grade: string(c.Level), RequiredPlan: string(c.RequiredPlan), Entitled: c.Entitled, Available: c.Available, UnavailableReason: c.UnavailableReason,
		DescriptionMax: int32(c.DescriptionMax), PreviewMin: int32(c.PreviewMin), PreviewMax: int32(c.PreviewMax), SpeechMax: int32(c.SpeechMax), VoiceReady: c.VoiceReady, ExportReady: c.ExportReady}
}

func speechAdmin(p modelcatalog.SpeechProfile) *v1.AdminSpeechProfile {
	b := p.Binding
	s := b.Settings
	out := &v1.AdminSpeechProfile{Id: p.ID, Revision: p.Revision, Label: p.Label, Grade: string(p.Level), Enabled: p.Enabled, VoiceReady: p.VoiceEvidence != "", ExportReady: p.ExportEvidence != "",
		Binding: &v1.SpeechProfileBinding{DesignModel: speechRef(b.Design), SpeechModel: speechRef(b.Synthesis), Settings: &v1.SpeechProfileSettings{Stability: s.Stability, SimilarityBoost: s.SimilarityBoost, Style: s.Style, SpeakerBoost: s.SpeakerBoost, Speed: s.Speed}, OutputFormat: b.OutputFormat, DescriptionMax: int32(b.DescriptionMax), PreviewMax: int32(b.PreviewMax), SpeechMax: int32(b.SpeechMax)}}
	for _, price := range p.Prices {
		v := &v1.SpeechOperationPrice{Operation: string(price.Operation), Source: price.Source, BoundsSource: price.BoundsSource, CheckedAt: price.CheckedAt.Format(time.RFC3339Nano), Complete: price.Complete}
		for _, c := range price.Charges {
			v.Charges = append(v.Charges, &v1.SpeechPriceComponent{Unit: string(c.Unit), UsdPerUnit: c.USDPerUnit, Multiplier: c.Multiplier, MaximumUnits: c.MaximumUnits, UnitsPerInputCharacter: c.UnitsPerInputCharacter})
		}
		out.Prices = append(out.Prices, v)
	}
	return out
}

func speechDraft(p *v1.AdminSpeechProfile) (modelcatalog.SpeechProfile, error) {
	if p == nil || p.GetBinding() == nil || p.GetBinding().GetSettings() == nil {
		return modelcatalog.SpeechProfile{}, modelcatalog.ErrSpeechProfileInvalid
	}
	b := p.GetBinding()
	s := b.GetSettings()
	out := modelcatalog.SpeechProfile{ID: p.GetId(), Label: p.GetLabel(), Level: modelcatalog.Level(p.GetGrade()), Enabled: p.GetEnabled(),
		Binding: modelcatalog.SpeechBinding{Design: speechDomainRef(b.GetDesignModel()), Synthesis: speechDomainRef(b.GetSpeechModel()), OutputFormat: b.GetOutputFormat(), DescriptionMax: int(b.GetDescriptionMax()), PreviewMax: int(b.GetPreviewMax()), SpeechMax: int(b.GetSpeechMax()), Settings: llm.SpeechSettings{Stability: s.GetStability(), SimilarityBoost: s.GetSimilarityBoost(), Style: s.GetStyle(), SpeakerBoost: s.GetSpeakerBoost(), Speed: s.GetSpeed()}}}
	for _, price := range p.GetPrices() {
		at, err := time.Parse(time.RFC3339Nano, price.GetCheckedAt())
		if err != nil {
			return modelcatalog.SpeechProfile{}, modelcatalog.ErrSpeechProfileInvalid
		}
		v := modelcatalog.SpeechPrice{Operation: modelcatalog.SpeechOperation(price.GetOperation()), Source: price.GetSource(), BoundsSource: price.GetBoundsSource(), CheckedAt: at, Complete: price.GetComplete()}
		for _, c := range price.GetCharges() {
			v.Charges = append(v.Charges, modelcatalog.SpeechCharge{Unit: llm.SpeechUnit(c.GetUnit()), USDPerUnit: c.GetUsdPerUnit(), Multiplier: c.GetMultiplier(), MaximumUnits: c.GetMaximumUnits(), UnitsPerInputCharacter: c.GetUnitsPerInputCharacter()})
		}
		out.Prices = append(out.Prices, v)
	}
	return out, nil
}

// A customer error cannot reflect SQL rows, keys, supplier prose or prices.
func speechError(err error) error {
	code := connect.CodeInternal
	message := "speech profile request failed"
	switch {
	case errors.Is(err, modelcatalog.ErrNotFound):
		code, message = connect.CodeNotFound, "speech profile not found"
	case errors.Is(err, modelcatalog.ErrSpeechProfileInvalid), errors.Is(err, modelcatalog.ErrInvalidLevel), errors.Is(err, modelcatalog.ErrFreeIneligible):
		code, message = connect.CodeInvalidArgument, "speech profile fields are invalid"
	case errors.Is(err, modelcatalog.ErrSpeechProfileConflict):
		code, message = connect.CodeAborted, "speech profile changed; refresh before continuing"
	case errors.Is(err, modelcatalog.ErrSpeechQualificationInvalid):
		code, message = connect.CodePermissionDenied, "speech qualification session is unavailable"
	case errors.Is(err, modelcatalog.ErrSpeechProfileUnavailable):
		code, message = connect.CodeFailedPrecondition, "speech profile is unavailable"
	}
	return connect.NewError(code, errors.New(message))
}

var _ postpilotv1connect.SpeechProfileServiceHandler = (*SpeechHandler)(nil)
