package rpc

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

type AgentInterceptor struct{}

func NewAgentInterceptor() *AgentInterceptor { return &AgentInterceptor{} }

func (i *AgentInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authorize(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *AgentInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authorize(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *AgentInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *AgentInterceptor) authorize(ctx context.Context, procedure string, header http.Header) (context.Context, error) {
	if !isAgentProcedure(procedure) {
		return ctx, nil
	}
	return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "publishing agent capability retired", postpilotv1.FailureReason_PUBLISH_AGENT_REVOKED, nil)
}

func isAgentProcedure(procedure string) bool {
	switch procedure {
	case postpilotv1connect.PublishingAgentServiceEnrollPublishingAgentProcedure,
		postpilotv1connect.PublishingAgentServiceSyncAgentProfileProcedure,
		postpilotv1connect.PublishingAgentServiceClaimPublishJobProcedure,
		postpilotv1connect.PublishingAgentServiceRenewPublishLeaseProcedure,
		postpilotv1connect.PublishingAgentServiceReportPublishProgressProcedure,
		postpilotv1connect.PublishingAgentServiceCompletePublishProcedure,
		postpilotv1connect.PublishingAgentServiceFailPublishProcedure:
		return true
	default:
		return false
	}
}

var _ connect.Interceptor = (*AgentInterceptor)(nil)
