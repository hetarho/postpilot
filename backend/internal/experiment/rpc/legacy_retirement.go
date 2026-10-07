package rpc

import (
	"context"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

func legacyComparisonRetired(ctx context.Context) error {
	if _, err := actingUser(ctx); err != nil {
		return err
	}
	return rpcserver.NewAppError(connect.CodeFailedPrecondition, "earlier comparisons are read only; start a common writing test", postpilotv1.FailureReason_WRITING_TEST_LEGACY_READ_ONLY, nil)
}
