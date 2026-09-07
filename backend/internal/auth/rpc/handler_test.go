package rpc_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
	"github.com/postpilot/backend/internal/plan"
)

const sessionTTL = 720 * time.Hour

type discardMailer struct{}

func (discardMailer) Send(context.Context, auth.Mail) error { return nil }

type recordingMailer struct{ sent []auth.Mail }

func (m *recordingMailer) Send(_ context.Context, mail auth.Mail) error {
	m.sent = append(m.sent, mail)
	return nil
}

type googleIdentity struct {
	claims auth.GoogleClaims
	err    error
}

func (g googleIdentity) Exchange(context.Context, string, string, string) (auth.GoogleClaims, error) {
	return g.claims, g.err
}

// newServer wires the real handler and interceptor over a real SQLite store and
// returns a client speaking to them across a real HTTP server.
//
// Nothing here is faked: these tests are about what a browser actually receives —
// the Set-Cookie string, the 401 — so a stubbed store or an in-process handler call
// would assert the wrong thing.
func newServer(t *testing.T) (postpilotv1connect.AuthServiceClient, *httptest.Server) {
	t.Helper()

	store := newStore(t)
	svc := auth.NewService(store, sessionTTL)
	svc.SetMailer(discardMailer{})
	svc.SetWebOrigin("https://postpilot.example.com")
	if err := svc.CreateUser(context.Background(), "alice", "s3cret", plan.Free); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	verifiedAt := time.Now()
	if err := store.SetEmail(context.Background(), "alice", "alice@example.com", &verifiedAt); err != nil {
		t.Fatalf("seed email: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(
		authrpc.NewHandler(svc, sessionTTL),
		connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), "")),
	))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL), server
}

// TestLoginSetCookieAttributes is plan 01 AC7 + AC10: the exact attribute set, and a
// Max-Age that matches the session lifetime the server stamped into the row.
func TestLoginSetCookieAttributes(t *testing.T) {
	client, _ := newServer(t)

	res, err := client.Login(context.Background(), connect.NewRequest(&postpilotv1.LoginRequest{
		LoginId:  "alice",
		Password: "s3cret",
	}))
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	setCookie := res.Header().Get("Set-Cookie")
	token := sessionToken(t, setCookie)

	if got, want := setCookie, "pp_session="+token+"; Path=/; Max-Age=2592000; HttpOnly; Secure; SameSite=Lax"; got != want {
		t.Errorf("Set-Cookie mismatch\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(strings.ToLower(setCookie), "domain") {
		t.Error("the cookie carries a Domain attribute — it must stay host-only")
	}

	// Plan 01 AC4 (server half): the token is in the header and nowhere else.
	if res.Msg.GetUser().GetId() != "alice" {
		t.Errorf("user id = %q, want alice", res.Msg.GetUser().GetId())
	}
	if got := res.Msg.GetUser().GetEmail(); got != "alice@example.com" {
		t.Errorf("user email = %q, want alice@example.com", got)
	}
	if !res.Msg.GetUser().GetEmailVerified() {
		t.Error("user email_verified = false, want true")
	}
	if !res.Msg.GetUser().GetHasPassword() {
		t.Error("user has_password = false, want true")
	}
	if strings.Contains(res.Msg.String(), token) {
		t.Errorf("the session token leaked into the response body: %s", res.Msg.String())
	}
}

func TestGoogleSignInIsPublicAndSetsTheLoginCookie(t *testing.T) {
	store := newStore(t)
	svc := auth.NewService(store, sessionTTL)
	svc.SetGoogle(googleIdentity{claims: auth.GoogleClaims{
		Subject: "google-1", Email: "person@example.com", EmailVerified: true,
	}})
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(
		authrpc.NewHandler(svc, sessionTTL),
		connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), "")),
	))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)

	res, err := client.SignInWithGoogle(context.Background(), connect.NewRequest(&postpilotv1.SignInWithGoogleRequest{
		Code: "code", CodeVerifier: "verifier", RedirectUri: "https://postpilot.example.com/login/google/callback",
	}))
	if err != nil {
		t.Fatalf("SignInWithGoogle: %v", err)
	}
	token := sessionToken(t, res.Header().Get("Set-Cookie"))
	wantCookie := "pp_session=" + token + "; Path=/; Max-Age=2592000; HttpOnly; Secure; SameSite=Lax"
	if got := res.Header().Get("Set-Cookie"); got != wantCookie {
		t.Fatalf("Set-Cookie = %q, want %q", got, wantCookie)
	}
	if res.Msg.GetUser().GetId() != "person@example.com" || res.Msg.GetPlan() != postpilotv1.Plan_PLAN_FREE {
		t.Fatalf("response = %v", res.Msg)
	}
}

