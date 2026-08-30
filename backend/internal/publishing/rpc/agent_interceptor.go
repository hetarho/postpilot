package rpc

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/publishing"
)

type AgentAuthenticator interface {
	AuthenticateAgent(ctx context.Context, rawToken string) (publishing.Agent, error)
}

type AgentInterceptor struct{ auth AgentAuthenticator }

func NewAgentInterceptor(auth AgentAuthenticator) *AgentInterceptor {
	return &AgentInterceptor{auth: auth}
}

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
	if !isAgentProcedure(procedure) || procedure == postpilotv1connect.PublishingAgentServiceEnrollPublishingAgentProcedure {
		return ctx, nil
	}
	scheme, token, ok := strings.Cut(strings.TrimSpace(header.Get("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("publishing agent authentication required"))
	}
	agent, err := i.auth.AuthenticateAgent(ctx, token)
	if err != nil {
		if errors.Is(err, publishing.ErrAgentRevoked) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("publishing agent authentication failed"))
		}
		// Database/touch failures are operational, not credential revocation. Returning
		// Unauthenticated would make the Mac supervisor stop permanently instead of
		// applying its transient-error backoff.
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("publishing agent authentication is temporarily unavailable"))
	}
	return withAgent(ctx, agent), nil
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
