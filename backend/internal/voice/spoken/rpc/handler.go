// Package rpc maps owner-scoped spoken library operations to Connect.
package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/auth"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/voice/spoken"
	"time"
)

type Handler struct{ svc *spoken.Service }

var _ postpilotv1connect.SpokenVoiceServiceHandler = (*Handler)(nil)

func NewHandler(svc *spoken.Service) *Handler { return &Handler{svc: svc} }
func actor(ctx context.Context) (string, plan.Plan, error) {
	owner, ok := auth.UserFromContext(ctx)
	tier, known := auth.PlanFromContext(ctx)
	if !ok || !known {
		return "", "", connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	}
	return owner, tier, nil
}
func failure(err error) error {
	if err == nil {
		return nil
	}
	code := connect.CodeFailedPrecondition
	message := "spoken voice unavailable"
	switch {
	case errors.Is(err, spoken.ErrNotFound):
		code = connect.CodeNotFound
		message = "spoken voice not found"
	case errors.Is(err, spoken.ErrInvalid):
		code = connect.CodeInvalidArgument
		message = "invalid spoken voice input"
	case errors.Is(err, spoken.ErrConflict):
		code = connect.CodeAborted
		message = "spoken voice revision changed"
	case errors.Is(err, spoken.ErrImmutable):
		message = "confirmed sound cannot be changed"
	case errors.Is(err, spoken.ErrAuditionRequired):
		message = "play and select the candidate before confirming"
	}
	return connect.NewError(code, errors.New(message))
}
func input(p *v1.SpokenDraftInput) spoken.DraftInput {
	return spoken.DraftInput{Name: p.GetName(), Description: p.GetDescription(), PreviewText: p.GetPreviewText(), ProfileID: p.GetProfileId(), ProfileRevision: p.GetProfileRevision(), QualificationSessionID: p.GetQualificationSessionId()}
}
func (h *Handler) ListSpokenDrafts(ctx context.Context, _ *connect.Request[v1.ListSpokenDraftsRequest]) (*connect.Response[v1.ListSpokenDraftsResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.ListDrafts(ctx, owner)
	if err != nil {
		return nil, failure(err)
	}
	out := &v1.ListSpokenDraftsResponse{}
	for _, d := range ds {
		out.Drafts = append(out.Drafts, draft(d))
	}
	return connect.NewResponse(out), nil
}
func draftResponse(d spoken.Draft, err error) (*connect.Response[v1.SpokenDraftResponse], error) {
	if err != nil {
		return nil, failure(err)
	}
	return connect.NewResponse(&v1.SpokenDraftResponse{Draft: draft(d)}), nil
}
func voiceResponse(v spoken.Voice, err error) (*connect.Response[v1.SpokenVoiceResponse], error) {
	if err != nil {
		return nil, failure(err)
	}
	return connect.NewResponse(&v1.SpokenVoiceResponse{Voice: voice(v)}), nil
}
func (h *Handler) GetSpokenDraft(ctx context.Context, r *connect.Request[v1.SpokenIDRequest]) (*connect.Response[v1.SpokenDraftResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return draftResponse(h.svc.GetDraft(ctx, owner, r.Msg.Id))
}
func (h *Handler) CreateSpokenDraft(ctx context.Context, r *connect.Request[v1.CreateSpokenDraftRequest]) (*connect.Response[v1.SpokenDraftResponse], error) {
	owner, tier, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return draftResponse(h.svc.CreateDraft(ctx, owner, tier, r.Msg.IdempotencyKey, input(r.Msg.Input)))
}
func (h *Handler) UpdateSpokenDraft(ctx context.Context, r *connect.Request[v1.UpdateSpokenDraftRequest]) (*connect.Response[v1.SpokenDraftResponse], error) {
	owner, tier, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return draftResponse(h.svc.UpdateDraft(ctx, owner, tier, r.Msg.Id, r.Msg.ExpectedRevision, r.Msg.IdempotencyKey, input(r.Msg.Input)))
}
func (h *Handler) DeleteSpokenDraft(ctx context.Context, r *connect.Request[v1.SpokenMutationRequest]) (*connect.Response[v1.SpokenEmptyResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteDraft(ctx, owner, r.Msg.Id, r.Msg.ExpectedRevision, r.Msg.IdempotencyKey); err != nil {
		return nil, failure(err)
	}
	return connect.NewResponse(&v1.SpokenEmptyResponse{}), nil
}
func (h *Handler) SelectSpokenCandidate(ctx context.Context, r *connect.Request[v1.SpokenCandidateRequest]) (*connect.Response[v1.SpokenDraftResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return draftResponse(h.svc.SelectCandidate(ctx, owner, r.Msg.DraftId, r.Msg.ExpectedRevision, r.Msg.IdempotencyKey, r.Msg.CandidateId))
}
func (h *Handler) AcknowledgeSpokenCandidate(ctx context.Context, r *connect.Request[v1.SpokenCandidateRequest]) (*connect.Response[v1.SpokenDraftResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return draftResponse(h.svc.AcknowledgeCandidate(ctx, owner, r.Msg.DraftId, r.Msg.ExpectedRevision, r.Msg.IdempotencyKey, r.Msg.CandidateId, r.Msg.PlaybackId))
}
func (h *Handler) ListSpokenVoices(ctx context.Context, r *connect.Request[v1.ListSpokenVoicesRequest]) (*connect.Response[v1.ListSpokenVoicesResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	vs, err := h.svc.ListVoices(ctx, owner, r.Msg.IncludeRemoved)
	if err != nil {
		return nil, failure(err)
	}
	out := &v1.ListSpokenVoicesResponse{}
	for _, v := range vs {
		out.Voices = append(out.Voices, voice(v))
	}
	return connect.NewResponse(out), nil
}
func (h *Handler) GetSpokenVoice(ctx context.Context, r *connect.Request[v1.SpokenIDRequest]) (*connect.Response[v1.SpokenVoiceResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return voiceResponse(h.svc.GetVoice(ctx, owner, r.Msg.Id))
}
func (h *Handler) RenameSpokenVoice(ctx context.Context, r *connect.Request[v1.RenameSpokenVoiceRequest]) (*connect.Response[v1.SpokenVoiceResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return voiceResponse(h.svc.RenameVoice(ctx, owner, r.Msg.Id, r.Msg.ExpectedRevision, r.Msg.IdempotencyKey, r.Msg.Name))
}
func (h *Handler) RemoveSpokenVoice(ctx context.Context, r *connect.Request[v1.SpokenMutationRequest]) (*connect.Response[v1.SpokenVoiceResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	return voiceResponse(h.svc.RemoveVoice(ctx, owner, r.Msg.Id, r.Msg.ExpectedRevision, r.Msg.IdempotencyKey))
}
func (h *Handler) GetSpokenSampleAccess(ctx context.Context, r *connect.Request[v1.SpokenIDRequest]) (*connect.Response[v1.SpokenSampleAccessResponse], error) {
	owner, _, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.svc.SampleAccess(ctx, owner, r.Msg.Id)
	if err != nil {
		return nil, failure(err)
	}
	return connect.NewResponse(&v1.SpokenSampleAccessResponse{PlaybackId: p.ID, Url: SamplePath + p.ID, ExpiresAt: p.ExpiresAt.UTC().Format(time.RFC3339Nano)}), nil
}