func TestGoogleSignInDisabledIsAStablePublicRefusal(t *testing.T) {
	client, _ := newServer(t)
	_, err := client.SignInWithGoogle(context.Background(), connect.NewRequest(&postpilotv1.SignInWithGoogleRequest{
		Code: "code", CodeVerifier: "verifier", RedirectUri: "https://postpilot.example.com/login/google/callback",
	}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("SignInWithGoogle = %v, want failed_precondition", err)
	}
	if reason := authAppErrorDetail(t, err).GetReason(); reason != "GOOGLE_SIGNIN_DISABLED" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestLoginFailureIsGenericAndSetsNoCookie(t *testing.T) {
	client, _ := newServer(t)

	unknown := loginError(t, client, "nobody", "s3cret")
	wrong := loginError(t, client, "alice", "wrong")

	if connect.CodeOf(unknown) != connect.CodeUnauthenticated {
		t.Errorf("unknown id code = %v, want unauthenticated", connect.CodeOf(unknown))
	}
	if connect.CodeOf(wrong) != connect.CodeUnauthenticated {
		t.Errorf("wrong password code = %v, want unauthenticated", connect.CodeOf(wrong))
	}
	if unknown.Error() != wrong.Error() {
		t.Errorf("failures differ:\n unknown: %s\n   wrong: %s", unknown, wrong)
	}
	if !strings.Contains(unknown.Error(), "invalid credentials") {
		t.Errorf("unexpected message: %s", unknown)
	}
	for _, err := range []error{unknown, wrong} {
		detail := authAppErrorDetail(t, err)
		if detail.GetReason() != "INVALID_CREDENTIALS" || len(detail.GetParams()) != 0 {
			t.Errorf("credential detail = %#v", detail)
		}
	}
}

func TestLoginThrottleIsPerIPAndRunsBeforeAccountAccounting(t *testing.T) {
	store := newStore(t)
	svc := auth.NewService(store, sessionTTL)
	svc.SetMailer(discardMailer{})
	if err := svc.CreateUser(context.Background(), "alice", "s3cret", plan.Free); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(
		authrpc.NewHandler(svc, sessionTTL),
		connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), "X-Forwarded-For")),
	))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)

	request := func(ip, password string) error {
		req := connect.NewRequest(&postpilotv1.LoginRequest{LoginId: "alice", Password: password})
		req.Header().Set("X-Forwarded-For", "198.51.100.99, "+ip)
		_, err := client.Login(context.Background(), req)
		return err
	}
	for attempt := 1; attempt <= 10; attempt++ {
		if err := request("203.0.113.10", "s3cret"); err != nil {
			t.Fatalf("allowed login %d: %v", attempt, err)
		}
	}
	refused := request("203.0.113.10", "s3cret")
	if connect.CodeOf(refused) != connect.CodeResourceExhausted {
		t.Fatalf("11th login = %v, want resource_exhausted", refused)
	}
	detail := authAppErrorDetail(t, refused)
	if detail.GetReason() != "TOO_MANY_ATTEMPTS" || len(detail.GetParams()) != 1 {
		t.Fatalf("throttle detail = %#v", detail)
	}
	retryAt, err := time.Parse(time.RFC3339, detail.GetParams()["retry_at"])
	if err != nil || retryAt.Location() != time.UTC || !retryAt.After(time.Now()) {
		t.Fatalf("retry_at = %q, %v", detail.GetParams()["retry_at"], err)
	}
	afterRefusal, err := store.GetUser(context.Background(), "alice")
	if err != nil || afterRefusal.FailedLogins != 0 || afterRefusal.LockedUntil != nil {
		t.Fatalf("throttle refusal changed account state = %+v, %v", afterRefusal, err)
	}

	otherPeerWrong := request("203.0.113.11", "wrong")
	if connect.CodeOf(otherPeerWrong) != connect.CodeUnauthenticated || authAppErrorDetail(t, otherPeerWrong).GetReason() != "INVALID_CREDENTIALS" {
		t.Fatalf("other peer wrong password = %v", otherPeerWrong)
	}
	if err := request("203.0.113.11", "s3cret"); err != nil {
		t.Fatalf("other peer correct login: %v", err)
	}
	account, err := store.GetUser(context.Background(), "alice")
	if err != nil || account.FailedLogins != 0 || account.LockedUntil != nil {
		t.Fatalf("throttled account state = %+v, %v", account, err)
	}
}

