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
// AppFailure is a domain error that already describes itself in machine-readable terms.
// The interface is structural on purpose: this package stays free of business meaning,
// and any context can satisfy it without importing anything from here.
type AppFailure interface {
	error
	Reason() string
	Params() map[string]string
}

// AppErrorFrom maps a self-describing domain failure onto a Connect status. Callers pick
// the code — the same failure is `resource_exhausted` when it refuses new work and
// `permission_denied` when it refuses authority.
func AppErrorFrom(code connect.Code, failure AppFailure) *connect.Error {
	return NewAppError(code, failure.Error(), failure.Reason(), failure.Params())
}

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
