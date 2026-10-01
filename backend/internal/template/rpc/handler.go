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
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/usage"
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
	created, err := h.service.Create(ctx, userID, template.Authored{
		Name: req.Msg.GetName(), Description: req.Msg.GetDescription(), Body: req.Msg.GetBody(), TitleArea: req.Msg.GetTitleArea(),
		Numbers: template.Numbers{TargetLength: number(req.Msg.TargetLength), TagCount: number(req.Msg.TagCount)},
	})
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
	//
	// The two generation numbers are the exception (TMPL-8): they travel as one pair that
	// is always written, an absent member meaning "no opinion" rather than "not part of this
	// edit", because the template screen holds both and sends both on every save.
	patch := template.Patch{
		Name: req.Msg.Name, Description: req.Msg.Description, Body: req.Msg.Body, TitleArea: req.Msg.TitleArea,
		Numbers: &template.Numbers{
			TargetLength: number(req.Msg.TargetLength), TagCount: number(req.Msg.TagCount),
		},
	}
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

// GetFormatGuide serves the 형식 안내 in the reader's language (TMPL-41). It needs a session like
// every template procedure, and it reads nothing but the process's own limits.
func (h *Handler) GetFormatGuide(ctx context.Context, req *connect.Request[postpilotv1.GetFormatGuideRequest]) (*connect.Response[postpilotv1.GetFormatGuideResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	language, err := guideLanguage(req.Msg.GetLanguage())
	if err != nil {
		return nil, err
	}
	text, err := h.service.FormatGuide(language)
	if err != nil {
		return nil, toConnectError("get format guide", err)
	}
	return connect.NewResponse(&postpilotv1.GetFormatGuideResponse{Text: text}), nil
}

// StartTemplateRequest enqueues a template request on the write model the client names
// (TMPL-58). Every refusal it can state — the box, the draft, the template, the cap, the model,
// the sample — comes before the credit hold; plan and credit refusals come from the hold.
func (h *Handler) StartTemplateRequest(ctx context.Context, req *connect.Request[postpilotv1.StartTemplateRequestRequest]) (*connect.Response[postpilotv1.StartTemplateRequestResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	language, err := guideLanguage(req.Msg.GetLanguage())
	if err != nil {
		return nil, err
	}
	draft := req.Msg.GetDraft()
	jobID, err := h.service.StartRequest(ctx, userID, template.StartRequest{
		WriteModel: modelRefValue(req.Msg.GetWriteModel()), Language: language, Text: req.Msg.GetText(),
		Draft: template.Draft{
			Name: draft.GetName(), Description: draft.GetDescription(), TitleArea: draft.GetTitleArea(), Body: draft.GetBody(),
		},
		TemplateID: req.Msg.GetTemplateId(), SamplePostSlug: req.Msg.GetSamplePostSlug(),
	})
	if err != nil {
		return nil, toConnectError("start template request", err)
	}
	return connect.NewResponse(&postpilotv1.StartTemplateRequestResponse{JobId: jobID}), nil
}

// CancelTemplateRequest stops the owner's queued or running request (TMPL-63); a finished one
// answers OK and changes nothing, since the editor discards its result anyway.
func (h *Handler) CancelTemplateRequest(ctx context.Context, req *connect.Request[postpilotv1.CancelTemplateRequestRequest]) (*connect.Response[postpilotv1.CancelTemplateRequestResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.CancelRequest(ctx, userID, req.Msg.GetJobId()); err != nil {
		return nil, toConnectError("cancel template request", err)
	}
	return connect.NewResponse(&postpilotv1.CancelTemplateRequestResponse{}), nil
}

// GetTemplateRequestResult reads a finished request's answer for its owner.
func (h *Handler) GetTemplateRequestResult(ctx context.Context, req *connect.Request[postpilotv1.GetTemplateRequestResultRequest]) (*connect.Response[postpilotv1.GetTemplateRequestResultResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	result, err := h.service.RequestResult(ctx, userID, req.Msg.GetJobId())
	if err != nil {
		return nil, toConnectError("get template request result", err)
	}
	return connect.NewResponse(&postpilotv1.GetTemplateRequestResultResponse{
		Draft: &postpilotv1.TemplateDraft{
			Name: result.Name, Description: result.Description, TitleArea: result.TitleArea, Body: result.Body,
		},
		Wishes: result.Wishes,
	}), nil
}

// EstimateTemplateRequest states 무료 or 약 n 크레딧 for one request on the named write model
// (QUOTA-67). A model with no figure answers with neither, which the box shows as no figure.
func (h *Handler) EstimateTemplateRequest(ctx context.Context, req *connect.Request[postpilotv1.EstimateTemplateRequestRequest]) (*connect.Response[postpilotv1.EstimateTemplateRequestResponse], error) {
	if _, err := actingUser(ctx); err != nil {
		return nil, err
	}
	estimate, err := h.service.EstimateRequest(ctx, modelRefValue(req.Msg.GetWriteModel()))
	if err != nil {
		return nil, toConnectError("estimate template request", err)
	}
	out := &postpilotv1.EstimateTemplateRequestResponse{Free: estimate.Free}
	if estimate.Available && !estimate.Free {
		credits := int32(estimate.Credits)
		out.Credits = &credits
	}
	return connect.NewResponse(out), nil
}

// guideLanguage is the reader's language, which every guide-teaching procedure needs and none
// guesses.
func guideLanguage(value postpilotv1.ContentLanguage) (template.Language, error) {
	switch value {
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:
		return template.LanguageKorean, nil
	case postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH:
		return template.LanguageEnglish, nil
	default:
		return "", rpcserver.NewAppError(connect.CodeInvalidArgument, "guide language is required", postpilotv1.FailureReason_CONTENT_LANGUAGE_REQUIRED, nil)
	}
}

func modelRefValue(ref *postpilotv1.ModelRef) string {
	if ref == nil || ref.GetProviderId() == "" || ref.GetModelId() == "" {
		return ""
	}
	return ref.GetProviderId() + "/" + ref.GetModelId()
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return userID, nil
}

// toConnectError maps the context's sentinels to wire codes. A foreign template is NotFound
// like an unknown one — the two must not be distinguishable.
//
// A parse failure carries the line, the reason and the area as allowlisted params, because the
// editor has to point at the offending line in the right area and it must not parse wire prose
// to find out which one (TMPL-20).
func toConnectError(op string, err error) error {
	// The plan and credit refusals come from the job-enqueue hold, so they translate exactly as
	// generation's do wherever that seam surfaces them.
	if access, ok := usage.ModelAccessFailure(err); ok {
		return rpcserver.AppErrorFrom(connect.CodeFailedPrecondition, access)
	}
	var credits *plan.InsufficientCreditsError
	if errors.As(err, &credits) {
		return rpcserver.AppErrorFrom(connect.CodeResourceExhausted, credits)
	}
	var tooLong *template.FieldTooLongError
	var outOfRange *template.NumberOutOfRangeError
	var parseErr *template.ParseError
	switch {
	case errors.As(err, &outOfRange):
		// `max` is omitted for a number with no ceiling: a params key claiming one would be a
		// rule nobody set.
		params := map[string]string{
			"field": outOfRange.Field, "min": strconv.Itoa(outOfRange.Min),
			"actual": strconv.Itoa(outOfRange.Value),
		}
		if outOfRange.Max > 0 {
			params["max"] = strconv.Itoa(outOfRange.Max)
		}
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template number is out of range", postpilotv1.FailureReason_TEMPLATE_NUMBER_OUT_OF_RANGE, params)
	case errors.As(err, &tooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template field is too long", postpilotv1.FailureReason_TEMPLATE_FIELD_TOO_LONG, map[string]string{
			"field": tooLong.Field, "max": strconv.Itoa(tooLong.Max), "actual": strconv.Itoa(tooLong.Chars),
		})
	case errors.As(err, &parseErr):
		params := map[string]string{"line": strconv.Itoa(parseErr.Line), "reason": parseErr.Reason, "area": parseErr.Area}
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template does not parse", postpilotv1.FailureReason_TEMPLATE_PARSE_FAILED, params)
	case errors.Is(err, template.ErrNameRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template name is required", postpilotv1.FailureReason_TEMPLATE_NAME_REQUIRED, nil)
	case errors.Is(err, template.ErrBodyRequired):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template body is required", postpilotv1.FailureReason_TEMPLATE_BODY_REQUIRED, nil)
	case errors.Is(err, template.ErrDuplicateName):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "template name already exists", postpilotv1.FailureReason_TEMPLATE_NAME_TAKEN, nil)
	case errors.Is(err, template.ErrTooMany):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "template limit reached", postpilotv1.FailureReason_TEMPLATE_LIMIT_REACHED, nil)
	case errors.Is(err, template.ErrRequestEmpty):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "template request is empty", postpilotv1.FailureReason_TEMPLATE_REQUEST_EMPTY, nil)
	case errors.Is(err, template.ErrRequestRunning):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "a template request is already running", postpilotv1.FailureReason_TEMPLATE_REQUEST_RUNNING, nil)
	case errors.Is(err, template.ErrSampleUnavailable):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "sample post cannot be read", postpilotv1.FailureReason_TEMPLATE_SAMPLE_UNAVAILABLE, nil)
	case errors.Is(err, template.ErrWriteModelRequired):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "an enabled write model is required", postpilotv1.FailureReason_GENERATION_WRITE_MODEL_REQUIRED, nil)
	case errors.Is(err, template.ErrRequestNotReady):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "template request has no result", postpilotv1.FailureReason_TEMPLATE_REQUEST_NOT_READY, nil)
	case errors.Is(err, template.ErrUnsupportedLanguage):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "guide language is required", postpilotv1.FailureReason_CONTENT_LANGUAGE_REQUIRED, nil)
	case errors.Is(err, template.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "template not found", postpilotv1.FailureReason_TEMPLATE_NOT_FOUND, nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
	}
}

func toProtoTemplate(t template.Template) *postpilotv1.Template {
	if t.ID == "" {
		return nil
	}
	return &postpilotv1.Template{
		Id: t.ID, Name: t.Name, Description: t.Description, Body: t.Body, TitleArea: t.TitleArea,
		TargetLength: protoNumber(t.TargetLength), TagCount: protoNumber(t.TagCount),
		PostCount: int32(t.PostCount),
		CreatedAt: t.CreatedAt.UTC().Format(timeLayout),
		UpdatedAt: t.UpdatedAt.UTC().Format(timeLayout),
	}
}

// number and protoNumber carry "no opinion" across the wire edge unchanged: an absent field
// is a template that says nothing about that number, never a zero.
func number(value *int32) *int {
	if value == nil {
		return nil
	}
	out := int(*value)
	return &out
}

func protoNumber(value *int) *int32 {
	if value == nil {
		return nil
	}
	out := int32(*value)
	return &out
}
