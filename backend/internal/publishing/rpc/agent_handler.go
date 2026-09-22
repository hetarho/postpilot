package rpc

import (
	"context"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/publishing"
)

type AgentHandler struct{}

func NewAgentHandler() *AgentHandler { return &AgentHandler{} }

func (h *AgentHandler) EnrollPublishingAgent(ctx context.Context, req *connect.Request[postpilotv1.EnrollPublishingAgentRequest]) (*connect.Response[postpilotv1.EnrollPublishingAgentResponse], error) {
	return nil, retiredAgentError()
}

func (h *AgentHandler) SyncAgentProfile(ctx context.Context, req *connect.Request[postpilotv1.SyncAgentProfileRequest]) (*connect.Response[postpilotv1.SyncAgentProfileResponse], error) {
	return nil, retiredAgentError()
}

func (h *AgentHandler) ClaimPublishJob(ctx context.Context, _ *connect.Request[postpilotv1.ClaimPublishJobRequest]) (*connect.Response[postpilotv1.ClaimPublishJobResponse], error) {
	return nil, retiredAgentError()
}

func (h *AgentHandler) RenewPublishLease(ctx context.Context, req *connect.Request[postpilotv1.RenewPublishLeaseRequest]) (*connect.Response[postpilotv1.RenewPublishLeaseResponse], error) {
	return nil, retiredAgentError()
}

func (h *AgentHandler) ReportPublishProgress(ctx context.Context, req *connect.Request[postpilotv1.ReportPublishProgressRequest]) (*connect.Response[postpilotv1.ReportPublishProgressResponse], error) {
	return nil, retiredAgentError()
}

func (h *AgentHandler) CompletePublish(ctx context.Context, req *connect.Request[postpilotv1.CompletePublishRequest]) (*connect.Response[postpilotv1.CompletePublishResponse], error) {
	return nil, retiredAgentError()
}

func (h *AgentHandler) FailPublish(ctx context.Context, req *connect.Request[postpilotv1.FailPublishRequest]) (*connect.Response[postpilotv1.FailPublishResponse], error) {
	return nil, retiredAgentError()
}

func retiredAgentError() error {
	return rpcserver.NewAppError(connect.CodeUnauthenticated, "publishing agent capability retired", postpilotv1.FailureReason_PUBLISH_AGENT_REVOKED, nil)
}

func actingAgent(ctx context.Context) (publishing.Agent, error) {
	agent, ok := agentFromContext(ctx)
	if !ok {
		return publishing.Agent{}, rpcserver.NewAppError(connect.CodeUnauthenticated, "publishing agent authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return agent, nil
}

var _ postpilotv1connect.PublishingAgentServiceHandler = (*AgentHandler)(nil)
