package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func spokenRecoveryProto(r *clip.RecoveryState) *v1.ClipSpokenDraft {
	out := &v1.ClipSpokenDraft{}
	if r != nil && r.Spoken != nil {
		out.Digest = clip.RecoveryDigest(r)
		out.Narration = narrationProto(&r.Spoken.Narration)
	}
	return out
}
func (h *Handler) GetClipSpokenDraft(ctx context.Context, req *connect.Request[v1.GetClipSpokenDraftRequest]) (*connect.Response[v1.ClipSpokenDraft], error) {
	owner, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	r, e := h.generation.SpokenRecovery(ctx, owner, req.Msg.ProjectId)
	if e != nil {
		return nil, toConnectError(e)
	}
	return connect.NewResponse(spokenRecoveryProto(r)), nil
}
func (h *Handler) SaveClipSpokenDraft(ctx context.Context, req *connect.Request[v1.SaveClipSpokenDraftRequest]) (*connect.Response[v1.ClipSpokenDraft], error) {
	owner, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.generation == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	r, e := h.generation.CorrectSpokenRecovery(ctx, owner, req.Msg.ProjectId, req.Msg.ExpectedDigest, narration(req.Msg.Narration))
	if e != nil {
		return nil, toConnectError(e)
	}
	return connect.NewResponse(spokenRecoveryProto(r)), nil
}
