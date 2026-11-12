package rpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func (h *Handler) setFinalizationState(out *v1.ClipProject, p clip.Project) {
	busy := out.LatestJob != nil && (out.LatestJob.Status == "queued" || out.LatestJob.Status == "running")
	canEdit := p.Finalized == nil && !busy
	out.CanEdit = &canEdit
	reason, confirmation := "unavailable", "unavailable"
	if p.Finalized != nil {
		reason, confirmation = "finalized", "finalized"
	} else if h.generation != nil {
		reason = h.generation.FinalizationRefusal(p, busy)
		confirmation = h.generation.ConfirmationRefusal(p, busy)
	}
	canFinalize, canConfirm := reason == "", confirmation == ""
	out.CanFinalize, out.FinalizationRefusal = &canFinalize, reason
	out.CanConfirm, out.ConfirmationRefusal = &canConfirm, confirmation
}

// confirmationFields carries CLIP-229's confirmation, CLIP-232's posted URL and CLIP-231's
// ticks; whether the project reads as confirmed is confirmed_plan_revision == edit_plan_revision.
func confirmationFields(out *v1.ClipProject, p clip.Project) {
	if c := p.Confirmed; c != nil {
		out.ConfirmedPlanRevision = int32(c.PlanRevision)
		out.ConfirmedAt = c.At.UTC().Format(time.RFC3339Nano)
	}
	if u := p.Published; u != nil {
		out.PublishedUrl = u.URL
		out.PublishedAt = u.At.UTC().Format(time.RFC3339Nano)
	}
	out.ChecklistTicks = append([]string{}, p.ChecklistTicks...)
}

// listFinalizationState is setFinalizationState for a directory row, which is
// read without its plan: what the row's own facts decide, and "unavailable"
// where only validating the plan could say the clip is ready (the detail does).
func listFinalizationState(out *v1.ClipProject, p clip.Project) {
	busy := out.LatestJob != nil && (out.LatestJob.Status == "queued" || out.LatestJob.Status == "running")
	canEdit, canFinalize, canConfirm := p.Finalized == nil && !busy, false, false
	reason, confirmation := "unavailable", "unavailable"
	switch {
	case p.Finalized != nil:
		reason, confirmation = "finalized", "finalized"
	case busy:
		reason, confirmation = "busy", "busy"
	case !clip.Confirmed(p):
		reason = "unconfirmed"
	case p.Result == nil || p.Result.ID == "":
		reason = "missing_render"
	case p.EditPlanRevision != p.RenderedPlanRevision:
		reason = "stale_render"
	}
	out.CanEdit, out.CanFinalize, out.FinalizationRefusal = &canEdit, &canFinalize, reason
	out.CanConfirm, out.ConfirmationRefusal = &canConfirm, confirmation
}

func (h *Handler) FinalizeClipProject(ctx context.Context, req *connect.Request[v1.FinalizeClipProjectRequest]) (*connect.Response[v1.FinalizeClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	_, err = h.service.FinalizeProject(ctx, clip.FinalizationRequest{UserID: user, ProjectID: req.Msg.ProjectId, ExpectedRevision: int(req.Msg.ExpectedRevision), ExpectedResultID: req.Msg.ExpectedResultId, PublishedURL: req.Msg.PublishedUrl})
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

// ConfirmClipProject is 확정하기 (CLIP-75): it records the plan revision, renders nothing and
// answers the project as the detail reads it.
func (h *Handler) ConfirmClipProject(ctx context.Context, req *connect.Request[v1.ConfirmClipProjectRequest]) (*connect.Response[v1.ConfirmClipProjectResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.service.ConfirmProject(ctx, clip.ConfirmationRequest{UserID: user, ProjectID: req.Msg.ProjectId, ExpectedPlanRevision: int(req.Msg.ExpectedPlanRevision)}); err != nil {
		return nil, toConnectError(err)
	}
	current, err := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: req.Msg.ProjectId}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ConfirmClipProjectResponse{Project: current.Msg.Project}), nil
}

// SaveClipPublishedUrl saves or replaces a finalized clip's posted URL (CLIP-232).
func (h *Handler) SaveClipPublishedUrl(ctx context.Context, req *connect.Request[v1.SaveClipPublishedUrlRequest]) (*connect.Response[v1.SaveClipPublishedUrlResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.service.SavePublishedURL(ctx, user, req.Msg.ProjectId, req.Msg.Url); err != nil {
		return nil, toConnectError(err)
	}
	current, err := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: req.Msg.ProjectId}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.SaveClipPublishedUrlResponse{Project: current.Msg.Project}), nil
}
