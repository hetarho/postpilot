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
	planrpc "github.com/postpilot/backend/internal/plan/rpc"
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

func (h *Handler) Signup(ctx context.Context, req *connect.Request[postpilotv1.SignupRequest]) (*connect.Response[postpilotv1.SignupResponse], error) {
	if err := h.svc.Signup(ctx, req.Msg.GetEmail(), req.Msg.GetPassword()); err != nil {
		return nil, authMutationError("signup", err)
	}
	return connect.NewResponse(&postpilotv1.SignupResponse{}), nil
}

func (h *Handler) ResendVerification(ctx context.Context, req *connect.Request[postpilotv1.ResendVerificationRequest]) (*connect.Response[postpilotv1.ResendVerificationResponse], error) {
	if err := h.svc.ResendVerification(ctx, req.Msg.GetEmail()); err != nil {
		return nil, authMutationError("resend verification", err)
	}
	return connect.NewResponse(&postpilotv1.ResendVerificationResponse{}), nil
}

func (h *Handler) VerifyEmail(ctx context.Context, req *connect.Request[postpilotv1.VerifyEmailRequest]) (*connect.Response[postpilotv1.VerifyEmailResponse], error) {
	if err := h.svc.VerifyEmail(ctx, req.Msg.GetToken()); err != nil {
		return nil, authMutationError("verify email", err)
	}
	return connect.NewResponse(&postpilotv1.VerifyEmailResponse{}), nil
}

func (h *Handler) RequestPasswordReset(ctx context.Context, req *connect.Request[postpilotv1.RequestPasswordResetRequest]) (*connect.Response[postpilotv1.RequestPasswordResetResponse], error) {
	if err := h.svc.RequestPasswordReset(ctx, req.Msg.GetEmail()); err != nil {
		return nil, authMutationError("request password reset", err)
	}
	return connect.NewResponse(&postpilotv1.RequestPasswordResetResponse{}), nil
}

func (h *Handler) ResetPassword(ctx context.Context, req *connect.Request[postpilotv1.ResetPasswordRequest]) (*connect.Response[postpilotv1.ResetPasswordResponse], error) {
	if err := h.svc.ResetPassword(ctx, req.Msg.GetToken(), req.Msg.GetNewPassword()); err != nil {
		if errors.Is(err, auth.ErrLinkInvalid) {
			return nil, rpcserver.NewAppError(connect.CodeFailedPrecondition, "password reset link invalid", "RESET_LINK_INVALID", nil)
		}
		return nil, authMutationError("reset password", err)
	}
	return connect.NewResponse(&postpilotv1.ResetPasswordResponse{}), nil
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
		User: userToProto(user),
		Plan: planrpc.ToProto(user.Plan),
	})
	// The token leaves the process here and nowhere else: a Set-Cookie header, not a
	// response field. HttpOnly keeps it away from JavaScript (so an XSS cannot read
	// it), Secure keeps it off plaintext HTTP, and no Domain attribute makes it
	// host-only so sibling projects on the same registered domain never see it.
	res.Header().Add("Set-Cookie", h.sessionCookie(rawToken, int(h.sessionTTL.Seconds())).String())
	return res, nil
}

