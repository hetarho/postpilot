package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/authoring"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
)

func (h *Handler) GetAuthoringRequestInspection(ctx context.Context, req *connect.Request[v1.GetAuthoringRequestInspectionRequest]) (*connect.Response[v1.GetAuthoringRequestInspectionResponse], error) {
	user, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	id := req.Msg.GetSessionId()
	if id == "" {
		id = req.Msg.GetDraftId()
	}
	if id == "" || req.Msg.GetSessionId() != "" && req.Msg.GetDraftId() != "" && req.Msg.GetSessionId() != req.Msg.GetDraftId() {
		return nil, toError(authoring.ErrInvalid)
	}
	k, m := kind(req.Msg.GetKind()), mode(req.Msg.GetMode())
	if !k.Valid() {
		return nil, toError(authoring.ErrInvalidKind)
	}
	if !m.Valid() || req.Msg.GetStage() != "" && req.Msg.GetStage() != "setting-authoring" {
		return nil, toError(authoring.ErrInvalid)
	}
	var status llm.InspectionStatus
	switch req.Msg.GetStatus() {
	case v1.InspectionStatus_INSPECTION_STATUS_CURRENT:
		status = llm.InspectionCurrent
	case v1.InspectionStatus_INSPECTION_STATUS_PREPARED:
		status = llm.InspectionPrepared
	case v1.InspectionStatus_INSPECTION_STATUS_CAPTURED:
		status = llm.InspectionCaptured
	default:
		return nil, toError(authoring.ErrInvalid)
	}
	out, err := h.service.InspectRequest(ctx, user, authoring.RequestInspectionInput{SessionID: id, Kind: k, Revision: req.Msg.GetRevision(), Mode: m, Prompt: req.Msg.GetPrompt(), Model: ref(req.Msg.GetModel()), CandidateCount: int(req.Msg.GetCandidateCount()), Status: status})
	if errors.Is(err, llm.ErrInvalidInspection) {
		err = authoring.ErrInvalid
	}
	if err != nil {
		return nil, toError(err)
	}
	wire, err := postrpc.RequestInspectionToProto(out)
	if err != nil {
		return nil, toError(authoring.ErrInvalid)
	}
	return connect.NewResponse(&v1.GetAuthoringRequestInspectionResponse{Inspection: wire}), nil
}
