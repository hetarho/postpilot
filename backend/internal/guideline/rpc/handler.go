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
	preset, err := h.service.Preset(ctx, userID)
	if err != nil {
		return nil, toConnectError("list guidelines", err)
	}
	out := make([]*postpilotv1.Guideline, 0, len(guidelines))
	for _, g := range guidelines {
		out = append(out, toProtoGuideline(g))
	}
	return connect.NewResponse(&postpilotv1.ListGuidelinesResponse{Guidelines: out, Preset: toProtoPreset(preset)}), nil
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
	fields, err := fromProtoFields(req.Msg.GetFields())
	if err != nil {
		return nil, toConnectError("create guideline", err)
	}
	created, err := h.service.Create(ctx, userID, req.Msg.GetText(), guideline.ScopePatch{
		Scope: scope, TemplateIDs: req.Msg.GetTemplateIds(), Fields: fields,
	}, req.Msg.GetFromCandidateId())
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
		fields, err := fromProtoFields(sent.GetFields())
		if err != nil {
			return nil, toConnectError("update guideline", err)
		}
		patch.Scope = &guideline.ScopePatch{Scope: scope, TemplateIDs: sent.GetTemplateIds(), Fields: fields}
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

// UpdateGuidelinePreset switches the product's 상위 노출 단어 사용 preset and picks its 분야, a
// presence patch: an absent `enabled` keeps the switch, and an absent `fields` MESSAGE keeps the
// set while a present one replaces it — present and empty clears it (GUIDE-38).
func (h *Handler) UpdateGuidelinePreset(ctx context.Context, req *connect.Request[postpilotv1.UpdateGuidelinePresetRequest]) (*connect.Response[postpilotv1.UpdateGuidelinePresetResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	patch := guideline.PresetPatch{Enabled: req.Msg.Enabled}
	if sent := req.Msg.GetFields(); sent != nil {
		fields, err := fromProtoFields(sent.GetFields())
		if err != nil {
			return nil, toConnectError("update guideline preset", err)
		}
		if fields == nil {
			fields = []string{}
		}
		patch.Fields = &fields
	}
	preset, err := h.service.UpdatePreset(ctx, userID, patch)
	if err != nil {
		return nil, toConnectError("update guideline preset", err)
	}
	return connect.NewResponse(&postpilotv1.UpdateGuidelinePresetResponse{Preset: toProtoPreset(preset)}), nil
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
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return userID, nil
}

// fromProtoScope refuses UNSPECIFIED rather than defaulting to global: a client that forgot
// the field would otherwise silently save a rule that applies to every post of the account.
// Any number the enum does not name is the same refusal, never a value (ARCH-3).
func fromProtoScope(scope postpilotv1.GuidelineScope) (guideline.Scope, error) {
	switch scope {
	case postpilotv1.GuidelineScope_GUIDELINE_SCOPE_GLOBAL:
		return guideline.ScopeGlobal, nil
	case postpilotv1.GuidelineScope_GUIDELINE_SCOPE_TEMPLATES:
		return guideline.ScopeTemplates, nil
	case postpilotv1.GuidelineScope_GUIDELINE_SCOPE_FIELDS:
		return guideline.ScopeFields, nil
	}
	return "", guideline.ErrScopeShape
}

// toProtoScope maps a stored kind to the wire. Anything else is UNSPECIFIED, never GLOBAL: a
// kind this edge cannot name must not read as a rule that reaches every post.
func toProtoScope(scope guideline.Scope) postpilotv1.GuidelineScope {
	switch scope {
	case guideline.ScopeGlobal:
		return postpilotv1.GuidelineScope_GUIDELINE_SCOPE_GLOBAL
	case guideline.ScopeTemplates:
		return postpilotv1.GuidelineScope_GUIDELINE_SCOPE_TEMPLATES
	case guideline.ScopeFields:
		return postpilotv1.GuidelineScope_GUIDELINE_SCOPE_FIELDS
	}
	return postpilotv1.GuidelineScope_GUIDELINE_SCOPE_UNSPECIFIED
}

// fromProtoFields brings 분야 in through the shared mapper. An unknown number, and UNSPECIFIED —
// which names no 분야 — are both a 분야 not on the list.
func fromProtoFields(values []postpilotv1.BlogField) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		id, ok := rpcserver.BlogFieldFromProto(value)
		if !ok || id == "" {
			return nil, guideline.ErrFieldNotFound
		}
		out = append(out, id)
	}
	return out, nil
}

// toProtoFields sends 분야 out through the same mapper, dropping an id it cannot name the way
// the template projection drops a deleted template.
func toProtoFields(ids []string) []postpilotv1.BlogField {
	if len(ids) == 0 {
		return nil
	}
	out := make([]postpilotv1.BlogField, 0, len(ids))
	for _, id := range ids {
		if value, ok := rpcserver.BlogFieldToProto(id); ok && value != postpilotv1.BlogField_BLOG_FIELD_UNSPECIFIED {
			out = append(out, value)
		}
	}
	return out
}

func toProtoPreset(preset guideline.Preset) *postpilotv1.GuidelinePreset {
	return &postpilotv1.GuidelinePreset{Text: guideline.PresetText, Enabled: preset.Enabled, Fields: toProtoFields(preset.Fields)}
}

// toConnectError maps the context's sentinels to wire codes. A foreign guideline is NotFound
// like an unknown one — the two must not be distinguishable — and so is a foreign template
// and a 분야 the product does not list.
func toConnectError(op string, err error) error {
	var tooLong *guideline.TextTooLongError
	var atCap *guideline.AccountCapError
	switch {
	case errors.As(err, &tooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "guideline text is too long", postpilotv1.FailureReason_GUIDELINE_TEXT_TOO_LONG, map[string]string{
			"max": strconv.Itoa(tooLong.Max), "actual": strconv.Itoa(tooLong.Chars),
		})
	case errors.As(err, &atCap):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "guideline limit reached", postpilotv1.FailureReason_GUIDELINE_LIMIT_REACHED, map[string]string{
			"max": strconv.Itoa(atCap.Max),
		})
	case errors.Is(err, guideline.ErrInvalidText):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "guideline text is required", postpilotv1.FailureReason_GUIDELINE_TEXT_REQUIRED, nil)
	case errors.Is(err, guideline.ErrScopeShape):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "guideline scope is invalid", postpilotv1.FailureReason_GUIDELINE_SCOPE_INVALID, nil)
	case errors.Is(err, guideline.ErrDuplicateText):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "guideline text already exists", postpilotv1.FailureReason_GUIDELINE_TEXT_TAKEN, nil)
	case errors.Is(err, guideline.ErrTemplateNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "scoped template not found", postpilotv1.FailureReason_GUIDELINE_TEMPLATE_NOT_FOUND, nil)
	case errors.Is(err, guideline.ErrFieldNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "scoped blog field not found", postpilotv1.FailureReason_GUIDELINE_FIELD_NOT_FOUND, nil)
	case errors.Is(err, guideline.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "guideline not found", postpilotv1.FailureReason_GUIDELINE_NOT_FOUND, nil)
	case errors.Is(err, guideline.ErrCandidateNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "guideline candidate not found", postpilotv1.FailureReason_GUIDELINE_CANDIDATE_NOT_FOUND, nil)
	default:
		slog.Error(op+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, op+" failed", postpilotv1.FailureReason_UNKNOWN_FAILURE, nil)
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
	return &postpilotv1.Guideline{
		Id: g.ID, Text: g.Text, Scope: toProtoScope(g.Scope), Templates: templates, Fields: toProtoFields(g.Fields),
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
