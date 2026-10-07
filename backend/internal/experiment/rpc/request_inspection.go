package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
)

type RequestInspectionHandler struct {
	postpilotv1connect.UnimplementedWritingInspectionServiceHandler
	reader experiment.TestRequestInspections
}

func NewRequestInspectionHandler(reader experiment.TestRequestInspections) *RequestInspectionHandler {
	if reader == nil {
		panic("experiment rpc: private request inspection reader is required")
	}
	return &RequestInspectionHandler{reader: reader}
}

func (h *RequestInspectionHandler) GetWritingTestRequestInspection(ctx context.Context, req *connect.Request[v1.GetWritingTestRequestInspectionRequest]) (*connect.Response[v1.GetWritingTestRequestInspectionResponse], error) {
	user, err := actingUser(ctx)
	if err != nil {
		return nil, err
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
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid inspection view"))
	}
	values, err := h.reader.ReadTestRequestInspections(ctx, user, req.Msg.GetTestId(), req.Msg.GetCandidateId(), req.Msg.GetStage(), status)
	if err != nil {
		switch {
		case errors.Is(err, experiment.ErrTestNotFound):
			return nil, connect.NewError(connect.CodeNotFound, errors.New("writing test evidence not found"))
		case errors.Is(err, llm.ErrInvalidInspection):
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid inspection request"))
		default:
			return nil, connect.NewError(connect.CodeInternal, errors.New("writing test inspection unavailable"))
		}
	}
	response := &v1.GetWritingTestRequestInspectionResponse{}
	for _, value := range values {
		wire, err := postrpc.RequestInspectionToProto(value)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("invalid private inspection projection"))
		}
		response.Inspections = append(response.Inspections, wire)
		response.Inspection = wire
	}
	if response.Inspection == nil {
		value := llm.UnavailableRequestInspection("", "writing-test")
		value.UnavailableReason = "capture_missing_stale_or_purged"
		response.Inspection, _ = postrpc.RequestInspectionToProto(value)
	}
	return connect.NewResponse(response), nil
}
