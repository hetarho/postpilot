package rpc_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	authrpc "github.com/postpilot/backend/internal/auth/rpc"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

const sessionTTL = 720 * time.Hour

// newServer wires the real handler and interceptor over a real SQLite store and
// returns a client speaking to them across a real HTTP server.
//
// Nothing here is faked: these tests are about what a browser actually receives —
// the Set-Cookie string, the 401 — so a stubbed store or an in-process handler call
// would assert the wrong thing.
func newServer(t *testing.T) (postpilotv1connect.AuthServiceClient, *httptest.Server) {
	t.Helper()

	svc := auth.NewService(newStore(t), sessionTTL)
	if err := svc.CreateUser(context.Background(), "alice", "s3cret"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewAuthServiceHandler(
		authrpc.NewHandler(svc, sessionTTL),
		connect.WithInterceptors(authrpc.NewInterceptor(svc)),
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
	if strings.Contains(res.Msg.String(), token) {
		t.Errorf("the session token leaked into the response body: %s", res.Msg.String())
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
	interceptor := authrpc.NewInterceptor(svc)

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

// fakeStreamConn is the minimum connect.StreamingHandlerConn the interceptor touches:
// it reads the spec and the request headers, and nothing else.
type fakeStreamConn struct {
	spec   connect.Spec
	header http.Header
}

func (c *fakeStreamConn) Spec() connect.Spec { return c.spec }
func (c *fakeStreamConn) RequestHeader() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}
func (c *fakeStreamConn) Peer() connect.Peer           { return connect.Peer{} }
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