func TestLockedAccountIsWireIdenticalToWrongPassword(t *testing.T) {
	client, _ := newServer(t)
	firstWrong := loginError(t, client, "alice", "wrong")
	for attempt := 2; attempt <= auth.LockThreshold; attempt++ {
		_ = loginError(t, client, "alice", "wrong")
	}
	lockedCorrect := loginError(t, client, "alice", "s3cret")
	if firstWrong.Error() != lockedCorrect.Error() {
		t.Fatalf("wrong and locked responses differ:\nwrong: %s\nlocked: %s", firstWrong, lockedCorrect)
	}
}

// TestInterceptorGuardsEveryProcedure is plan 01 AC1: no cookie means 401 on anything
// but Login, and a valid cookie gets through.
func TestInterceptorGuardsEveryProcedure(t *testing.T) {
	client, _ := newServer(t)

	if _, err := client.GetMe(context.Background(), connect.NewRequest(&postpilotv1.GetMeRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("GetMe without a cookie: %v, want unauthenticated", err)
	} else if detail := authAppErrorDetail(t, err); detail.GetReason() != "AUTH_REQUIRED" || len(detail.GetParams()) != 0 {
		t.Errorf("authentication detail = %#v", detail)
	}

	cookie := login(t, client)

	res, err := client.GetMe(context.Background(), withCookie(&postpilotv1.GetMeRequest{}, cookie))
	if err != nil {
		t.Fatalf("GetMe with a valid cookie: %v", err)
	}
	if res.Msg.GetUser().GetId() != "alice" {
		t.Errorf("user id = %q, want alice", res.Msg.GetUser().GetId())
	}
	if got := res.Msg.GetUser().GetEmail(); got != "alice@example.com" {
		t.Errorf("user email = %q, want alice@example.com", got)
	}
	if !res.Msg.GetUser().GetEmailVerified() {
		t.Error("user email_verified = false, want true")
	}
	if !res.Msg.GetUser().GetHasPassword() {
		t.Error("user has_password = false, want true")
	}
}

func TestInterceptorPublicSignupAndVerificationSetButProtectsRegisterEmail(t *testing.T) {
	client, _ := newServer(t)
	ctx := context.Background()

	if _, err := client.Signup(ctx, connect.NewRequest(&postpilotv1.SignupRequest{
		Email: "new@example.com", Password: "password1",
	})); err != nil {
		t.Fatalf("Signup without cookie: %v", err)
	}
	if _, err := client.ResendVerification(ctx, connect.NewRequest(&postpilotv1.ResendVerificationRequest{
		Email: "new@example.com",
	})); err != nil {
		t.Fatalf("ResendVerification without cookie: %v", err)
	}
	if _, err := client.VerifyEmail(ctx, connect.NewRequest(&postpilotv1.VerifyEmailRequest{
		Token: "not-a-token",
	})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("VerifyEmail without cookie = %v, want its domain failure rather than 401", err)
	}
	if _, err := client.RequestPasswordReset(ctx, connect.NewRequest(&postpilotv1.RequestPasswordResetRequest{
		Email: "alice@example.com",
	})); err != nil {
		t.Fatalf("RequestPasswordReset without cookie: %v", err)
	}
	if _, err := client.ResetPassword(ctx, connect.NewRequest(&postpilotv1.ResetPasswordRequest{
		Token: "not-a-token", NewPassword: "new-password",
	})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ResetPassword without cookie = %v, want its domain failure rather than 401", err)
	} else if detail := authAppErrorDetail(t, err); detail.GetReason() != "RESET_LINK_INVALID" {
		t.Fatalf("ResetPassword reason = %q, want RESET_LINK_INVALID", detail.GetReason())
	}
	if _, err := client.RegisterEmail(ctx, connect.NewRequest(&postpilotv1.RegisterEmailRequest{
		Email: "alice@example.com",
	})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("RegisterEmail without cookie = %v, want 401", err)
	}
	if _, err := client.ChangePassword(ctx, connect.NewRequest(&postpilotv1.ChangePasswordRequest{
		CurrentPassword: "s3cret", NewPassword: "new-password",
	})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("ChangePassword without cookie = %v, want 401", err)
	}
}

func TestResetPasswordRevokesThePreResetCookie(t *testing.T) {
	store := newStore(t)
	svc := auth.NewService(store, sessionTTL)
	mailer := &recordingMailer{}
	svc.SetMailer(mailer)
	svc.SetWebOrigin("https://postpilot.example.com")
	if err := svc.CreateUser(context.Background(), "alice", "s3cret", plan.Free); err != nil {
		t.Fatal(err)
	}
	verifiedAt := time.Now()
	if err := store.SetEmail(context.Background(), "alice", "alice@example.com", &verifiedAt); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(
		authrpc.NewHandler(svc, sessionTTL),
		connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), "")),
	))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)

	loginRes, err := client.Login(context.Background(), connect.NewRequest(&postpilotv1.LoginRequest{
		LoginId: "alice", Password: "s3cret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	cookie := "pp_session=" + sessionToken(t, loginRes.Header().Get("Set-Cookie"))
	if _, err := client.RequestPasswordReset(context.Background(), connect.NewRequest(&postpilotv1.RequestPasswordResetRequest{
		Email: "alice@example.com",
	})); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("reset mails = %d, want 1", len(mailer.sent))
	}
	raw := resetTokenFromMail(t, mailer.sent[0])
	if _, err := client.ResetPassword(context.Background(), connect.NewRequest(&postpilotv1.ResetPasswordRequest{
		Token: raw, NewPassword: "new-password",
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetMe(context.Background(), withCookie(&postpilotv1.GetMeRequest{}, cookie)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("GetMe with pre-reset cookie = %v, want unauthenticated", err)
	}
	if err := loginError(t, client, "alice", "s3cret"); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("old password login = %v, want unauthenticated", err)
	}
	if _, err := client.Login(context.Background(), connect.NewRequest(&postpilotv1.LoginRequest{
		LoginId: "alice", Password: "new-password",
	})); err != nil {
		t.Fatalf("new password login: %v", err)
	}
}

func TestChangePasswordFailureReasonsStayInsideTheLiveSession(t *testing.T) {
	t.Run("wrong current password", func(t *testing.T) {
		client, _ := newServer(t)
		cookie := login(t, client)
		_, err := client.ChangePassword(context.Background(), withCookie(&postpilotv1.ChangePasswordRequest{
			CurrentPassword: "wrong", NewPassword: "new-password",
		}, cookie))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("ChangePassword = %v, want failed_precondition", err)
		}
		if detail := authAppErrorDetail(t, err); detail.GetReason() != "CURRENT_PASSWORD_WRONG" {
			t.Fatalf("reason = %q, want CURRENT_PASSWORD_WRONG", detail.GetReason())
		}
	})

	t.Run("password not set", func(t *testing.T) {
		store := newStore(t)
		svc := auth.NewService(store, sessionTTL)
		if err := svc.CreateUser(context.Background(), "alice", "s3cret", plan.Free); err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		mux.Handle(postpilotv1connect.NewAuthServiceHandler(
			authrpc.NewHandler(svc, sessionTTL),
			connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), "")),
		))
		server := httptest.NewServer(mux)
		defer server.Close()
		client := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)
		cookie := login(t, client)
		if err := store.UpdatePasswordHash(context.Background(), "alice", ""); err != nil {
			t.Fatal(err)
		}

		_, err := client.ChangePassword(context.Background(), withCookie(&postpilotv1.ChangePasswordRequest{
			CurrentPassword: "anything", NewPassword: "new-password",
		}, cookie))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("ChangePassword = %v, want failed_precondition", err)
		}
		if detail := authAppErrorDetail(t, err); detail.GetReason() != "PASSWORD_NOT_SET" {
			t.Fatalf("reason = %q, want PASSWORD_NOT_SET", detail.GetReason())
		}
	})
}

