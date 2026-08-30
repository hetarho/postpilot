package rpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/publishing"
)

type AgentHandler struct{ service *publishing.Service }

func NewAgentHandler(service *publishing.Service) *AgentHandler {
	return &AgentHandler{service: service}
}

func (h *AgentHandler) EnrollPublishingAgent(ctx context.Context, req *connect.Request[postpilotv1.EnrollPublishingAgentRequest]) (*connect.Response[postpilotv1.EnrollPublishingAgentResponse], error) {
	enrollment, err := h.service.Enroll(ctx, req.Msg.GetDeviceCode(), req.Msg.GetBrowserLabel())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.EnrollPublishingAgentResponse{AgentId: enrollment.AgentID, AgentToken: enrollment.Token, LeaseTtlSeconds: int64(enrollment.LeaseTTL / time.Second)}), nil
}

func (h *AgentHandler) SyncAgentProfile(ctx context.Context, req *connect.Request[postpilotv1.SyncAgentProfileRequest]) (*connect.Response[postpilotv1.SyncAgentProfileResponse], error) {
	agent, err := actingAgent(ctx)
	if err != nil {
		return nil, err
	}
	categories := make([]publishing.Category, 0, len(req.Msg.GetCategories()))
	for _, category := range req.Msg.GetCategories() {
		categories = append(categories, publishing.Category{ID: category.GetId(), Name: category.GetName()})
	}
	updated, err := h.service.SyncAgent(ctx, agent, publishing.ProfileUpdate{PlatformAccountID: req.Msg.GetPlatformAccountId(), PlatformAccountLabel: req.Msg.GetPlatformAccountLabel(), BrowserLabel: req.Msg.GetBrowserLabel(), Categories: categories, DefaultCategoryID: req.Msg.GetDefaultCategoryId(), DefaultVisibility: fromProtoVisibility(req.Msg.GetDefaultVisibility()), CompatibilityReady: req.Msg.GetCompatibilityReady(), HermesVersion: req.Msg.GetHermesVersion()})
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.SyncAgentProfileResponse{Agent: toProtoAgent(updated)}), nil
}

func (h *AgentHandler) ClaimPublishJob(ctx context.Context, _ *connect.Request[postpilotv1.ClaimPublishJobRequest]) (*connect.Response[postpilotv1.ClaimPublishJobResponse], error) {
	agent, err := actingAgent(ctx)
	if err != nil {
		return nil, err
	}
	claim, err := h.service.Claim(ctx, agent)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.ClaimPublishJobResponse{Job: toProtoJob(claim.Job), LeaseToken: claim.LeaseToken, LeaseExpiresAt: claim.LeaseExpires.UTC().Format(time.RFC3339), LeaseTtlSeconds: int64(claim.LeaseTTL / time.Second), Manifest: toProtoManifest(claim.Manifest, claim.URLs)}), nil
}

func (h *AgentHandler) RenewPublishLease(ctx context.Context, req *connect.Request[postpilotv1.RenewPublishLeaseRequest]) (*connect.Response[postpilotv1.RenewPublishLeaseResponse], error) {
	agent, err := actingAgent(ctx)
	if err != nil {
		return nil, err
	}
	expires, err := h.service.Renew(ctx, agent, req.Msg.GetJobId(), req.Msg.GetLeaseToken())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.RenewPublishLeaseResponse{LeaseExpiresAt: expires.UTC().Format(time.RFC3339)}), nil
}

func (h *AgentHandler) ReportPublishProgress(ctx context.Context, req *connect.Request[postpilotv1.ReportPublishProgressRequest]) (*connect.Response[postpilotv1.ReportPublishProgressResponse], error) {
	agent, err := actingAgent(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.Progress(ctx, agent, req.Msg.GetJobId(), req.Msg.GetLeaseToken(), fromProtoStage(req.Msg.GetStage()), req.Msg.GetProgressSequence())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.ReportPublishProgressResponse{Job: toProtoJob(job)}), nil
}

func (h *AgentHandler) CompletePublish(ctx context.Context, req *connect.Request[postpilotv1.CompletePublishRequest]) (*connect.Response[postpilotv1.CompletePublishResponse], error) {
	agent, err := actingAgent(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.Complete(ctx, agent, req.Msg.GetJobId(), req.Msg.GetLeaseToken(), req.Msg.GetProgressSequence(), req.Msg.GetPlatformPostUrl())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.CompletePublishResponse{Job: toProtoJob(job)}), nil
}

func (h *AgentHandler) FailPublish(ctx context.Context, req *connect.Request[postpilotv1.FailPublishRequest]) (*connect.Response[postpilotv1.FailPublishResponse], error) {
	agent, err := actingAgent(ctx)
	if err != nil {
		return nil, err
	}
	job, err := h.service.Fail(ctx, agent, req.Msg.GetJobId(), req.Msg.GetLeaseToken(), req.Msg.GetProgressSequence(), fromProtoFailure(req.Msg.GetKind()), req.Msg.GetDetail())
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&postpilotv1.FailPublishResponse{Job: toProtoJob(job)}), nil
}

func actingAgent(ctx context.Context) (publishing.Agent, error) {
	agent, ok := agentFromContext(ctx)
	if !ok {
		return publishing.Agent{}, connect.NewError(connect.CodeUnauthenticated, errors.New("publishing agent authentication required"))
	}
	return agent, nil
}

var _ postpilotv1connect.PublishingAgentServiceHandler = (*AgentHandler)(nil)
