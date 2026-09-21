// Package rpc is the authenticated clip transport boundary.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobrpc "github.com/postpilot/backend/internal/job/rpc"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

type Handler struct {
	service    *clipapp.Service
	sources    *clipapp.SourceService
	generation *clipapp.GenerationService
	jobs       *job.Queue
}

func (h *Handler) WithGeneration(g *clipapp.GenerationService, j *job.Queue) *Handler {
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
	id, err := h.generation.Start(ctx, user, req.Msg.ProjectId, req.Msg.BatchId, observe.String(), write.String(), clip.QuoteApproval{CancellationPolicyVersion: int(req.Msg.CancellationPolicyVersion), QuoteID: req.Msg.QuoteId, MaxCredits: maxCredits})
	if err != nil {
		var admission *clip.ModelAdmissionError
		if errors.Is(err, llm.ErrUnsupported) && !errors.As(err, &admission) {
			return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition, "video input is required", postpilotv1.FailureReason_MODEL_VIDEO_UNSUPPORTED, map[string]string{"model": observe.String()})
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

func NewHandler(service *clipapp.Service) *Handler                     { return &Handler{service: service} }
func (h *Handler) WithSources(sources *clipapp.SourceService) *Handler { h.sources = sources; return h }
func actingUser(ctx context.Context) (string, error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return user, nil
}
func toConnectError(err error) error {
	var facts *clip.MissingFactsError
	var problem *composition.Problem
	var admission *clip.ModelAdmissionError
	var cut *clip.CutError
	switch {
	case errors.As(err, &cut):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid clip cut", postpilotv1.FailureReason_CLIP_INVALID_INPUT, map[string]string{"cut_id": cut.CutID, "check": cut.OutputValidationCode()})
	case errors.Is(err, clip.ErrFinalized):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip is finalized", postpilotv1.FailureReason_CLIP_FINALIZED, nil)
	case errors.Is(err, clip.ErrFinalizationConflict):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip result changed", postpilotv1.FailureReason_CLIP_FINALIZATION_CONFLICT, nil)
	case errors.Is(err, clip.ErrFinalizationInvalid):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip cannot be finalized", postpilotv1.FailureReason_CLIP_FINALIZATION_INVALID, nil)
	case errors.Is(err, clip.ErrPreviewBusy):
		return rpcserver.NewAppError(connect.CodeResourceExhausted, "clip preview is busy", postpilotv1.FailureReason_CLIP_PREVIEW_BUSY, nil)
	case errors.Is(err, clip.ErrPreviewTooLarge):
		return rpcserver.NewAppError(connect.CodeResourceExhausted, "clip preview is too large", postpilotv1.FailureReason_CLIP_PREVIEW_TOO_LARGE, nil)
	case errors.Is(err, clip.ErrPreviewUnavailable):
		return rpcserver.NewAppError(connect.CodeUnavailable, "clip preview unavailable", postpilotv1.FailureReason_CLIP_PREVIEW_UNAVAILABLE, nil)
	case errors.As(err, &problem):
		// A bounded answer names what the owner typed and by how much it is over,
		// because a line number in a template body means nothing in the answer
		// form (CLIP-102).
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid clip composition", postpilotv1.FailureReason_CLIP_COMPOSITION_INVALID, problem.FailureParams())
	case errors.Is(err, clip.ErrCompositionUnavailable):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip composition execution unavailable", postpilotv1.FailureReason_CLIP_COMPOSITION_UNAVAILABLE, nil)
	// The model's admission answer first: it unwraps to the generic unsupported
	// error and must keep its own reason and the model it names (CLIP-44, LANG-21).
	case errors.As(err, &admission):
		f := admission.Failure()
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip analysis model not eligible", rpcserver.ReasonOf(f.Reason), f.Params)
	case errors.Is(err, clip.ErrQuoteRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip credit approval required", postpilotv1.FailureReason_CLIP_QUOTE_REQUIRED, nil)
	case errors.Is(err, clip.ErrCancellationPolicy):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip cancellation approval required", postpilotv1.FailureReason_CLIP_CANCELLATION_POLICY_REQUIRED, nil)
	case errors.Is(err, clip.ErrQuoteExpired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip credit quote expired", postpilotv1.FailureReason_CLIP_QUOTE_EXPIRED, nil)
	case errors.Is(err, clip.ErrQuoteChanged):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip credit quote changed", postpilotv1.FailureReason_CLIP_QUOTE_CHANGED, nil)
	case errors.Is(err, clip.ErrPricingUnavailable):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip model pricing unavailable", postpilotv1.FailureReason_CLIP_MODEL_PRICING_UNAVAILABLE, nil)
	case errors.Is(err, clip.ErrModelInputUnsupported):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip model input unsupported", postpilotv1.FailureReason_CLIP_MODEL_INPUT_UNSUPPORTED, nil)
	case errors.Is(err, clip.ErrWorkspaceLimit):
		return rpcserver.NewAppError(connect.CodeResourceExhausted, "clip workspace limit", postpilotv1.FailureReason_CLIP_WORKSPACE_LIMIT, nil)
	case errors.Is(err, clip.ErrInputTooLarge):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "clip input limit", postpilotv1.FailureReason_CLIP_INPUT_TOO_LARGE, nil)
	case errors.Is(err, clip.ErrAnalysisTooLarge):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "clip analysis copy limit", postpilotv1.FailureReason_CLIP_ANALYSIS_TOO_LARGE, nil)
	case errors.Is(err, clip.ErrPlanConflict):
		return rpcserver.NewAppError(connect.CodeAborted, "clip edit plan changed", postpilotv1.FailureReason_CLIP_PLAN_CONFLICT, nil)
	case errors.Is(err, clip.ErrDisclosureRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip disclosure is required", postpilotv1.FailureReason_CLIP_DISCLOSURE_REQUIRED, nil)
	case errors.Is(err, clip.ErrTargetDurationRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip target duration is required", postpilotv1.FailureReason_CLIP_TARGET_DURATION_REQUIRED, nil)
	case errors.As(err, &facts):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip needs more on-screen facts", postpilotv1.FailureReason_CLIP_FACTS_REQUIRED, map[string]string{"labels": strings.Join(facts.Labels, ", ")})
	case errors.Is(err, clip.ErrBusy):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip is busy", postpilotv1.FailureReason_CLIP_BUSY, nil)
	case errors.Is(err, llm.ErrModelUnavailable):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "model is unavailable", postpilotv1.FailureReason_MODEL_UNAVAILABLE, nil)
	case errors.Is(err, llm.ErrProviderDisabled):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "model provider is disabled", postpilotv1.FailureReason_PROVIDER_DISABLED, nil)
	case errors.Is(err, clip.ErrInvalidMedia):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "clip media is invalid", postpilotv1.FailureReason_CLIP_INVALID_MEDIA, nil)
	case errors.Is(err, clip.ErrCopyTooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "clip copy does not fit", postpilotv1.FailureReason_CLIP_COPY_TOO_LONG, nil)
	case errors.Is(err, clip.ErrSourceExpired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip originals expired", postpilotv1.FailureReason_CLIP_SOURCE_EXPIRED, nil)
	case errors.Is(err, clip.ErrSourceMissing):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip originals missing", postpilotv1.FailureReason_CLIP_SOURCE_MISSING, nil)
	case errors.Is(err, clip.ErrSourceState):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "clip source batch is not available", postpilotv1.FailureReason_CLIP_SOURCE_UNAVAILABLE, nil)
	case errors.Is(err, clip.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "clip or video template not found", postpilotv1.FailureReason_CLIP_NOT_FOUND, nil)
	case errors.Is(err, clip.ErrDuplicateName):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "video template name already exists", postpilotv1.FailureReason_CLIP_TEMPLATE_NAME_TAKEN, nil)
	case errors.Is(err, clip.ErrRenderUnavailable):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.Is(err, clip.ErrInvalid):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid clip input", postpilotv1.FailureReason_CLIP_INVALID_INPUT, nil)
	default:
		slog.Error("clip operation failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, "clip operation failed", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
}
func fields(values []*v1.ClipInformationField) []clip.InformationField {
	out := make([]clip.InformationField, 0, len(values))
	for _, v := range values {
		out = append(out, clip.InformationField{Label: v.GetLabel(), Prompt: v.GetPrompt()})
	}
	return out
}

// captionStyles carries the wrapper's presence through: absent leaves the
// selection as it is, and present-and-empty is a selection of none, which
// resolves to the default style alone (CLIP-142).
func captionStyles(m *v1.ClipCaptionStyles) *[]string {
	if m == nil {
		return nil
	}
	values := m.GetValues()
	if values == nil {
		values = []string{}
	}
	return &values
}
func answers(values []*v1.ClipAnswer) []clip.Answer {
	out := make([]clip.Answer, 0, len(values))
	for _, v := range values {
		out = append(out, clip.Answer{Label: v.GetLabel(), Text: v.GetText()})
	}
	return out
}
func templateProto(t clip.VideoTemplate) *v1.VideoTemplate {
	out := &v1.VideoTemplate{CompositionBody: t.CompositionBody, CompositionLegacy: t.CompositionLegacy, CompositionConverted: t.CompositionConverted, Id: t.ID, Name: t.Name, CutGuidance: t.CutGuidance, CaptionPace: t.CaptionPace, Accent: t.Accent, Preset: t.Preset, ProjectCount: int32(t.ProjectCount), CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: t.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	for _, f := range t.InformationFields {
		out.InformationFields = append(out.InformationFields, &v1.ClipInformationField{Label: f.Label, Prompt: f.Prompt})
	}
	return out
}
func projectProto(p clip.Project) *v1.ClipProject {
	canEdit, canFinalize := p.Finalized == nil, false
	out := &v1.ClipProject{CanEdit: &canEdit, CanFinalize: &canFinalize, Composition: compositionProto(p.Composition), Id: p.ID, Title: p.Title, VideoTemplateId: p.VideoTemplateID, Ratio: p.Ratio, Language: languageToProto(p.Language), Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, Cta: p.CTA, Instruction: p.Instruction, CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, AllowedCaptionStyles: p.CaptionStyles, TargetDurationMs: int32(p.TargetDurationMS), EditPlanRevision: int32(p.EditPlanRevision), RenderedPlanRevision: int32(p.RenderedPlanRevision), CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	if p.EditPlan != "" {
		if plan, err := clip.DecodeEditPlan(p.EditPlan); err == nil {
			for _, n := range clip.ActivePlanNotices(plan, p.DesignSelection().RegionPresets()) {
				out.Notices = append(out.Notices, &v1.ClipNotice{Code: n.Reason, CutId: n.CutID, ElementId: n.ElementID, Action: n.Action})
			}
		}
	}
	if f := p.Finalized; f != nil {
		out.FinalizedAt = f.At.UTC().Format(time.RFC3339Nano)
		out.FinalizedPlanRevision = int32(f.PlanRevision)
		out.FinalizedResultId = f.ResultID
		out.FinalizationRefusal = "finalized"
	}
	for _, a := range p.Answers {
		out.Answers = append(out.Answers, &v1.ClipAnswer{Label: a.Label, Text: a.Text})
	}
	// Verbatim, newest first, exactly as the store answered (CLIP-133).
	for _, r := range p.Requests {
		out.Requests = append(out.Requests, &v1.ClipProjectRequest{Kind: r.Kind, Body: r.Body, CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano)})
	}
	if r := p.Result; r != nil {
		out.Result = &v1.ClipResult{Id: r.ID, ContentType: r.ContentType, Bytes: r.Bytes, DurationMs: int32(r.DurationMS), CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano), ViewUrl: r.ViewURL, DownloadUrl: r.DownloadURL}
		kind := v1.ClipRenderKind_CLIP_RENDER_KIND_SERVER
		if r.RenderKind() == clip.RenderBrowser {
			kind = v1.ClipRenderKind_CLIP_RENDER_KIND_BROWSER
		}
		out.Result.RenderKind, out.LastRenderKind = kind, &kind
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
		out = append(out, templateProto(h.service.TemplateProjection(v)))
	}
	return connect.NewResponse(&v1.ListVideoTemplatesResponse{Templates: out}), nil
}
func (h *Handler) CreateVideoTemplate(ctx context.Context, req *connect.Request[v1.CreateVideoTemplateRequest]) (*connect.Response[v1.CreateVideoTemplateResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if m.CompositionBody != nil && *m.CompositionBody == "" {
		return nil, toConnectError(&composition.Problem{ElementID: "clip", Line: 1, Reason: "root"})
	}
	value, err := h.service.CreateTemplate(ctx, user, clip.Recipe{CompositionBody: m.GetCompositionBody(), Name: m.Name, InformationFields: fields(m.InformationFields), CutGuidance: m.CutGuidance, CaptionPace: m.CaptionPace, Accent: m.Accent, Preset: m.Preset})
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
	p := clip.TemplatePatch{CompositionBody: m.CompositionBody, Name: m.Name, CutGuidance: m.CutGuidance, Accent: m.Accent, Preset: m.Preset}
	if m.InformationFields != nil {
		v := fields(m.InformationFields.Values)
		p.InformationFields = &v
	}
	p.CaptionPace = m.CaptionPace
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
			j, err := h.jobs.LatestFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: v.ID}, job.Filter{UserID: user})
			if err != nil {
				return nil, toConnectError(err)
			}
			p.LatestJob = jobrpc.ToProto(j)
		}
		h.setFinalizationState(p, v)
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
	language, err := languageFromProto(m.Language)
	if err != nil {
		return nil, toConnectError(err)
	}
	value, err := h.service.CreateProject(ctx, user, clip.ProjectInput{Language: language, CompositionInputs: compositionInputs(m.CompositionInputs), Title: m.Title, VideoTemplateID: m.VideoTemplateId, Ratio: m.Ratio, TargetDurationMS: int(m.TargetDurationMs), Disclosure: m.Disclosure, HideDisclosure: m.HideDisclosure, CTA: m.Cta, Instruction: m.Instruction, CaptionPace: m.CaptionPace, Accent: m.Accent, IntroPreset: m.IntroPreset, OutroPreset: m.OutroPreset, CaptionStyles: captionStyles(m.AllowedCaptionStyles), Answers: answers(m.Answers)})
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
	// The job status and the project snapshot are separate reads, so their ORDER
	// decides which way they may skew. The finisher commits the result and the
	// terminal job row in one transaction, so a job read FIRST is always paired
	// with a project read that already carries what that job produced. The other
	// order reports a done job whose result the snapshot has not seen yet, and a
	// poller then shows a finished clip with nothing to play.
	var latest *job.JobSummary
	if h.jobs != nil {
		latest, err = h.jobs.LatestFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: req.Msg.Id}, job.Filter{UserID: user})
		if err != nil {
			return nil, toConnectError(err)
		}
	}
	value, err := h.service.GetProject(ctx, user, req.Msg.Id)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := projectProto(value)
	// A finalized project is read in ① and ② as well as played in ③ (CLIP-160),
	// and both readings are projections of the stored plan and evidence: no
	// original is touched here, and every write stays refused where it is made.
	out.Observations = observationsProto(value)
	if h.generation != nil {
		// Unreadable evidence must not hide an otherwise downloadable result.
		// Its dependent correction projection cannot be used in that case.
		if out.Observations.Status != "unavailable" {
			state, err := h.generation.EditingState(value)
			if err != nil {
				return nil, toConnectError(err)
			}
			out.Editing = editingProto(state)
		}
		accounting, err := h.generation.Accounting(ctx, user, value.ID)
		if err != nil {
			return nil, toConnectError(err)
		}
		out.Accounting = accountingProto(accounting)
	}
	if h.jobs != nil {
		j := latest
		out.LatestJob = jobrpc.ToProto(j)
		if value.Finalized == nil && h.generation != nil && j != nil && (j.Status == "failed" || j.Status == "cancelled") {
			c, readErr := h.generation.AttemptCheckpoint(ctx, user, value.ID, j.ID)
			out.AttemptInspection = attemptInspectionProto(j.ID, j.Stage, c, readErr)
		}
		if j != nil {
			snapshot, err := h.jobs.LatestSnapshot(ctx, user, job.Subject{Dimension: clip.JobSubject, ID: value.ID})
			if err != nil {
				return nil, toConnectError(err)
			}
			if snapshot != nil && snapshot.ID == j.ID && (snapshot.DispatchReady || job.Terminal(snapshot.Status)) {
				if a := clip.IdentifyAttempt(user, value.ID, snapshot.ID, snapshot.Kind, snapshot.Payload); a != nil {
					out.LatestAttempt = &v1.ClipAttempt{JobId: a.JobID, BatchId: a.BatchID, QuoteId: a.QuoteID}
				}
			}
		}
	}
	h.setFinalizationState(out, value)
	return connect.NewResponse(&v1.GetClipProjectResponse{Project: out}), nil
}
func (h *Handler) UpdateClipProject(ctx context.Context, req *connect.Request[v1.UpdateClipProjectRequest]) (*connect.Response[v1.UpdateClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	p := clip.ProjectPatch{CompositionInputs: compositionInputs(m.CompositionInputs), Title: m.Title, VideoTemplateID: m.VideoTemplateId, Disclosure: m.Disclosure, HideDisclosure: m.HideDisclosure, CTA: m.Cta, Instruction: m.Instruction, CaptionPace: m.CaptionPace, Accent: m.Accent, IntroPreset: m.IntroPreset, OutroPreset: m.OutroPreset, CaptionStyles: captionStyles(m.AllowedCaptionStyles), Answers: answers(m.Answers)}
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
