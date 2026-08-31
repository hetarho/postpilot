package rpc

import (
	"context"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/publishing"
)

type UserHandler struct{ service *publishing.Service }

func NewUserHandler(service *publishing.Service) *UserHandler { return &UserHandler{service: service} }

func (h *UserHandler) CreateAgentPairing(ctx context.Context, req *connect.Request[postpilotv1.CreateAgentPairingRequest]) (*connect.Response[postpilotv1.CreateAgentPairingResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	pairing, err := h.service.CreatePairing(ctx, userID, req.Msg.GetLabel())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.CreateAgentPairingResponse{DeviceCode: pairing.DeviceCode, ExpiresAt: pairing.ExpiresAt.UTC().Format(time.RFC3339)}), nil
}

func (h *UserHandler) ListPublishingAgents(ctx context.Context, _ *connect.Request[postpilotv1.ListPublishingAgentsRequest]) (*connect.Response[postpilotv1.ListPublishingAgentsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	agents, err := h.service.ListAgents(ctx, userID)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*postpilotv1.PublishingAgent, 0, len(agents))
	for _, agent := range agents {
		out = append(out, toProtoAgent(agent))
	}
	return connect.NewResponse(&postpilotv1.ListPublishingAgentsResponse{Agents: out}), nil
}

func (h *UserHandler) UpdatePublishingAgent(ctx context.Context, req *connect.Request[postpilotv1.UpdatePublishingAgentRequest]) (*connect.Response[postpilotv1.UpdatePublishingAgentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	agent, err := h.service.UpdateAgent(ctx, userID, req.Msg.GetAgentId(), req.Msg.GetLabel(), req.Msg.GetDefaultCategoryId(), fromProtoVisibility(req.Msg.GetDefaultVisibility()))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.UpdatePublishingAgentResponse{Agent: toProtoAgent(agent)}), nil
}

func (h *UserHandler) RevokePublishingAgent(ctx context.Context, req *connect.Request[postpilotv1.RevokePublishingAgentRequest]) (*connect.Response[postpilotv1.RevokePublishingAgentResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.RevokeAgent(ctx, userID, req.Msg.GetAgentId()); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.RevokePublishingAgentResponse{}), nil
}

func (h *UserHandler) StartPublish(ctx context.Context, req *connect.Request[postpilotv1.StartPublishRequest]) (*connect.Response[postpilotv1.StartPublishResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.Start(ctx, publishing.StartRequest{UserID: userID, PostSlug: req.Msg.GetPostSlug(), ExpectedContentRevision: req.Msg.GetExpectedContentRevision(), AgentID: req.Msg.GetAgentId(), CategoryID: req.Msg.GetCategoryId(), Visibility: fromProtoVisibility(req.Msg.GetVisibility())})
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.StartPublishResponse{Job: toProtoJob(job)}), nil
}

func (h *UserHandler) GetPublishJob(ctx context.Context, req *connect.Request[postpilotv1.GetPublishJobRequest]) (*connect.Response[postpilotv1.GetPublishJobResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.GetJob(ctx, userID, req.Msg.GetPostSlug(), req.Msg.GetJobId())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.GetPublishJobResponse{Job: toProtoJob(job)}), nil
}

func (h *UserHandler) ListRetryablePublishJobs(ctx context.Context, _ *connect.Request[postpilotv1.ListRetryablePublishJobsRequest]) (*connect.Response[postpilotv1.ListRetryablePublishJobsResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := h.service.ListRetryable(ctx, userID)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*postpilotv1.PublishJob, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, toProtoJob(job))
	}
	return connect.NewResponse(&postpilotv1.ListRetryablePublishJobsResponse{Jobs: out}), nil
}

func (h *UserHandler) RetryPublish(ctx context.Context, req *connect.Request[postpilotv1.RetryPublishRequest]) (*connect.Response[postpilotv1.RetryPublishResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.Retry(ctx, userID, req.Msg.GetJobId())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.RetryPublishResponse{Job: toProtoJob(job)}), nil
}

func (h *UserHandler) CancelPublish(ctx context.Context, req *connect.Request[postpilotv1.CancelPublishRequest]) (*connect.Response[postpilotv1.CancelPublishResponse], error) {
	userID, err := actingUser(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.Cancel(ctx, userID, req.Msg.GetJobId())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.CancelPublishResponse{Job: toProtoJob(job)}), nil
}

func actingUser(ctx context.Context) (string, error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return userID, nil
}

var _ postpilotv1connect.PublishingServiceHandler = (*UserHandler)(nil)