func TestUnverifiedCorrectPasswordIsWireIdenticalToWrongPassword(t *testing.T) {
	store := newStore(t)
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser(context.Background(), auth.User{
		ID: "waiting@example.com", Email: "waiting@example.com", PasswordHash: hash,
		Plan: plan.Free, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	svc := auth.NewService(store, sessionTTL)
	svc.SetMailer(discardMailer{})
	svc.SetWebOrigin("https://postpilot.example.com")
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(
		authrpc.NewHandler(svc, sessionTTL),
		connect.WithInterceptors(authrpc.NewInterceptor(svc, auth.NewThrottle(), "")),
	))
	server := httptest.NewServer(mux)
	defer server.Close()
	client := postpilotv1connect.NewAuthServiceClient(server.Client(), server.URL)

	wrong := loginError(t, client, "waiting@example.com", "wrong")
	correct := loginError(t, client, "waiting@example.com", "password1")
	if wrong.Error() != correct.Error() {
		t.Fatalf("wrong and unverified responses differ:\nwrong: %s\ncorrect: %s", wrong, correct)
	}
}

// TestInterceptorRejectsTamperedCookie is plan 01 AC2 at the transport level.
func TestInterceptorRejectsTamperedCookie(t *testing.T) {
	client, _ := newServer(t)
	cookie := login(t, client)

	tampered := "pp_session=" + flipFirstChar(strings.TrimPrefix(cookie, "pp_session="))
	_, err := client.GetMe(context.Background(), withCookie(&postpilotv1.GetMeRequest{}, tampered))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("tampered cookie: %v, want unauthenticated", err)
	}
}

