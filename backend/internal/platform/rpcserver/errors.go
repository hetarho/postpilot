package rpcserver

import (
	"errors"
	"maps"

	"connectrpc.com/connect"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// NewAppError creates a Connect error with exactly one machine-readable product
// failure detail. The message is an operator-safe English fallback; clients render
// user-facing copy from reason and the display-safe params owned by the calling
// context.
//
// This helper deliberately does not own a reason registry or translate domain
// errors. Each RPC adapter remains responsible for choosing its stable reason and
// allowlisting params at the context boundary.
func NewAppError(code connect.Code, message, reason string, params map[string]string) *connect.Error {
	connectErr := connect.NewError(code, errors.New(message))
	detail, err := connect.NewErrorDetail(&postpilotv1.AppErrorDetail{
		Reason: reason,
		Params: maps.Clone(params),
	})
	if err == nil {
		connectErr.AddDetail(detail)
	}
	return connectErr
}
