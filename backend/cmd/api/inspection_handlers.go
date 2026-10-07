package main

import (
	"context"

	"connectrpc.com/connect"
	authoringrpc "github.com/postpilot/backend/internal/authoring/rpc"
	experimentrpc "github.com/postpilot/backend/internal/experiment/rpc"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	postrpc "github.com/postpilot/backend/internal/post/rpc"
)

// The shared read transport delegates ownership and private payload decisions to
// each domain's RPC adapter. The composition root only assembles the service.
type writingInspectionHandler struct {
	*postrpc.InspectionHandler
	tests     *experimentrpc.RequestInspectionHandler
	authoring *authoringrpc.Handler
}

func (h writingInspectionHandler) GetWritingTestRequestInspection(ctx context.Context, request *connect.Request[v1.GetWritingTestRequestInspectionRequest]) (*connect.Response[v1.GetWritingTestRequestInspectionResponse], error) {
	return h.tests.GetWritingTestRequestInspection(ctx, request)
}

func (h writingInspectionHandler) GetAuthoringRequestInspection(ctx context.Context, request *connect.Request[v1.GetAuthoringRequestInspectionRequest]) (*connect.Response[v1.GetAuthoringRequestInspectionResponse], error) {
	return h.authoring.GetAuthoringRequestInspection(ctx, request)
}

var _ postpilotv1connect.WritingInspectionServiceHandler = writingInspectionHandler{}
