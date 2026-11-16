package rpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

// ActingUser reads the user the auth interceptor placed on the context. Reaching a
// handler without one is a wiring fault, answered as the same AUTH_REQUIRED every
// other authenticated procedure returns.
func ActingUser(ctx context.Context) (string, error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return "", rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", postpilotv1.FailureReason_AUTH_REQUIRED, nil)
	}
	return user, nil
}
