package rpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/authoring"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/usage"
)

type Handler struct{ service *authoring.Service }

var _ postpilotv1connect.ConfigurationAuthoringServiceHandler = (*Handler)(nil)

func NewHandler(service *authoring.Service) *Handler { return &Handler{service: service} }
func owner(ctx context.Context) (string, error) {
	id, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "unauthenticated", v1.FailureReason_AUTH_REQUIRED, nil)
	}
	return id, nil
}
func kind(v v1.ConfigurationKind) authoring.Kind {
	switch v {
	case v1.ConfigurationKind_CONFIGURATION_KIND_POST_TEMPLATE:
		return authoring.PostTemplate
	case v1.ConfigurationKind_CONFIGURATION_KIND_VIDEO_TEMPLATE:
		return authoring.VideoTemplate
	case v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE:
		return authoring.PostGuideline
	case v1.ConfigurationKind_CONFIGURATION_KIND_VIDEO_GUIDELINE:
		return authoring.VideoGuideline
	case v1.ConfigurationKind_CONFIGURATION_KIND_WRITING_VOICE:
		return authoring.WritingVoice
	}
	return ""
}
func kindProto(k authoring.Kind) v1.ConfigurationKind {
	switch k {
	case authoring.PostTemplate:
		return v1.ConfigurationKind_CONFIGURATION_KIND_POST_TEMPLATE
	case authoring.VideoTemplate:
		return v1.ConfigurationKind_CONFIGURATION_KIND_VIDEO_TEMPLATE
	case authoring.PostGuideline:
		return v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE
	case authoring.VideoGuideline:
		return v1.ConfigurationKind_CONFIGURATION_KIND_VIDEO_GUIDELINE
	case authoring.WritingVoice:
		return v1.ConfigurationKind_CONFIGURATION_KIND_WRITING_VOICE
	}
	return v1.ConfigurationKind_CONFIGURATION_KIND_UNSPECIFIED
}
func mode(v v1.AuthoringMode) authoring.Mode {
	if v == v1.AuthoringMode_AUTHORING_MODE_RECOMMEND {
		return authoring.Recommend
	}
	if v == v1.AuthoringMode_AUTHORING_MODE_REFINE {
		return authoring.Refine
	}
	return ""
}
func ref(v *v1.ModelRef) llm.ModelRef {
	return llm.ModelRef{ProviderID: v.GetProviderId(), ModelID: v.GetModelId()}
}
func artifact(a authoring.Artifact) *v1.AuthoringArtifact {
	return &v1.AuthoringArtifact{Id: a.ID, Name: a.Name, Description: a.Description, Body: a.Body, TitleArea: a.TitleArea}
}
func session(s *authoring.Session) *v1.AuthoringSession {
	if s == nil {
		return nil
	}
	out := &v1.AuthoringSession{Id: s.ID, Kind: kindProto(s.Kind), Revision: s.Revision, Phase: s.Phase, ActiveJobId: s.ActiveJobID, TargetId: s.TargetID, TargetVersion: s.TargetVersion, FailureReason: s.FailureReason, PendingRequest: s.PendingRequest}
	for _, a := range s.Candidates {
		out.Candidates = append(out.Candidates, artifact(a))
	}
	if s.Selected != nil {
		out.Selected = artifact(*s.Selected)
	}
	for _, t := range s.Turns {
		out.Turns = append(out.Turns, &v1.AuthoringTurn{Id: t.ID, Request: t.Request, Reply: t.Reply, JobId: t.JobID, Status: t.Status})
	}
	if s.Saved != nil {
		out.Saved = &v1.AuthoringSavedRef{Kind: kindProto(s.Saved.Kind), Id: s.Saved.ID, Name: s.Saved.Name}
	}
	return out
}
func respond(s authoring.Session, e error) (*connect.Response[v1.AuthoringSessionResponse], error) {
	if e != nil {
		return nil, toError(e)
	}
	return connect.NewResponse(&v1.AuthoringSessionResponse{Session: session(&s)}), nil
}
func (h *Handler) CreateAuthoringSession(ctx context.Context, r *connect.Request[v1.CreateAuthoringSessionRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	s, e := h.service.Create(ctx, u, kind(r.Msg.GetKind()), r.Msg.GetTargetId(), r.Msg.GetRequestId())
	return respond(s, e)
}
func (h *Handler) GetAuthoringSession(ctx context.Context, r *connect.Request[v1.GetAuthoringSessionRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	s, e := h.service.Get(ctx, u, r.Msg.GetSessionId())
	return respond(s, e)
}
func (h *Handler) GetLatestAuthoringSession(ctx context.Context, r *connect.Request[v1.GetLatestAuthoringSessionRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	s, e := h.service.Latest(ctx, u, kind(r.Msg.GetKind()), r.Msg.GetTargetId())
	if e != nil {
		return nil, toError(e)
	}
	return connect.NewResponse(&v1.AuthoringSessionResponse{Session: session(s)}), nil
}
func (h *Handler) EstimateAuthoringOperation(ctx context.Context, r *connect.Request[v1.EstimateAuthoringOperationRequest]) (*connect.Response[v1.EstimateAuthoringOperationResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	e0, e := h.service.EstimateFor(ctx, u, kind(r.Msg.GetKind()), mode(r.Msg.GetMode()), ref(r.Msg.GetWriteModel()), r.Msg.GetSessionId())
	if e != nil {
		return nil, toError(e)
	}
	out := &v1.EstimateAuthoringOperationResponse{Free: e0.Free}
	if e0.Available {
		credits := int64(e0.Credits)
		out.Credits = &credits
	}
	return connect.NewResponse(out), nil
}
func (h *Handler) StartAuthoringOperation(ctx context.Context, r *connect.Request[v1.StartAuthoringOperationRequest]) (*connect.Response[v1.StartAuthoringOperationResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	id, s, e := h.service.Start(ctx, u, authoring.Start{SessionID: r.Msg.GetSessionId(), ExpectedRevision: r.Msg.GetExpectedRevision(), RequestID: r.Msg.GetRequestId(), Mode: mode(r.Msg.GetMode()), Prompt: r.Msg.GetPrompt(), WriteModel: ref(r.Msg.GetWriteModel())})
	if e != nil {
		return nil, toError(e)
	}
	return connect.NewResponse(&v1.StartAuthoringOperationResponse{JobId: id, Session: session(&s)}), nil
}
func (h *Handler) SelectAuthoringCandidate(ctx context.Context, r *connect.Request[v1.SelectAuthoringCandidateRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	s, e := h.service.Select(ctx, u, r.Msg.GetSessionId(), r.Msg.GetExpectedRevision(), r.Msg.GetCandidateId())
	return respond(s, e)
}
func (h *Handler) CancelAuthoringOperation(ctx context.Context, r *connect.Request[v1.CancelAuthoringOperationRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	s, e := h.service.Cancel(ctx, u, r.Msg.GetSessionId(), r.Msg.GetJobId())
	return respond(s, e)
}
func (h *Handler) SaveAuthoringSession(ctx context.Context, r *connect.Request[v1.SaveAuthoringSessionRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	u, e := owner(ctx)
	if e != nil {
		return nil, e
	}
	s, e := h.service.Save(ctx, u, r.Msg.GetSessionId(), r.Msg.GetExpectedRevision(), r.Msg.GetMakeDefault())
	return respond(s, e)
}
func toError(e error) error {
	var insufficient *plan.InsufficientCreditsError
	if errors.As(e, &insufficient) {
		return rpcserver.AppErrorFrom(connect.CodeResourceExhausted, insufficient)
	}
	if failure, ok := usage.ModelAccessFailure(e); ok {
		return rpcserver.AppErrorFrom(connect.CodeFailedPrecondition, failure)
	}
	var targetRefusal rpcserver.AppFailure
	if !errors.Is(e, authoring.ErrOutput) && errors.As(e, &targetRefusal) {
		return rpcserver.AppErrorFrom(connect.CodeFailedPrecondition, targetRefusal)
	}
	code, reason := connect.CodeFailedPrecondition, v1.FailureReason_AUTHORING_NOT_READY
	switch {
	case errors.Is(e, authoring.ErrNotFound):
		code, reason = connect.CodeNotFound, v1.FailureReason_AUTHORING_SESSION_NOT_FOUND
	case errors.Is(e, authoring.ErrStale):
		code, reason = connect.CodeAborted, v1.FailureReason_AUTHORING_REVISION_CONFLICT
	case errors.Is(e, authoring.ErrBusy):
		reason = v1.FailureReason_AUTHORING_RUNNING
	case errors.Is(e, authoring.ErrOutput):
		reason = v1.FailureReason_AUTHORING_OUTPUT_INVALID
	case errors.Is(e, authoring.ErrInvalid):
		code, reason = connect.CodeInvalidArgument, v1.FailureReason_AUTHORING_MESSAGE_INVALID
	case errors.Is(e, authoring.ErrHistoryFull):
		reason = v1.FailureReason_AUTHORING_HISTORY_FULL
	case errors.Is(e, authoring.ErrTargetConflict):
		code, reason = connect.CodeAborted, v1.FailureReason_AUTHORING_SAVE_CONFLICT
	case errors.Is(e, authoring.ErrInvalidKind):
		code, reason = connect.CodeInvalidArgument, v1.FailureReason_AUTHORING_KIND_INVALID
	case errors.Is(e, authoring.ErrModel):
		reason = v1.FailureReason_AUTHORING_MODEL_REQUIRED
	case errors.Is(e, authoring.ErrNoSelection), errors.Is(e, authoring.ErrPublication):
	default:
		slog.Error("authoring request failed", "err", e)
		code, reason = connect.CodeInternal, v1.FailureReason_UNKNOWN_FAILURE
	}
	return rpcserver.NewAppError(code, "authoring request refused", reason, nil)
}
