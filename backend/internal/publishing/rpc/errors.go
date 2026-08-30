package rpc

import (
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/publishing"
)

func toConnectError(err error) error {
	switch {
	case errors.Is(err, publishing.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("발행 작업 또는 연결을 찾지 못했어요."))
	case errors.Is(err, publishing.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, errors.New("이 계정의 발행 작업이 아니에요."))
	case errors.Is(err, publishing.ErrInvalid), errors.Is(err, publishing.ErrPairingInvalid):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("발행 요청 값을 확인해 주세요."))
	case errors.Is(err, publishing.ErrPairingLimit), errors.Is(err, publishing.ErrAlreadyPublishing):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("이미 대기 중인 연결 또는 발행 작업이 있어요."))
	case errors.Is(err, publishing.ErrAgentRevoked):
		// The human session is still valid. Agent bearer-token authentication is
		// rejected by agentInterceptor before handlers; here revocation is a
		// user-facing business-state race and must not trigger the SPA logout path.
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("Mac 연결이 해제되었어요."))
	case errors.Is(err, publishing.ErrAgentNotReady), errors.Is(err, publishing.ErrCategoryNotFound), errors.Is(err, publishing.ErrPostNotFinalized), errors.Is(err, publishing.ErrCommitFence):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("발행 전 준비 상태를 확인해 주세요."))
	case errors.Is(err, publishing.ErrStaleRevision), errors.Is(err, publishing.ErrLeaseInvalid), errors.Is(err, publishing.ErrTransition):
		return connect.NewError(connect.CodeAborted, errors.New("상태가 바뀌었어요. 새로고침 후 다시 확인해 주세요."))
	case errors.Is(err, publishing.ErrPublishedURLInvalid):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("발행된 주소가 연결한 네이버 블로그와 일치하지 않아요."))
	default:
		slog.Error("publishing RPC failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New("발행 작업을 처리하지 못했어요."))
	}
}
