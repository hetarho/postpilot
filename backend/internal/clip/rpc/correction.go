package rpc

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func correctionPlan(p *v1.ClipEditPlan) clip.CorrectionPlan {
	out := clip.CorrectionPlan{DurationMS: int(p.GetDurationMs())}
	for _, c := range p.GetCuts() {
		copy := c.GetCopy()
		out.Cuts = append(out.Cuts, clip.CorrectionCut{ID: c.GetId(), SourceID: c.GetSourceId(), Fingerprint: c.GetFingerprint(), StartMS: int(c.GetStartMs()), EndMS: int(c.GetEndMs()), VolumePermille: int(c.GetVolumePermille()), Chips: c.GetChips(), Copy: clip.Caption{Text: copy.GetText(), Anchor: copy.GetPosition(), Align: copy.GetAlign(), Keyword: copy.GetKeyword(), Style: copy.GetStyle(), Accent: copy.GetAccent(), StartMS: int(copy.GetStartMs()), EndMS: int(copy.GetEndMs())}})
	}
	return out
}
func editingProto(s *clip.CorrectionState) *v1.ClipEditingState {
	if s == nil {
		return nil
	}
	out := &v1.ClipEditingState{Plan: &v1.ClipEditPlan{DurationMs: int32(s.Plan.DurationMS)}, CopyStyles: s.CopyStyles, FadeMs: int32(s.FadeMS), MaxCuts: int32(s.MaxCuts), MaxCopyRunes: int32(s.MaxCopyRunes), MinDurationMs: int32(s.MinDurationMS), MaxDurationMs: int32(s.MaxDurationMS)}
	for _, c := range s.Plan.Cuts {
		out.Plan.Cuts = append(out.Plan.Cuts, &v1.ClipEditCut{Id: c.ID, SourceId: c.SourceID, Fingerprint: c.Fingerprint, StartMs: int32(c.StartMS), EndMs: int32(c.EndMS), VolumePermille: int32(c.VolumePermille), Chips: c.Chips, Copy: &v1.ClipCaption{Text: c.Copy.Text, Position: c.Copy.Anchor, Align: c.Copy.Align, Keyword: c.Copy.Keyword, Style: c.Copy.Style, Accent: c.Copy.Accent, StartMs: int32(c.Copy.StartMS), EndMs: int32(c.Copy.EndMS)}})
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
