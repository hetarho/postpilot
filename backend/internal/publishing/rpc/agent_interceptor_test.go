package rpc

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/publishing"
)

type fakeAgentAuthenticator struct {
	agent publishing.Agent
	err   error
	token string
}

func (f *fakeAgentAuthenticator) AuthenticateAgent(_ context.Context, token string) (publishing.Agent, error) {
	f.token = token
	return f.agent, f.err
}

func TestAgentInterceptorSeparatesHumanAndAgentCredentials(t *testing.T) {
	auth := &fakeAgentAuthenticator{agent: publishing.Agent{ID: "agent", UserID: "alice"}}
	interceptor := NewAgentInterceptor(auth)

	humanHeader := http.Header{"Cookie": []string{"postpilot_session=human"}}
	if _, err := interceptor.authorize(context.Background(), postpilotv1connect.PublishingAgentServiceClaimPublishJobProcedure, humanHeader); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("human cookie authenticated agent procedure: %v", err)
	}

	agentHeader := http.Header{"Authorization": []string{"Bearer raw-agent-token"}}
	ctx, err := interceptor.authorize(context.Background(), postpilotv1connect.PublishingAgentServiceClaimPublishJobProcedure, agentHeader)
	if err != nil {
		t.Fatal(err)
	}
	if auth.token != "raw-agent-token" {
		t.Fatalf("token=%q", auth.token)
	}
	if agent, ok := agentFromContext(ctx); !ok || agent.ID != "agent" || agent.UserID != "alice" {
		t.Fatalf("acting agent=%#v ok=%v", agent, ok)
	}

	// The agent interceptor does not attach an agent identity to a human procedure;
	// the existing session interceptor remains its only authentication gate.
	humanCtx, err := interceptor.authorize(context.Background(), postpilotv1connect.PublishingServiceListPublishingAgentsProcedure, agentHeader)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := agentFromContext(humanCtx); ok {
		t.Fatal("agent token authenticated a human publishing procedure")
	}
}

func TestAgentInterceptorAllowsEnrollmentOnlyAndRejectsRevokedToken(t *testing.T) {
	auth := &fakeAgentAuthenticator{err: publishing.ErrAgentRevoked}
	interceptor := NewAgentInterceptor(auth)
	if _, err := interceptor.authorize(context.Background(), postpilotv1connect.PublishingAgentServiceEnrollPublishingAgentProcedure, nil); err != nil {
		t.Fatalf("one-time enrollment was gated by agent auth: %v", err)
	}
	header := http.Header{"Authorization": []string{"Bearer revoked"}}
	if _, err := interceptor.authorize(context.Background(), postpilotv1connect.PublishingAgentServiceSyncAgentProfileProcedure, header); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("revoked token error=%v", err)
	}
	if !errors.Is(auth.err, publishing.ErrAgentRevoked) {
		t.Fatal("test authenticator lost revocation state")
	}
}

func TestAgentInterceptorKeepsOperationalAuthenticationFailuresRetryable(t *testing.T) {
	auth := &fakeAgentAuthenticator{err: errors.New("database temporarily unavailable")}
	interceptor := NewAgentInterceptor(auth)
	header := http.Header{"Authorization": []string{"Bearer valid-looking-token"}}

	if _, err := interceptor.authorize(context.Background(), postpilotv1connect.PublishingAgentServiceClaimPublishJobProcedure, header); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("operational authentication error=%v", err)
	}
}
