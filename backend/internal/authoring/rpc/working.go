package rpc

import (
	"connectrpc.com/connect"
	"context"
	"github.com/postpilot/backend/internal/authoring"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"time"
)

func draftState(s authoring.DraftState) v1.AuthoringDraftState {
	switch s {
	case authoring.DraftValid:
		return v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_VALID
	case authoring.DraftIncomplete:
		return v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_INCOMPLETE
	case authoring.DraftInvalid:
		return v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_INVALID
	}
	return v1.AuthoringDraftState_AUTHORING_DRAFT_STATE_UNSPECIFIED
}
func (h *Handler) PatchAuthoringDraft(ctx context.Context, r *connect.Request[v1.PatchAuthoringDraftRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	user, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	a := r.Msg.GetWorkingSource()
	if a == nil {
		return nil, toError(authoring.ErrInvalid)
	}
	state, err := h.service.PatchDraft(ctx, authoring.DraftMutation{UserID: user, SessionID: r.Msg.GetSessionId(), ExpectedRevision: r.Msg.GetExpectedRevision(), OperationKey: r.Msg.GetOperationKey(), WorkingSource: authoring.Artifact{Name: a.GetName(), Description: a.GetDescription(), Body: a.GetBody(), TitleArea: a.GetTitleArea()}})
	return respond(state, err)
}
func (h *Handler) ResetAuthoringChat(ctx context.Context, r *connect.Request[v1.ResetAuthoringChatRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	user, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	state, err := h.service.ResetChat(ctx, authoring.ResetMutation{UserID: user, SessionID: r.Msg.GetSessionId(), ExpectedRevision: r.Msg.GetExpectedRevision(), OperationKey: r.Msg.GetOperationKey()})
	return respond(state, err)
}
func (h *Handler) ResetAuthoringBaseline(ctx context.Context, r *connect.Request[v1.ResetAuthoringBaselineRequest]) (*connect.Response[v1.AuthoringSessionResponse], error) {
	user, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	state, err := h.service.ResetBaseline(ctx, authoring.ResetMutation{UserID: user, SessionID: r.Msg.GetSessionId(), ExpectedRevision: r.Msg.GetExpectedRevision(), OperationKey: r.Msg.GetOperationKey()})
	return respond(state, err)
}
func (h *Handler) ListAuthoringSummaries(ctx context.Context, r *connect.Request[v1.ListAuthoringSummariesRequest]) (*connect.Response[v1.ListAuthoringSummariesResponse], error) {
	user, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	summaries, next, err := h.service.ListSummaries(ctx, authoring.SummaryQuery{UserID: user, Kind: kind(r.Msg.GetKind()), UnsavedOnly: r.Msg.GetUnsavedOnly(), PageSize: int(r.Msg.GetPageSize()), PageToken: r.Msg.GetPageToken()})
	if err != nil {
		return nil, toError(err)
	}
	out := &v1.ListAuthoringSummariesResponse{NextPageToken: next}
	for _, s := range summaries {
		row := &v1.AuthoringSummary{SessionId: s.SessionID, Kind: kindProto(s.Kind), TargetId: s.TargetID, DisplayName: s.DisplayName, Revision: s.Revision, SavedAvailable: s.SavedAvailable, HasUnpublishedChanges: s.HasUnpublishedChanges, ActiveJobId: s.ActiveJobID, PublicationPending: s.PublicationPending, TargetConflict: s.TargetConflict, DraftState: draftState(s.DraftState), UpdatedAt: s.UpdatedAt.Format(time.RFC3339Nano)}
		if s.LastPublication != nil {
			row.LastPublication = &v1.AuthoringSavedRef{Kind: kindProto(s.LastPublication.Kind), Id: s.LastPublication.ID, Name: s.LastPublication.Name}
		}
		out.Summaries = append(out.Summaries, row)
	}
	return connect.NewResponse(out), nil
}