// TestInterceptorCoversStreamingHandlers guards the gap connect.UnaryInterceptorFunc
// leaves: its WrapStreamingHandler is a pass-through, so a streaming procedure added
// later (the generation job queue, plan 05) would ship with no session check at all.
func TestInterceptorCoversStreamingHandlers(t *testing.T) {
	svc := auth.NewService(newStore(t), sessionTTL)
	interceptor := authrpc.NewInterceptor(svc, auth.NewThrottle(), "")

	reached := false
	wrapped := interceptor.WrapStreamingHandler(func(context.Context, connect.StreamingHandlerConn) error {
		reached = true
		return nil
	})

	err := wrapped(context.Background(), &fakeStreamConn{
		spec: connect.Spec{Procedure: "/postpilot.v1.SomeFutureService/Stream"},
	})
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("streaming call without a cookie: %v, want unauthenticated", err)
	}
	if reached {
		t.Error("the streaming handler ran without a session")
	}
}

func TestInterceptorThrottlesStreamingHandlersWithTheSamePeerKey(t *testing.T) {
	svc := auth.NewService(newStore(t), sessionTTL)
	interceptor := authrpc.NewInterceptor(svc, auth.NewThrottle(), "")
	reached := 0
	wrapped := interceptor.WrapStreamingHandler(func(context.Context, connect.StreamingHandlerConn) error {
		reached++
		return nil
	})
	conn := &fakeStreamConn{
		spec: connect.Spec{Procedure: postpilotv1connect.AuthServiceLoginProcedure},
		peer: connect.Peer{Addr: "192.0.2.8:4444"},
	}
	for attempt := 1; attempt <= 10; attempt++ {
		if err := wrapped(context.Background(), conn); err != nil {
			t.Fatalf("stream attempt %d: %v", attempt, err)
		}
	}
	if err := wrapped(context.Background(), conn); connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("stream attempt 11 = %v, want resource_exhausted", err)
	}
	if reached != 10 {
		t.Fatalf("stream handler reached %d times, want 10", reached)
	}
}

// TestLogoutWithoutASessionStillClearsTheCookie: the cookie is HttpOnly, so if the
// server answers 401 instead of a clearing Set-Cookie, nothing can remove it from the
// browser and a dead session's cookie lingers for its full 30 days.
func TestLogoutWithoutASessionStillClearsTheCookie(t *testing.T) {
	client, _ := newServer(t)

	res, err := client.Logout(context.Background(), connect.NewRequest(&postpilotv1.LogoutRequest{}))
	if err != nil {
		t.Fatalf("Logout without a session: %v", err)
	}
	if got, want := res.Header().Get("Set-Cookie"), "pp_session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax"; got != want {
		t.Errorf("clearing cookie mismatch\n got: %s\nwant: %s", got, want)
	}
}

