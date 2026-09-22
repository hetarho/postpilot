package rpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestHumanPublishingMutationsRefuseAfterRetirement(t *testing.T) {
	handler := NewUserHandler(nil)
	ctx := auth.WithUser(context.Background(), "alice")
	checks := []struct {
		name string
		call func() error
	}{
		{"pair", func() error {
			_, err := handler.CreateAgentPairing(ctx, connect.NewRequest(&postpilotv1.CreateAgentPairingRequest{}))
			return err
		}},
		{"update agent", func() error {
			_, err := handler.UpdatePublishingAgent(ctx, connect.NewRequest(&postpilotv1.UpdatePublishingAgentRequest{}))
			return err
		}},
		{"revoke agent", func() error {
			_, err := handler.RevokePublishingAgent(ctx, connect.NewRequest(&postpilotv1.RevokePublishingAgentRequest{}))
			return err
		}},
		{"start", func() error {
			_, err := handler.StartPublish(ctx, connect.NewRequest(&postpilotv1.StartPublishRequest{}))
			return err
		}},
		{"retry", func() error {
			_, err := handler.RetryPublish(ctx, connect.NewRequest(&postpilotv1.RetryPublishRequest{}))
			return err
		}},
		{"cancel", func() error {
			_, err := handler.CancelPublish(ctx, connect.NewRequest(&postpilotv1.CancelPublishRequest{}))
			return err
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.call()
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("code=%v err=%v", connect.CodeOf(err), err)
			}
			if reason := publishingErrorReason(t, err); reason != "PUBLISH_AGENT_UNAVAILABLE" {
				t.Fatalf("reason=%q", reason)
			}
		})
	}
}

func TestRetiredHumanMutationStillRequiresSession(t *testing.T) {
	handler := NewUserHandler(nil)
	_, err := handler.StartPublish(context.Background(), connect.NewRequest(&postpilotv1.StartPublishRequest{}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("code=%v err=%v", connect.CodeOf(err), err)
	}
}
