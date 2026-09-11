// Package rpc is the authenticated clip transport boundary.
package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobrpc "github.com/postpilot/backend/internal/job/rpc"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"log/slog"
	"strings"
	"time"
)

type Handler struct {
	service    *clip.Service
	sources    *clip.SourceService
	generation *clip.GenerationService
	jobs       *job.Queue
}

func (h *Handler) WithGeneration(g *clip.GenerationService, j *job.Queue) *Handler {
	h.generation = g
	h.jobs = j
	return h
}

func (h *Handler) StartClipGeneration(ctx context.Context, req *connect.Request[v1.StartClipGenerationRequest]) (*connect.Response[v1.StartClipGenerationResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(errors.New("clip generation unavailable"))
	}
	observe := llm.ModelRef{ProviderID: req.Msg.GetObserveModel().GetProviderId(), ModelID: req.Msg.GetObserveModel().GetModelId()}
	write := llm.ModelRef{ProviderID: req.Msg.GetWriteModel().GetProviderId(), ModelID: req.Msg.GetWriteModel().GetModelId()}
	var maxCredits *int
	if req.Msg.ApprovedMaxCredits != nil {
		value := int(*req.Msg.ApprovedMaxCredits)
		maxCredits = &value
	}
	id, err := h.generation.Start(ctx, user, req.Msg.ProjectId, req.Msg.BatchId, observe.String(), write.String(), clip.QuoteApproval{QuoteID: req.Msg.QuoteId, MaxCredits: maxCredits})
	if err != nil {
		var admission *clip.ModelAdmissionError
		if errors.Is(err, llm.ErrUnsupported) && !errors.As(err, &admission) {
			return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition, "video input is required", "MODEL_VIDEO_UNSUPPORTED", map[string]string{"model": observe.String()})
		}
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.StartClipGenerationResponse{JobId: id}), nil
}

// ListClipAnalysisEligibility answers CLIP-44 for every registered observe model:
// refs and statuses only, no model call, no write, nothing from the provider.
func (h *Handler) ListClipAnalysisEligibility(ctx context.Context, _ *connect.Request[v1.ListClipAnalysisEligibilityRequest]) (*connect.Response[v1.ListClipAnalysisEligibilityResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrPricingUnavailable)
	}
	items, err := h.generation.ListAnalysisEligibility(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := &v1.ListClipAnalysisEligibilityResponse{Models: make([]*v1.ClipAnalysisModelEligibility, 0, len(items))}
	for _, item := range items {
		out.Models = append(out.Models, &v1.ClipAnalysisModelEligibility{Model: &v1.ModelRef{ProviderId: item.Model.ProviderID, ModelId: item.Model.ModelID}, Status: eligibilityProto(item.Status)})
	}
	return connect.NewResponse(out), nil
}

// eligibilityProto maps the five domain statuses; anything else is the
// unspecified wire value, which no reader may take as eligible.
func eligibilityProto(s clip.EligibilityStatus) v1.ClipAnalysisEligibility {
	switch s {
	case clip.EligibilityEligible:
		return v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_ELIGIBLE
	case clip.EligibilityVideoInputAbsent:
		return v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_VIDEO_INPUT_ABSENT
	case clip.EligibilityInlineEndpointUnavailable:
		return v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_INLINE_ENDPOINT_UNAVAILABLE
	case clip.EligibilityRequiredParametersUnsupported:
		return v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_REQUIRED_PARAMETERS_UNSUPPORTED
	case clip.EligibilityPriceCeilingUnavailable:
		return v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_PRICE_CEILING_UNAVAILABLE
	}
	return v1.ClipAnalysisEligibility_CLIP_ANALYSIS_ELIGIBILITY_UNSPECIFIED
}

