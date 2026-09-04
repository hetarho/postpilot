// Package rpc is the template context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/template"
)

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Handler struct{ service *template.Service }

func NewHandler(service *template.Service) *Handler { return &Handler{service: service} }

func (h *Handler) ListTemplates(ctx context.Context, _ *connect.Request[postpilotv1.ListTemplatesRequest]) (*connect.Response[postpilotv1.ListTemplatesResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	templates, err := h.service.List(ctx, userID)
	if err != nil {
		return nil, toConnectError("list templates", err)
	}
	out := make([]*postpilotv1.Template, 0, len(templates))
	for _, t := range templates {
		out = append(out, toProtoTemplate(t))
	}
	return connect.NewResponse(&postpilotv1.ListTemplatesResponse{Templates: out}), nil
}

func (h *Handler) CreateTemplate(ctx context.Context, req *connect.Request[postpilotv1.CreateTemplateRequest]) (*connect.Response[postpilotv1.CreateTemplateResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	created, err := h.service.Create(ctx, userID, req.Msg.GetName(), req.Msg.GetDescription(), req.Msg.GetBody())
	if err != nil {
		return nil, toConnectError("create template", err)
	}
	return connect.NewResponse(&postpilotv1.CreateTemplateResponse{Template: toProtoTemplate(created)}), nil
}

func (h *Handler) UpdateTemplate(ctx context.Context, req *connect.Request[postpilotv1.UpdateTemplateRequest]) (*connect.Response[postpilotv1.UpdateTemplateResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	// Presence is the edit unit: a field the request did not carry is never named by any
	// statement, so two fields edited from two tabs cannot overwrite each other.
	patch := template.Patch{Name: req.Msg.Name, Description: req.Msg.Description, Body: req.Msg.Body}
	updated, err := h.service.Update(ctx, userID, req.Msg.GetId(), patch)
	if err != nil {
		return nil, toConnectError("update template", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateTemplateResponse{Template: toProtoTemplate(updated)}), nil
}

func (h *Handler) DeleteTemplate(ctx context.Context, req *connect.Request[postpilotv1.DeleteTemplateRequest]) (*connect.Response[postpilotv1.DeleteTemplateResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	detached, err := h.service.Delete(ctx, userID, req.Msg.GetId())
	if err != nil {
		return nil, toConnectError("delete template", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteTemplateResponse{DetachedPosts: int32(detached)}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return userID, nil
}

// toConnectError maps the context's sentinels to wire codes. A foreign template is NotFound
// like an unknown one — the two must not be distinguishable.
//
// A parse failure carries the line and the reason as allowlisted params, because the editor
// has to point at the offending line and it must not parse wire prose to find out which one.
func toConnectError(op string, err error) error {
	var tooLong *template.FieldTooLongError
	var parseErr *template.ParseError
	switch {
	case errors.As(err, &tooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template field is too long", "TEMPLATE_FIELD_TOO_LONG", map[string]string{
			"field": tooLong.Field, "max": strconv.Itoa(tooLong.Max), "actual": strconv.Itoa(tooLong.Chars),
		})
	case errors.As(err, &parseErr):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template body does not parse", "TEMPLATE_PARSE_FAILED", map[string]string{
			"line": strconv.Itoa(parseErr.Line), "reason": parseErr.Reason,
		})
	case errors.Is(err, template.ErrNameRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template name is required", "TEMPLATE_NAME_REQUIRED", nil)
	case errors.Is(err, template.ErrBodyRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template body is required", "TEMPLATE_BODY_REQUIRED", nil)
	case errors.Is(err, template.ErrDuplicateName):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "template name already exists", "TEMPLATE_NAME_TAKEN", nil)
	case errors.Is(err, template.ErrTooMany):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "template limit reached", "TEMPLATE_LIMIT_REACHED", nil)
	case errors.Is(err, template.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "template not found", "TEMPLATE_NOT_FOUND", nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", "UNKNOWN_FAILURE", nil)
	}
}

func toProtoTemplate(t template.Template) *postpilotv1.Template {
	if t.ID == "" {
		return nil
	}
	return &postpilotv1.Template{
		Id: t.ID, Name: t.Name, Description: t.Description, Body: t.Body,
		PostCount: int32(t.PostCount),
		CreatedAt: t.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt: t.UpdatedAt.UTC().Format(timeLayout),
	}
}
