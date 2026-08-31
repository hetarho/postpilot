// Package rpc is the auth context's transport edge: it maps proto ↔ domain and owns
// the one thing the proto contract deliberately cannot express — the session cookie.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/platform/rpcserver"
)

// invalidCredentialsMessage is the single text every login failure returns. It is a
// fixed string, not a formatted one, so an unknown id and a wrong password are
// byte-identical on the wire (plan 01 AC3).
const invalidCredentialsMessage = "invalid credentials"

// Handler implements postpilotv1connect.AuthServiceHandler.
type Handler struct {
	svc *auth.Service
	// sessionTTL drives the cookie's Max-Age. It is the same value the service stamps
	// into sessions.expires_at, so the cookie and the row can never disagree.
	sessionTTL time.Duration
}

// NewHandler returns the AuthService implementation.
func NewHandler(svc *auth.Service, sessionTTL time.Duration) *Handler {
	return &Handler{svc: svc, sessionTTL: sessionTTL}
}

// Login authenticates and hands back a session cookie.
func (h *Handler) Login(ctx context.Context, req *connect.Request[postpilotv1.LoginRequest]) (*connect.Response[postpilotv1.LoginResponse], error) {
	user, rawToken, err := h.svc.Login(ctx, req.Msg.GetLoginId(), req.Msg.GetPassword())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, invalidCredentialsMessage, "INVALID_CREDENTIALS", nil)
		}
		// An infrastructure failure must not become "invalid credentials" — that would
		// hide an outage behind a login form. The detail stays in the log.
		slog.Error("login failed", "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "login failed", "UNKNOWN_FAILURE", nil)
	}

	res := connect.NewResponse(&postpilotv1.LoginResponse{
		User: &postpilotv1.User{Id: user.ID},
	})
	// The token leaves the process here and nowhere else: a Set-Cookie header, not a
	// response field. HttpOnly keeps it away from JavaScript (so an XSS cannot read
	// it), Secure keeps it off plaintext HTTP, and no Domain attribute makes it
	// host-only so sibling projects on the same registered domain never see it.
	res.Header().Add("Set-Cookie", h.sessionCookie(rawToken, int(h.sessionTTL.Seconds())).String())
	return res, nil
}

// Logout revokes the session and clears the cookie.
func (h *Handler) Logout(ctx context.Context, req *connect.Request[postpilotv1.LogoutRequest]) (*connect.Response[postpilotv1.LogoutResponse], error) {
	if err := h.svc.Logout(ctx, cookieValue(req.Header())); err != nil {
		slog.Error("logout failed", "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "logout failed", "UNKNOWN_FAILURE", nil)
	}

	res := connect.NewResponse(&postpilotv1.LogoutResponse{})
	// MaxAge<0 emits `Max-Age=0`, which expires the cookie. Every other attribute must
	// still match the one that was set, or the browser keeps the original alongside it.
	res.Header().Add("Set-Cookie", h.sessionCookie("", -1).String())
	return res, nil
}

// GetMe reports the account behind the current session. It carries no logic: the
// interceptor has already proven the session, so reaching this function IS the answer.
func (h *Handler) GetMe(ctx context.Context, _ *connect.Request[postpilotv1.GetMeRequest]) (*connect.Response[postpilotv1.GetMeResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	return connect.NewResponse(&postpilotv1.GetMeResponse{
		User: &postpilotv1.User{Id: userID},
	}), nil
}

func (h *Handler) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

// cookieValue reads the session cookie off a request. Both the interceptor and Logout
// need it: the context the interceptor publishes carries the user id, not the token,
// so a handler that must act on THIS session has to read the cookie itself.
func cookieValue(header http.Header) string {
	// http.Request is the documented way to parse a Cookie header; Connect exposes
	// headers only as an http.Header, so borrow the parser.
	c, err := (&http.Request{Header: header}).Cookie(auth.SessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

var _ postpilotv1connect.AuthServiceHandler = (*Handler)(nil)
