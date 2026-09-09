// Package rpc is the authenticated clip transport boundary.
package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"log/slog"
	"time"
)

type Handler struct {
	service *clip.Service
	sources *clip.SourceService
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
	switch {
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
	out := &v1.VideoTemplate{Id: t.ID, Name: t.Name, CutGuidance: t.CutGuidance, CopyStyles: t.CopyStyles, Accent: t.Accent, ProjectCount: int32(t.ProjectCount), CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: t.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	for _, f := range t.InformationFields {
		out.InformationFields = append(out.InformationFields, &v1.ClipInformationField{Label: f.Label, Prompt: f.Prompt})
	}
	return out
}
func projectProto(p clip.Project) *v1.ClipProject {
	out := &v1.ClipProject{Id: p.ID, Title: p.Title, VideoTemplateId: p.VideoTemplateID, Ratio: p.Ratio, TargetDurationMs: int32(p.TargetDurationMS), EditPlanRevision: int32(p.EditPlanRevision), RenderedPlanRevision: int32(p.RenderedPlanRevision), CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339Nano)}
	for _, a := range p.Answers {
		out.Answers = append(out.Answers, &v1.ClipAnswer{Label: a.Label, Text: a.Text})
	}
	if r := p.Result; r != nil {
		out.Result = &v1.ClipResult{ContentType: r.ContentType, Bytes: r.Bytes, DurationMs: int32(r.DurationMS), CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano)}
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
	value, err := h.service.CreateTemplate(ctx, user, clip.Recipe{Name: m.Name, InformationFields: fields(m.InformationFields), CutGuidance: m.CutGuidance, CopyStyles: m.CopyStyles, Accent: m.Accent})
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
	p := clip.TemplatePatch{Name: m.Name, CutGuidance: m.CutGuidance, Accent: m.Accent}
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
		out = append(out, projectProto(v))
	}
	return connect.NewResponse(&v1.ListClipProjectsResponse{Projects: out}), nil
}
func (h *Handler) CreateClipProject(ctx context.Context, req *connect.Request[v1.CreateClipProjectRequest]) (*connect.Response[v1.CreateClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	value, err := h.service.CreateProject(ctx, user, clip.ProjectInput{Title: m.Title, VideoTemplateID: m.VideoTemplateId, Ratio: m.Ratio, TargetDurationMS: int(m.TargetDurationMs), Answers: answers(m.Answers)})
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
	return connect.NewResponse(&v1.GetClipProjectResponse{Project: projectProto(value)}), nil
}
func (h *Handler) UpdateClipProject(ctx context.Context, req *connect.Request[v1.UpdateClipProjectRequest]) (*connect.Response[v1.UpdateClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	p := clip.ProjectPatch{Title: m.Title, VideoTemplateID: m.VideoTemplateId, Answers: answers(m.Answers)}
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