// TestLogoutRevokesAndClears is plan 01 AC6: replaying the same cookie after logout
// fails server-side, so a stolen copy is worthless.
func TestLogoutRevokesAndClears(t *testing.T) {
	client, _ := newServer(t)
	cookie := login(t, client)

	res, err := client.Logout(context.Background(), withCookie(&postpilotv1.LogoutRequest{}, cookie))
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}

	// Same attributes as the cookie it replaces, or the browser keeps both.
	if got, want := res.Header().Get("Set-Cookie"), "pp_session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Lax"; got != want {
		t.Errorf("clearing cookie mismatch\n got: %s\nwant: %s", got, want)
	}

	_, err = client.GetMe(context.Background(), withCookie(&postpilotv1.GetMeRequest{}, cookie))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("replayed cookie after logout: %v, want unauthenticated", err)
	}
}

// --- helpers ---

func login(t *testing.T, client postpilotv1connect.AuthServiceClient) string {
	t.Helper()
	res, err := client.Login(context.Background(), connect.NewRequest(&postpilotv1.LoginRequest{
		LoginId:  "alice",
		Password: "s3cret",
	}))
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return "pp_session=" + sessionToken(t, res.Header().Get("Set-Cookie"))
}

func loginError(t *testing.T, client postpilotv1connect.AuthServiceClient, id, password string) error {
	t.Helper()
	res, err := client.Login(context.Background(), connect.NewRequest(&postpilotv1.LoginRequest{
		LoginId:  id,
		Password: password,
	}))
	if err == nil {
		t.Fatalf("Login(%q) unexpectedly succeeded", id)
	}
	if res != nil {
		t.Fatalf("Login(%q) returned a response alongside an error", id)
	}
	return err
}

func withCookie[T any](msg *T, cookie string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Cookie", cookie)
	return req
}

func sessionToken(t *testing.T, setCookie string) string {
	t.Helper()
	header := http.Header{}
	header.Add("Set-Cookie", setCookie)
	for _, c := range (&http.Response{Header: header}).Cookies() {
		if c.Name == "pp_session" {
			return c.Value
		}
	}
	t.Fatalf("no pp_session cookie in %q", setCookie)
	return ""
}

func resetTokenFromMail(t *testing.T, mail auth.Mail) string {
	t.Helper()
	for _, line := range strings.Split(mail.Text, "\n") {
		parsed, err := url.Parse(line)
		if err == nil && parsed.Path == "/reset-password" && parsed.Query().Get("token") != "" {
			return parsed.Query().Get("token")
		}
	}
	t.Fatalf("password reset mail has no token URL: %s", mail.Text)
	return ""
}

// fakeStreamConn is the minimum connect.StreamingHandlerConn the interceptor touches:
// it reads the spec and the request headers, and nothing else.
type fakeStreamConn struct {
	spec   connect.Spec
	header http.Header
	peer   connect.Peer
}

func (c *fakeStreamConn) Spec() connect.Spec { return c.spec }
func (c *fakeStreamConn) RequestHeader() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}
func (c *fakeStreamConn) Peer() connect.Peer           { return c.peer }
func (c *fakeStreamConn) Receive(any) error            { return nil }
func (c *fakeStreamConn) Send(any) error               { return nil }
func (c *fakeStreamConn) ResponseHeader() http.Header  { return http.Header{} }
func (c *fakeStreamConn) ResponseTrailer() http.Header { return http.Header{} }

func flipFirstChar(s string) string {
	if s == "" {
		return s
	}
	replacement := byte('A')
	if s[0] == replacement {
		replacement = 'B'
	}
	return string(replacement) + s[1:]
}

func authAppErrorDetail(t *testing.T, err error) *postpilotv1.AppErrorDetail {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error type = %T, want *connect.Error", err)
	}
	if len(connectErr.Details()) != 1 {
		t.Fatalf("details = %d, want 1", len(connectErr.Details()))
	}
	value, valueErr := connectErr.Details()[0].Value()
	if valueErr != nil {
		t.Fatalf("decode detail: %v", valueErr)
	}
	detail, ok := value.(*postpilotv1.AppErrorDetail)
	if !ok {
		t.Fatalf("detail type = %T", value)
	}
	return detail
}
