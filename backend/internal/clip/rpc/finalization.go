package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func (h *Handler) setFinalizationState(out *v1.ClipProject, p clip.Project) {
	busy := out.LatestJob != nil && (out.LatestJob.Status == "queued" || out.LatestJob.Status == "running")
	canEdit := p.Finalized == nil && !busy
	out.CanEdit = &canEdit
	reason := "unavailable"
	if p.Finalized != nil {
		reason = "finalized"
	} else if h.generation != nil {
		reason = h.generation.FinalizationRefusal(p, busy)
	}
	canFinalize := reason == ""
	out.CanFinalize, out.FinalizationRefusal = &canFinalize, reason
}

func (h *Handler) FinalizeClipProject(ctx context.Context, req *connect.Request[v1.FinalizeClipProjectRequest]) (*connect.Response[v1.FinalizeClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	_, err = h.service.FinalizeProject(ctx, clip.FinalizationRequest{UserID: user, ProjectID: req.Msg.ProjectId, ExpectedRevision: int(req.Msg.ExpectedRevision), ExpectedResultID: req.Msg.ExpectedResultId})
	if err != nil {
		failure := toConnectError(err)
		// Conflict details are owner-scoped and carry the current result identity.
		if errors.Is(err, clip.ErrFinalizationConflict) || errors.Is(err, clip.ErrFinalizationInvalid) || errors.Is(err, clip.ErrBusy) {
			current, readErr := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: req.Msg.ProjectId}))
			var app *connect.Error
			if readErr == nil && errors.As(failure, &app) {
				if detail, e := connect.NewErrorDetail(current.Msg.Project); e == nil {
					app.AddDetail(detail)
				}
			}
		}
		return nil, failure
	}
	current, err := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: req.Msg.ProjectId}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.FinalizeClipProjectResponse{Project: current.Msg.Project}), nil
}
