package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// caption is the one mapping from the wire's caption to the domain's; the wire
// still calls the vertical anchor `position` (CDS-12).
func caption(c *v1.ClipCaption) clip.Caption {
	return clip.Caption{Pace: c.GetPace(), Text: c.GetText(), Anchor: c.GetPosition(), Align: c.GetAlign(), Keyword: c.GetKeyword(),
		Style: c.GetStyle(), Accent: c.GetAccent(), StartMS: int(c.GetStartMs()), EndMS: int(c.GetEndMs())}
}
func captionProto(c clip.Caption) *v1.ClipCaption {
	return &v1.ClipCaption{Pace: c.Pace, Text: c.Text, Position: c.Anchor, Align: c.Align, Keyword: c.Keyword,
		Style: c.Style, Accent: c.Accent, StartMs: int32(c.StartMS), EndMs: int32(c.EndMS)}
}
func correctionPlan(p *v1.ClipEditPlan) clip.CorrectionPlan {
	out := clip.CorrectionPlan{DurationMS: int(p.GetDurationMs()), Hook: p.GetHook()}
	for _, c := range p.GetCuts() {
		copies := make([]clip.Caption, 0, len(c.GetCopies()))
		for _, copy := range c.GetCopies() {
			copies = append(copies, caption(copy))
		}
		// A client that has not moved to `copies` still sends the one `copy`,
		// and is read exactly as it was before CDS-43.
		if len(copies) == 0 && c.GetCopy() != nil {
			copies = append(copies, caption(c.GetCopy()))
		}
		out.Cuts = append(out.Cuts, clip.CorrectionCut{ID: c.GetId(), SourceID: c.GetSourceId(), Fingerprint: c.GetFingerprint(), StartMS: int(c.GetStartMs()), EndMS: int(c.GetEndMs()), TransitionMS: int(c.GetTransitionMs()), VolumePermille: int(c.GetVolumePermille()), Chips: c.GetChips(), Copies: copies})
	}
	return out
}
func editingProto(s *clip.CorrectionState) *v1.ClipEditingState {
	if s == nil {
		return nil
	}
	out := &v1.ClipEditingState{Plan: &v1.ClipEditPlan{DurationMs: int32(s.Plan.DurationMS), Hook: s.Plan.Hook}, CopyStyles: s.CopyStyles, FadeMs: int32(s.FadeMS), MaxCuts: int32(s.MaxCuts), MaxCopyRunes: int32(s.MaxCopyRunes), MinDurationMs: int32(s.MinDurationMS), MaxDurationMs: int32(s.MaxDurationMS)}
	for _, c := range s.Plan.Cuts {
		copies := make([]*v1.ClipCaption, 0, len(c.Copies))
		for _, copy := range c.Copies {
			copies = append(copies, captionProto(copy))
		}
		// `copy` stays populated with the first one for a release, so a client
		// that has not moved to `copies` still shows the sentence.
		first := &v1.ClipCaption{}
		if len(copies) > 0 {
			first = copies[0]
		}
		out.Plan.Cuts = append(out.Plan.Cuts, &v1.ClipEditCut{Id: c.ID, SourceId: c.SourceID, Fingerprint: c.Fingerprint, StartMs: int32(c.StartMS), EndMs: int32(c.EndMS), TransitionMs: int32(c.TransitionMS), VolumePermille: int32(c.VolumePermille), Chips: c.Chips, Copy: first, Copies: copies})
	}
	for _, s := range s.Sources {
		out.Sources = append(out.Sources, &v1.ClipRetainedSource{Id: s.ID, Fingerprint: s.Fingerprint, Filename: s.Filename, DurationMs: int32(s.Info.DurationMS), Width: int32(s.Info.Width), Height: int32(s.Info.Height)})
	}
	return out
}
func (h *Handler) SaveClipEditPlan(ctx context.Context, req *connect.Request[v1.SaveClipEditPlanRequest]) (*connect.Response[v1.SaveClipEditPlanResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(errors.New("clip correction unavailable"))
	}
	if _, err = h.generation.SaveCorrection(ctx, user, req.Msg.ProjectId, int(req.Msg.ExpectedRevision), correctionPlan(req.Msg.Plan)); err != nil {
		return nil, toConnectError(err)
	}
	response, err := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: req.Msg.ProjectId}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.SaveClipEditPlanResponse{Project: response.Msg.Project}), nil
}
func (h *Handler) StartClipRender(ctx context.Context, req *connect.Request[v1.StartClipRenderRequest]) (*connect.Response[v1.StartClipRenderResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if h.generation == nil {
		return nil, toConnectError(errors.New("clip rendering unavailable"))
	}
	id, err := h.generation.StartRender(ctx, user, req.Msg.ProjectId, req.Msg.BatchId, int(req.Msg.ExpectedRevision))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.StartClipRenderResponse{JobId: id}), nil
}