// SignInWithGoogle exchanges Google's short-lived code and emits the same host-only
// session cookie as password Login.
func (h *Handler) SignInWithGoogle(ctx context.Context, req *connect.Request[postpilotv1.SignInWithGoogleRequest]) (*connect.Response[postpilotv1.SignInWithGoogleResponse], error) {
	user, rawToken, err := h.svc.SignInWithGoogleCode(
		ctx, req.Msg.GetCode(), req.Msg.GetCodeVerifier(), req.Msg.GetRedirectUri(),
	)
	if err != nil {
		return nil, googleSignInError(err)
	}
	res := connect.NewResponse(&postpilotv1.SignInWithGoogleResponse{
		User: userToProto(user), Plan: planrpc.ToProto(user.Plan),
	})
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
// interceptor has already proven the session and resolved the tier, so reaching this
// function IS the answer.
//
// The plan rides this probe so the app can gate master-only surfaces during boot without
// a second round-trip; the server still refuses those procedures on its own.
func (h *Handler) GetMe(ctx context.Context, _ *connect.Request[postpilotv1.GetMeRequest]) (*connect.Response[postpilotv1.GetMeResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	acting, _ := auth.PlanFromContext(ctx)
	user, err := h.svc.Account(ctx, userID)
	if err != nil {
		slog.Error("get account failed", "err", err)
		return nil, rpcserver.NewAppError(connect.CodeInternal, "get account failed", "UNKNOWN_FAILURE", nil)
	}
	return connect.NewResponse(&postpilotv1.GetMeResponse{
		User: userToProto(user),
		Plan: planrpc.ToProto(acting),
	}), nil
}

func (h *Handler) RegisterEmail(ctx context.Context, req *connect.Request[postpilotv1.RegisterEmailRequest]) (*connect.Response[postpilotv1.RegisterEmailResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	if err := h.svc.RegisterEmail(ctx, userID, req.Msg.GetEmail()); err != nil {
		return nil, authMutationError("register email", err)
	}
	return connect.NewResponse(&postpilotv1.RegisterEmailResponse{}), nil
}

func (h *Handler) ChangePassword(ctx context.Context, req *connect.Request[postpilotv1.ChangePasswordRequest]) (*connect.Response[postpilotv1.ChangePasswordResponse], error) {
	userID, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, rpcserver.NewAppError(connect.CodeUnauthenticated, "authentication required", "AUTH_REQUIRED", nil)
	}
	if err := h.svc.ChangePassword(ctx, userID, req.Msg.GetCurrentPassword(), req.Msg.GetNewPassword()); err != nil {
		return nil, authMutationError("change password", err)
	}
	return connect.NewResponse(&postpilotv1.ChangePasswordResponse{}), nil
}

func userToProto(user auth.User) *postpilotv1.User {
	return &postpilotv1.User{
		Id: user.ID, Email: user.Email, EmailVerified: user.EmailVerifiedAt != nil,
		HasPassword: user.PasswordHash != "",
	}
}

func authMutationError(operation string, err error) error {
	switch {
	case errors.Is(err, auth.ErrInvalidEmail):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "invalid email", "INVALID_EMAIL", nil)
	case errors.Is(err, auth.ErrPasswordTooShort):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "password too short", "PASSWORD_TOO_SHORT", map[string]string{"min": "8"})
	case errors.Is(err, auth.ErrPasswordTooLong):
		return rpcserver.NewAppError(connect.CodeInvalidArgument, "password too long", "PASSWORD_TOO_LONG", map[string]string{"max": "128"})
	case errors.Is(err, auth.ErrLinkInvalid):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "verification link invalid", "VERIFICATION_LINK_INVALID", nil)
	case errors.Is(err, auth.ErrEmailAlreadyVerified):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "email already verified", "EMAIL_ALREADY_VERIFIED", nil)
	case errors.Is(err, auth.ErrPasswordNotSet):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "password not set", "PASSWORD_NOT_SET", nil)
	case errors.Is(err, auth.ErrCurrentPasswordWrong):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "current password wrong", "CURRENT_PASSWORD_WRONG", nil)
	default:
		slog.Error(operation+" failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, operation+" failed", "UNKNOWN_FAILURE", nil)
	}
}

func googleSignInError(err error) error {
	switch {
	case errors.Is(err, auth.ErrGoogleSignInDisabled):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "google sign-in disabled", "GOOGLE_SIGNIN_DISABLED", nil)
	case errors.Is(err, auth.ErrGoogleEmailUnverified):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "google email unverified", "GOOGLE_EMAIL_UNVERIFIED", nil)
	case errors.Is(err, auth.ErrGoogleAccountMismatch):
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "google account mismatch", "GOOGLE_ACCOUNT_MISMATCH", nil)
	case errors.Is(err, auth.ErrGoogleSignInFailed):
		slog.Info("google sign-in refused", "err", err)
		return rpcserver.NewAppError(connect.CodeFailedPrecondition, "google sign-in failed", "GOOGLE_SIGNIN_FAILED", nil)
	default:
		slog.Error("google sign-in failed", "err", err)
		return rpcserver.NewAppError(connect.CodeInternal, "google sign-in failed", "UNKNOWN_FAILURE", nil)
	}
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
