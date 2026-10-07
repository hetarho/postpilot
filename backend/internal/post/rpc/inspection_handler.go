package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

type inspectionOriginReader interface {
	Get(context.Context, string, string) (post.Post, error)
}

// InspectionHandler owns the post methods of the separate read service. The test
// and authoring methods remain reserved for their owning private payload fences.
type InspectionHandler struct {
	postpilotv1connect.UnimplementedWritingInspectionServiceHandler
	reader   post.RequestInspectionReader
	origins  inspectionOriginReader
	captures post.PostRequestCaptureReader
}

func NewInspectionHandler(reader post.RequestInspectionReader, origins inspectionOriginReader, captures post.PostRequestCaptureReader) *InspectionHandler {
	if reader == nil || origins == nil || captures == nil {
		panic("post rpc: inspection readers are required")
	}
	return &InspectionHandler{reader: reader, origins: origins, captures: captures}
}

func (h *InspectionHandler) GetPostOriginReview(ctx context.Context, req *connect.Request[postpilotv1.GetPostOriginReviewRequest]) (*connect.Response[postpilotv1.GetPostOriginReviewResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	value, err := h.origins.Get(ctx, userID, req.Msg.PostSlug)
	if err != nil {
		return nil, toConnectError("read post origins", err)
	}
	var review *postpilotv1.OriginReview
	if value.Content != nil && value.ContentOrigins != nil {
		current := post.ContentOriginIdentity(*value.Content, value.ContentRevision)
		resolved := post.ValidateResultOriginReview(*value.Content, current, value.ContentOrigins)
		if resolved.Review.Result == value.ContentOrigins.Result {
			review, err = OriginReviewToProto(&resolved.Review)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("invalid private origin projection"))
			}
		}
	}
	return connect.NewResponse(&postpilotv1.GetPostOriginReviewResponse{Review: review}), nil
}

func (h *InspectionHandler) GetPostRequestInspection(ctx context.Context, req *connect.Request[postpilotv1.GetPostRequestInspectionRequest]) (*connect.Response[postpilotv1.GetPostRequestInspectionResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	status, err := inspectionStatusFromProto(req.Msg.Status)
	if err != nil || status == llm.InspectionUnavailable {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid inspection view"))
	}
	value, err := h.reader.ReadPostRequestInspection(ctx, userID, req.Msg.PostSlug, req.Msg.Stage, status)
	if err != nil {
		return nil, inspectionConnectError(err)
	}
	wire, err := RequestInspectionToProto(value)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid private inspection projection"))
	}
	response := &postpilotv1.GetPostRequestInspectionResponse{Inspection: wire}
	if status == llm.InspectionCaptured {
		values, err := h.captures.ReadPostRequestCaptures(ctx, userID, req.Msg.PostSlug, req.Msg.Stage)
		if err != nil {
			return nil, inspectionConnectError(err)
		}
		for _, value := range values {
			wire, err := RequestInspectionToProto(value)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("invalid private capture projection"))
			}
			response.Inspections = append(response.Inspections, wire)
		}
		// Keep the legacy primary field consistent with the same plural snapshot.
		if len(response.Inspections) > 0 {
			response.Inspection = response.Inspections[len(response.Inspections)-1]
		} else {
			unavailable := llm.UnavailableRequestInspection(req.Msg.Stage, "")
			unavailable.UnavailableReason = "capture_missing_stale_or_purged"
			response.Inspection, _ = RequestInspectionToProto(unavailable)
		}
	}
	return connect.NewResponse(response), nil
}

func inspectionConnectError(err error) error {
	if errors.Is(err, llm.ErrInvalidInspection) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid inspection request"))
	}
	return toConnectError("read post inspection", err)
}
