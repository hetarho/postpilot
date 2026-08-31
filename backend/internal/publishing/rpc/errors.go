package rpc

import (
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/platform/rpcserver"
	"github.com/postpilot/backend/internal/publishing"
)

func toConnectError(err error) error {
	switch {
	case errors.Is(err, publishing.ErrNotFound):
		return rpcserver.NewAppError(connect.CodeNotFound, "publishing resource not found", "PUBLISH_NOT_FOUND", nil)
	case errors.Is(err, publishing.ErrForbidden):
		return rpcserver.NewAppError(connect.CodePermissionDenied, "publishing resource belongs to another user", "PUBLISH_FORBIDDEN", nil)
	case errors.Is(err, publishing.ErrInvalid):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid publishing request", "PUBLISH_REQUEST_INVALID", nil)
	case errors.Is(err, publishing.ErrPairingInvalid):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid pairing request", "PUBLISH_PAIRING_INVALID", nil)
	case errors.Is(err, publishing.ErrPairingLimit):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "pairing limit reached", "PUBLISH_PAIRING_LIMIT", nil)
	case errors.Is(err, publishing.ErrAlreadyPublishing):
		return rpcserver.NewAppError(connect.CodeAlreadyExists, "publish already exists", "PUBLISH_ALREADY_EXISTS", nil)
	case errors.Is(err, publishing.ErrAgentRevoked):
		// The human session is still valid. Agent bearer-token authentication is
		// rejected by agentInterceptor before handlers; here revocation is a
		// user-facing business-state race and must not trigger the SPA logout path.
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "publishing agent revoked", "PUBLISH_AGENT_REVOKED", nil)
	case errors.Is(err, publishing.ErrAgentNotReady):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "publishing agent not ready", "PUBLISH_AGENT_NOT_READY", nil)
	case errors.Is(err, publishing.ErrCategoryNotFound):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "publishing category not found", "PUBLISH_CATEGORY_NOT_FOUND", nil)
	case errors.Is(err, publishing.ErrPostNotFinalized):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "post not finalized", "PUBLISH_POST_NOT_FINALIZED", nil)
	case errors.Is(err, publishing.ErrCommitFence):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "publish commit fence reached", "PUBLISH_COMMIT_FENCE", nil)
	case errors.Is(err, publishing.ErrStaleRevision):
		return rpcserver.NewAppError(connect.CodeAborted, "stale post revision", "PUBLISH_STALE_REVISION", nil)
	case errors.Is(err, publishing.ErrLeaseInvalid):
		return rpcserver.NewAppError(connect.CodeAborted, "invalid publish lease", "PUBLISH_LEASE_INVALID", nil)
	case errors.Is(err, publishing.ErrTransition):
		return rpcserver.NewAppError(connect.CodeAborted, "invalid publish transition", "PUBLISH_TRANSITION_INVALID", nil)
	case errors.Is(err, publishing.ErrPublishedURLInvalid):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "invalid published URL", "PUBLISH_URL_INVALID", nil)
	default:
		slog.Error("publishing RPC failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, "publishing request failed", "UNKNOWN_FAILURE", nil)
	}
}
