// Package rpc is the guideline context's authenticated Connect edge.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

type Handler struct{ service *guideline.Service }

func NewHandler(service *guideline.Service) *Handler { return &Handler{service: service} }

func (h *Handler) ListGuidelines(ctx context.Context, _ *connect.Request[postpilotv1.ListGuidelinesRequest]) (*connect.Response[postpilotv1.ListGuidelinesResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	guidelines, err := h.service.List(ctx, userID)
	if err != nil {
		return nil, toConnectError("list guidelines", err)
	}
	out := make([]*postpilotv1.Guideline, 0, len(guidelines))
	for _, g := range guidelines {
		out = append(out, toProtoGuideline(g))
	}
	return connect.NewResponse(&postpilotv1.ListGuidelinesResponse{Guidelines: out}), nil
}

func (h *Handler) CreateGuideline(ctx context.Context, req *connect.Request[postpilotv1.CreateGuidelineRequest]) (*connect.Response[postpilotv1.CreateGuidelineResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	scope, err := fromProtoScope(req.Msg.GetScope())
	if err != nil {
		return nil, toConnectError("create guideline", err)
	}
	created, err := h.service.Create(ctx, userID, req.Msg.GetText(), scope, req.Msg.GetTemplateIds(), req.Msg.GetFromCandidateId())
	if err != nil {
		return nil, toConnectError("create guideline", err)
	}
	return connect.NewResponse(&postpilotv1.CreateGuidelineResponse{Guideline: toProtoGuideline(created)}), nil
}

func (h *Handler) UpdateGuideline(ctx context.Context, req *connect.Request[postpilotv1.UpdateGuidelineRequest]) (*connect.Response[postpilotv1.UpdateGuidelineResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	// Presence is the edit unit. For the scope that means MESSAGE presence: an absent patch
	// leaves the scope alone, and a present one replaces the kind and the whole set together.
	patch := guideline.Patch{}
	if req.Msg.Text != nil {
		patch.Text = req.Msg.Text
	}
	if sent := req.Msg.GetScope(); sent != nil {
		scope, err := fromProtoScope(sent.GetScope())
		if err != nil {
			return nil, toConnectError("update guideline", err)
		}
		patch.Scope = &guideline.ScopePatch{Scope: scope, TemplateIDs: sent.GetTemplateIds()}
	}
	updated, err := h.service.Update(ctx, userID, req.Msg.GetId(), patch)
	if err != nil {
		return nil, toConnectError("update guideline", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateGuidelineResponse{Guideline: toProtoGuideline(updated)}), nil
}

func (h *Handler) DeleteGuideline(ctx context.Context, req *connect.Request[postpilotv1.DeleteGuidelineRequest]) (*connect.Response[postpilotv1.DeleteGuidelineResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.Delete(ctx, userID, req.Msg.GetId()); err != nil {
		return nil, toConnectError("delete guideline", err)
	}
	return connect.NewResponse(&postpilotv1.DeleteGuidelineResponse{}), nil
}

// ListGuidelineCandidates serves the review list. queue_full comes from the server because
// the pending bound is server-side: the client relays it rather than predicting it, exactly
// as it does for the account guideline cap.
func (h *Handler) ListGuidelineCandidates(ctx context.Context, _ *connect.Request[postpilotv1.ListGuidelineCandidatesRequest]) (*connect.Response[postpilotv1.ListGuidelineCandidatesResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	candidates, queueFull, err := h.service.ListCandidates(ctx, userID)
	if err != nil {
		return nil, toConnectError("list guideline candidates", err)
	}
	out := make([]*postpilotv1.GuidelineCandidate, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, toProtoCandidate(c))
	}
	return connect.NewResponse(&postpilotv1.ListGuidelineCandidatesResponse{Candidates: out, QueueFull: queueFull}), nil
}

func (h *Handler) DismissGuidelineCandidate(ctx context.Context, req *connect.Request[postpilotv1.DismissGuidelineCandidateRequest]) (*connect.Response[postpilotv1.DismissGuidelineCandidateResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.DismissCandidate(ctx, userID, req.Msg.GetId()); err != nil {
		return nil, toConnectError("dismiss guideline candidate", err)
	}
	return connect.NewResponse(&postpilotv1.DismissGuidelineCandidateResponse{}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return userID, nil
}

// fromProtoScope refuses UNSPECIFIED rather than defaulting to global: a client that forgot
// the field would otherwise silently save a rule that applies to every post of the account.
func fromProtoScope(scope postpilotv1.GuidelineScope) (guideline.Scope, error) {
	switch scope {
	case postpilotv1.GuidelineScope_GUIDELINE_SCOPE_GLOBAL:
		return guideline.ScopeGlobal, nil
	case postpilotv1.GuidelineScope_GUIDELINE_SCOPE_TEMPLATES:
		return guideline.ScopeTemplates, nil
	default:
		return "", guideline.ErrScopeShape
	}
}

// toConnectError maps the context's sentinels to wire codes. A foreign guideline is NotFound
// like an unknown one — the two must not be distinguishable — and so is a foreign template.
func toConnectError(op string, err error) error {
	var tooLong *guideline.TextTooLongError
	var atCap *guideline.AccountCapError
	switch {
	case errors.As(err, &tooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "guideline text is too long", "GUIDELINE_TEXT_TOO_LONG", map[string]string{
			"max": strconv.Itoa(tooLong.Max), "actual": strconv.Itoa(tooLong.Chars),
		})
	case errors.As(err, &atCap):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "guideline limit reached", "GUIDELINE_LIMIT_REACHED", map[string]string{
			"max": strconv.Itoa(atCap.Max),
		})
	case errors.Is(err, guideline.ErrInvalidText):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "guideline text is required", "GUIDELINE_TEXT_REQUIRED", nil)
	case errors.Is(err, guideline.ErrScopeShape):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "guideline scope is invalid", "GUIDELINE_SCOPE_INVALID", nil)
	case errors.Is(err, guideline.ErrDuplicateText):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "guideline text already exists", "GUIDELINE_TEXT_TAKEN", nil)
	case errors.Is(err, guideline.ErrTemplateNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "scoped template not found", "GUIDELINE_TEMPLATE_NOT_FOUND", nil)
	case errors.Is(err, guideline.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "guideline not found", "GUIDELINE_NOT_FOUND", nil)
	case errors.Is(err, guideline.ErrCandidateNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "guideline candidate not found", "GUIDELINE_CANDIDATE_NOT_FOUND", nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", "UNKNOWN_FAILURE", nil)
	}
}

func toProtoGuideline(g guideline.Guideline) *postpilotv1.Guideline {
	if g.ID == "" {
		return nil
	}
	templates := make([]*postpilotv1.GuidelineTemplateRef, 0, len(g.Templates))
	for _, ref := range g.Templates {
		templates = append(templates, &postpilotv1.GuidelineTemplateRef{Id: ref.ID, Name: ref.Name})
	}
	scope := postpilotv1.GuidelineScope_GUIDELINE_SCOPE_GLOBAL
	if g.Scope == guideline.ScopeTemplates {
		scope = postpilotv1.GuidelineScope_GUIDELINE_SCOPE_TEMPLATES
	}
	return &postpilotv1.Guideline{
		Id: g.ID, Text: g.Text, Scope: scope, Templates: templates,
		CreatedAt: g.CreatedAt.UTC().Format(timeLayout), UpdatedAt: g.UpdatedAt.UTC().Format(timeLayout),
	}
}

func toProtoCandidate(c guideline.Candidate) *postpilotv1.GuidelineCandidate {
	return &postpilotv1.GuidelineCandidate{
		Id: c.ID, Text: c.Text, PostSlug: c.PostSlug, Occurrences: int32(c.Occurrences),
		FirstSeenAt: c.FirstSeenAt.UTC().Format(timeLayout),
		LastSeenAt:  c.LastSeenAt.UTC().Format(timeLayout),
	}
}
