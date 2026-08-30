package rpc

import (
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/publishing"
)

func TestRevokedAgentBusinessStateDoesNotInvalidateHumanSession(t *testing.T) {
	if code := connect.CodeOf(toConnectError(publishing.ErrAgentRevoked)); code != connect.CodeFailedPrecondition {
		t.Fatalf("revoked agent business-state code=%v", code)
	}
}
