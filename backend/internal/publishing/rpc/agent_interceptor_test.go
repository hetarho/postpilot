package rpc

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

func TestAgentInterceptorRejectsEveryRetiredAgentCapability(t *testing.T) {
	interceptor := NewAgentInterceptor()

	for _, procedure := range []string{
		postpilotv1connect.PublishingAgentServiceEnrollPublishingAgentProcedure,
		postpilotv1connect.PublishingAgentServiceSyncAgentProfileProcedure,
		postpilotv1connect.PublishingAgentServiceClaimPublishJobProcedure,
		postpilotv1connect.PublishingAgentServiceRenewPublishLeaseProcedure,
		postpilotv1connect.PublishingAgentServiceReportPublishProgressProcedure,
		postpilotv1connect.PublishingAgentServiceCompletePublishProcedure,
		postpilotv1connect.PublishingAgentServiceFailPublishProcedure,
	} {
		if _, err := interceptor.authorize(context.Background(), procedure, http.Header{"Authorization": []string{"Bearer old-token"}}); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("procedure %s error=%v", procedure, err)
		}
	}
	// The agent interceptor does not attach an agent identity to a human procedure;
	// the existing session interceptor remains its only authentication gate.
	humanCtx, err := interceptor.authorize(context.Background(), postpilotv1connect.PublishingServiceListPublishingAgentsProcedure, http.Header{"Authorization": []string{"Bearer old-token"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := agentFromContext(humanCtx); ok {
		t.Fatal("agent token authenticated a human publishing procedure")
	}
}
