package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/usage"
	"time"
)

func (h *Handler) WithSpeech(s *clipapp.SpeechService) *Handler { h.speech = s; return h }
func (h *Handler) QuoteClipSpeech(ctx context.Context, r *connect.Request[v1.QuoteClipSpeechRequest]) (*connect.Response[v1.ClipSpeechQuote], error) {
	owner, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.speech == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	q, e := h.speech.Quote(ctx, owner, r.Msg.ProjectId, int(r.Msg.ExpectedRevision))
	if e != nil {
		return nil, toConnectError(e)
	}
	expires := ""
	if !q.ExpiresAt.IsZero() {
		expires = q.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	return connect.NewResponse(&v1.ClipSpeechQuote{QuoteId: q.ID, MaximumCredits: int32(q.MaximumCredits), ExpiresAt: expires, SegmentIds: q.SegmentIDs, PlanRevision: int32(q.Revision), CancellationPolicyVersion: usage.UnitCancellationPolicyVersion}), nil
}
func (h *Handler) StartClipSpeech(ctx context.Context, r *connect.Request[v1.StartClipSpeechRequest]) (*connect.Response[v1.StartClipSpeechResponse], error) {
	owner, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.speech == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	a := usage.UnitApproval{QuoteID: r.Msg.QuoteId, CancellationPolicyVersion: int(r.Msg.CancellationPolicyVersion)}
	if r.Msg.ApprovedMaxCredits != nil {
		n := int(*r.Msg.ApprovedMaxCredits)
		a.ApprovedMaxCredits = &n
	}
	id, e := h.speech.Start(ctx, owner, r.Msg.ProjectId, int(r.Msg.ExpectedRevision), r.Msg.IdempotencyKey, a)
	if e != nil {
		return nil, toConnectError(e)
	}
	return connect.NewResponse(&v1.StartClipSpeechResponse{JobId: id}), nil
}
func (h *Handler) GetClipSpeechReadiness(ctx context.Context, r *connect.Request[v1.GetClipSpeechReadinessRequest]) (*connect.Response[v1.ClipSpeechReadiness], error) {
	owner, e := actingUser(ctx)
	if e != nil {
		return nil, e
	}
	if h.speech == nil {
		return nil, toConnectError(clip.ErrCompositionUnavailable)
	}
	p, e := h.speech.Store.GetProject(ctx, owner, r.Msg.ProjectId)
	if e != nil {
		return nil, toConnectError(e)
	}
	edit, e := clip.DecodeEditPlan(p.EditPlan)
	if e != nil {
		return nil, toConnectError(e)
	}
	out := &v1.ClipSpeechReadiness{PlanRevision: int32(p.EditPlanRevision), RenderReady: clip.NarrationReadiness(edit) == nil}
	if edit.Narration != nil {
		last := 0
		for _, s := range edit.Narration.Segments {
			item := &v1.ClipSpeechSegmentReadiness{SegmentId: s.ID, State: "missing"}
			if s.Speech != nil {
				item.AssetId = s.Speech.AssetID
				item.DurationMs = int32(s.Speech.DurationMS())
				item.HasCharacterTiming = len(s.Speech.Timing) > 0
				item.State = "stale"
				if clip.CompatibleSpeech(edit.Narration, s) {
					item.State = "ready"
					if s.StartMS < last || s.EndMS > edit.DurationMS || s.StartMS+s.Speech.DurationMS() > s.EndMS {
						item.State = "timing_conflict"
					}
				}
			}
			out.Segments = append(out.Segments, item)
			last = s.EndMS
		}
	}
	return connect.NewResponse(out), nil
}
