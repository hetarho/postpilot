package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

// unauthenticatedMessage is what an unauthenticated call sees. Missing, forged, and
// expired sessions all produce it — the client's next move is the same in every case.
const unauthenticatedMessage = "unauthenticated"

// publicProcedures are the only procedures reachable without a session:
//   - Login, because it is how you get one.
//   - Logout, because logging out must always clear the cookie. Gated, it would answer
//     401 to an already-expired session and leave that HttpOnly cookie sitting in the
//     browser until it aged out on its own — revoking a dead session is a no-op anyway.
//   - Ping, the wire test the frontend runs before any account exists.
//
// Everything else is closed by default: a new service is protected the moment it is
// mounted, with no change here.
var publicProcedures = map[string]bool{
	postpilotv1connect.AuthServiceLoginProcedure:  true,
	postpilotv1connect.AuthServiceLogoutProcedure: true,
	postpilotv1connect.HealthServicePingProcedure: true,
}

// Interceptor is the authentication gate for every Connect procedure.
//
// It is the only place a request turns into an acting user: it resolves the session
// cookie once and puts the user id in the context, so downstream handlers never parse
// a cookie, never touch the sessions table, and never take a user id from a payload.
//
// It is a full connect.Interceptor rather than a connect.UnaryInterceptorFunc because
// that helper leaves WrapStreamingHandler as a pass-through — the first streaming
// procedure added later (the generation job queue, plan 05) would ship unauthenticated
// with nothing here to notice.
type Interceptor struct {
	svc *auth.Service
}

// NewInterceptor returns the authentication gate.
func NewInterceptor(svc *auth.Service) *Interceptor {
	return &Interceptor{svc: svc}
}

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authorize(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authorize(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

// WrapStreamingClient is a pass-through: this process is the server, and outbound
// client calls carry no postpilot session.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// authorize returns the context downstream handlers should see, or an Unauthenticated
// error. It fails closed: any path that does not explicitly succeed denies.
func (i *Interceptor) authorize(ctx context.Context, procedure string, header http.Header) (context.Context, error) {
	if publicProcedures[procedure] {
		return ctx, nil
	}

	userID, err := i.svc.Authenticate(ctx, cookieValue(header))
	if err != nil {
		if !errors.Is(err, auth.ErrNoSession) {
			// A store failure is not the caller's fault, but it still must not let the
			// request through — deny, and keep the reason in the log rather than in the
			// response.
			slog.Error("session lookup failed", "procedure", procedure, "err", err)
		}
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New(unauthenticatedMessage))
	}

	return auth.WithUser(ctx, userID), nil
}

var _ connect.Interceptor = (*Interceptor)(nil)