func NewHandler(service *clip.Service) *Handler                     { return &Handler{service: service} }
func (h *Handler) WithSources(sources *clip.SourceService) *Handler { h.sources = sources; return h }
func actingUser(ctx context.Context) (string, error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return user, nil
}
func toConnectError(err error) error {
	var facts *clip.MissingFactsError
	var admission *clip.ModelAdmissionError
	switch {
	// The model's admission answer first: it unwraps to the generic unsupported
	// error and must keep its own reason and the model it names (CLIP-44, LANG-21).
	case errors.As(err, &admission):
		f := admission.Failure()
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip analysis model not eligible", f.Reason, f.Params)
	case errors.Is(err, clip.ErrQuoteRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip credit approval required", "CLIP_QUOTE_REQUIRED", nil)
	case errors.Is(err, clip.ErrQuoteExpired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip credit quote expired", "CLIP_QUOTE_EXPIRED", nil)
	case errors.Is(err, clip.ErrQuoteChanged):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip credit quote changed", "CLIP_QUOTE_CHANGED", nil)
	case errors.Is(err, clip.ErrPricingUnavailable):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip model pricing unavailable", "CLIP_MODEL_PRICING_UNAVAILABLE", nil)
	case errors.Is(err, clip.ErrModelInputUnsupported):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip model input unsupported", "CLIP_MODEL_INPUT_UNSUPPORTED", nil)
	case errors.Is(err, clip.ErrWorkspaceLimit):
		return rpcserver.NewAppError(connect.CodeResourceExhausted, "clip workspace limit", "CLIP_WORKSPACE_LIMIT", nil)
	case errors.Is(err, clip.ErrAnalysisTooLarge):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "clip analysis copy limit", "CLIP_ANALYSIS_TOO_LARGE", nil)
	case errors.Is(err, clip.ErrPlanConflict):
		return rpcserver.NewAppError(connect.CodeAborted, "clip edit plan changed", "CLIP_PLAN_CONFLICT", nil)
	case errors.Is(err, clip.ErrDisclosureRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip disclosure is required", "CLIP_DISCLOSURE_REQUIRED", nil)
	case errors.As(err, &facts):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip needs more on-screen facts", "CLIP_FACTS_REQUIRED", map[string]string{"labels": strings.Join(facts.Labels, ", ")})
	case errors.Is(err, clip.ErrBusy):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip is busy", "CLIP_BUSY", nil)
	case errors.Is(err, llm.ErrModelUnavailable):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "model is unavailable", "MODEL_UNAVAILABLE", nil)
	case errors.Is(err, llm.ErrProviderDisabled):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "model provider is disabled", "PROVIDER_DISABLED", nil)
	case errors.Is(err, clip.ErrInvalidMedia):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "clip media is invalid", "CLIP_INVALID_MEDIA", nil)
	case errors.Is(err, clip.ErrCopyTooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "clip copy does not fit", "CLIP_COPY_TOO_LONG", nil)
	case errors.Is(err, clip.ErrSourceState):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip source batch is not available", "CLIP_SOURCE_UNAVAILABLE", nil)
	case errors.Is(err, clip.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "clip or video template not found", "CLIP_NOT_FOUND", nil)
	case errors.Is(err, clip.ErrDuplicateName):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "video template name already exists", "CLIP_TEMPLATE_NAME_TAKEN", nil)
	case errors.Is(err, clip.ErrInvalid):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid clip input", "CLIP_INVALID_INPUT", nil)
	default:
		slog.Error("clip operation failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, "clip operation failed", "UNKNOWN_FAILURE", nil)
	}
}
func fields(values []*v1.ClipInformationField) []clip.InformationField {
	out := make([]clip.InformationField, 0, len(values))
	for _, v := range values {
		out = append(out, clip.InformationField{Label: v.GetLabel(), Prompt: v.GetPrompt()})
	}
	return out
}
func answers(values []*v1.ClipAnswer) []clip.Answer {
	out := make([]clip.Answer, 0, len(values))
	for _, v := range values {
		out = append(out, clip.Answer{Label: v.GetLabel(), Text: v.GetText()})
	}
	return out
}
func templateProto(t clip.VideoTemplate) *v1.VideoTemplate {
	out := &v1.VideoTemplate{Id: t.ID, Name: t.Name, CutGuidance: t.CutGuidance, CopyStyles: t.CopyStyles, Accent: t.Accent, Preset: t.Preset, ProjectCount: int32(t.ProjectCount), CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: t.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	for _, f := range t.InformationFields {
		out.InformationFields = append(out.InformationFields, &v1.ClipInformationField{Label: f.Label, Prompt: f.Prompt})
	}
	return out
}
func projectProto(p clip.Project) *v1.ClipProject {
	out := &v1.ClipProject{Id: p.ID, Title: p.Title, VideoTemplateId: p.VideoTemplateID, Ratio: p.Ratio, Disclosure: p.Disclosure, Cta: p.CTA, TargetDurationMs: int32(p.TargetDurationMS), EditPlanRevision: int32(p.EditPlanRevision), RenderedPlanRevision: int32(p.RenderedPlanRevision), CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	for _, a := range p.Answers {
		out.Answers = append(out.Answers, &v1.ClipAnswer{Label: a.Label, Text: a.Text})
	}
	if r := p.Result; r != nil {
		out.Result = &v1.ClipResult{ContentType: r.ContentType, Bytes: r.Bytes, DurationMs: int32(r.DurationMS), CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano), ViewUrl: r.ViewURL, DownloadUrl: r.DownloadURL}
	}
	return out
}
func (h *Handler) ListVideoTemplates(ctx context.Context, req *connect.Request[v1.ListVideoTemplatesRequest]) (*connect.Response[v1.ListVideoTemplatesResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	values, err := h.service.ListTemplates(ctx, user)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*v1.VideoTemplate, 0, len(values))
	for _, v := range values {
		out = append(out, templateProto(v))
	}
	return connect.NewResponse(&v1.ListVideoTemplatesResponse{Templates: out}), nil
}
func (h *Handler) CreateVideoTemplate(ctx context.Context, req *connect.Request[v1.CreateVideoTemplateRequest]) (*connect.Response[v1.CreateVideoTemplateResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	value, err := h.service.CreateTemplate(ctx, user, clip.Recipe{Name: m.Name, InformationFields: fields(m.InformationFields), CutGuidance: m.CutGuidance, CopyStyles: m.CopyStyles, Accent: m.Accent, Preset: m.Preset})
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.CreateVideoTemplateResponse{Template: templateProto(value)}), nil
}
func (h *Handler) UpdateVideoTemplate(ctx context.Context, req *connect.Request[v1.UpdateVideoTemplateRequest]) (*connect.Response[v1.UpdateVideoTemplateResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	p := clip.TemplatePatch{Name: m.Name, CutGuidance: m.CutGuidance, Accent: m.Accent, Preset: m.Preset}
	if m.InformationFields != nil {
		v := fields(m.InformationFields.Values)
		p.InformationFields = &v
	}
	if m.CopyStyles != nil {
		p.CopyStyles = &m.CopyStyles.Values
	}
	value, err := h.service.UpdateTemplate(ctx, user, m.Id, p)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.UpdateVideoTemplateResponse{Template: templateProto(value)}), nil
}

// SeedPresetFields answers with the reserved information fields a preset needs,
// so the editor can seed them without the owner typing a Korean label exactly.
// Pure and owner-independent, but authenticated like every other procedure.
func (h *Handler) SeedPresetFields(ctx context.Context, req *connect.Request[v1.SeedPresetFieldsRequest]) (*connect.Response[v1.SeedPresetFieldsResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	if !clip.ValidPreset(req.Msg.Preset) {
		return nil, toConnectError(clip.ErrInvalid)
	}
	out := &v1.SeedPresetFieldsResponse{}
	for _, f := range design.PresetFields(req.Msg.Preset) {
		out.Fields = append(out.Fields, &v1.ClipInformationField{Label: f.Label, Prompt: f.Prompt})
	}
	return connect.NewResponse(out), nil
}
func (h *Handler) DeleteVideoTemplate(ctx context.Context, req *connect.Request[v1.DeleteVideoTemplateRequest]) (*connect.Response[v1.DeleteVideoTemplateResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	n, err := h.service.DeleteTemplate(ctx, user, req.Msg.Id)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.DeleteVideoTemplateResponse{DetachedProjects: int32(n)}), nil
}
func (h *Handler) ListClipProjects(ctx context.Context, req *connect.Request[v1.ListClipProjectsRequest]) (*connect.Response[v1.ListClipProjectsResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	values, err := h.service.ListProjects(ctx, user)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*v1.ClipProject, 0, len(values))
	for _, v := range values {
		p := projectProto(v)
		// The directory badges a running generation and a failed attempt (CLIP-41), so each row
		// carries its latest job the way the detail does. One indexed read per project, as the
		// post list does for its own active job; `editing`, `accounting` and `latest_attempt`
		// stay detail-only.
		if h.jobs != nil {
			j, err := h.jobs.LatestForClip(ctx, user, v.ID)
			if err != nil {
				return nil, toConnectError(err)
			}
			p.LatestJob = jobrpc.ToProto(j)
		}
		out = append(out, p)
	}
	return connect.NewResponse(&v1.ListClipProjectsResponse{Projects: out}), nil
}
func (h *Handler) CreateClipProject(ctx context.Context, req *connect.Request[v1.CreateClipProjectRequest]) (*connect.Response[v1.CreateClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	value, err := h.service.CreateProject(ctx, user, clip.ProjectInput{Title: m.Title, VideoTemplateID: m.VideoTemplateId, Ratio: m.Ratio, TargetDurationMS: int(m.TargetDurationMs), Disclosure: m.Disclosure, CTA: m.Cta, Answers: answers(m.Answers)})
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.CreateClipProjectResponse{Project: projectProto(value)}), nil
}
func (h *Handler) GetClipProject(ctx context.Context, req *connect.Request[v1.GetClipProjectRequest]) (*connect.Response[v1.GetClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	value, err := h.service.GetProject(ctx, user, req.Msg.Id)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := projectProto(value)
	if h.generation != nil {
		state, err := h.generation.EditingState(value)
		if err != nil {
			return nil, toConnectError(err)
		}
		out.Editing = editingProto(state)
		accounting, err := h.generation.Accounting(ctx, user, value.ID)
		if err != nil {
			return nil, toConnectError(err)
		}
		out.Accounting = accountingProto(accounting)
	}
	if h.jobs != nil {
		j, err := h.jobs.LatestForClip(ctx, user, value.ID)
		if err != nil {
			return nil, toConnectError(err)
		}
		out.LatestJob = jobrpc.ToProto(j)
		if j != nil {
			snapshot, err := h.jobs.LatestClipSnapshot(ctx, user, value.ID)
			if err != nil {
				return nil, toConnectError(err)
			}
			if snapshot != nil && snapshot.ID == j.ID && (snapshot.DispatchReady || snapshot.Status == job.StatusDone || snapshot.Status == job.StatusFailed) {
				if a := clip.IdentifyAttempt(user, value.ID, snapshot.ID, snapshot.Kind, snapshot.Payload); a != nil {
					out.LatestAttempt = &v1.ClipAttempt{JobId: a.JobID, BatchId: a.BatchID, QuoteId: a.QuoteID}
				}
			}
		}
	}
	return connect.NewResponse(&v1.GetClipProjectResponse{Project: out}), nil
}
func (h *Handler) UpdateClipProject(ctx context.Context, req *connect.Request[v1.UpdateClipProjectRequest]) (*connect.Response[v1.UpdateClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	p := clip.ProjectPatch{Title: m.Title, VideoTemplateID: m.VideoTemplateId, Disclosure: m.Disclosure, CTA: m.Cta, Answers: answers(m.Answers)}
	if m.TargetDurationMs != nil {
		v := int(*m.TargetDurationMs)
		p.TargetDurationMS = &v
	}
	value, err := h.service.UpdateProject(ctx, user, m.Id, p)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.UpdateClipProjectResponse{Project: projectProto(value)}), nil
}
func (h *Handler) DeleteClipProject(ctx context.Context, req *connect.Request[v1.DeleteClipProjectRequest]) (*connect.Response[v1.DeleteClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.DeleteProject(ctx, user, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.DeleteClipProjectResponse{}), nil
}
