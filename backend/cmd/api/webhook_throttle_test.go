package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/postpilot/backend/internal/auth"
)

// The webhook route bypasses the Connect interceptors, so it used to carry no throttle at
// all: every anonymous POST forced one outbound provider call (review/diff-260908 F6).
func TestWebhookRouteRefusesABurstBeforeReachingTheHandler(t *testing.T) {
	handlerCalls := 0
	route := throttledRoute(auth.NewThrottle(), auth.ThrottleWebhook, "",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			handlerCalls++
			w.WriteHeader(http.StatusOK)
		}))

	post := func(remoteAddr string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/webhooks/toss", nil)
		request.RemoteAddr = remoteAddr
		recorder := httptest.NewRecorder()
		route.ServeHTTP(recorder, request)
		return recorder
	}

	// The window's own size decides where the refusal lands; what this pins is that a burst
	// past it stops reaching the handler at all.
	allowed := 0
	var refused *httptest.ResponseRecorder
	for range 200 {
		if recorder := post("203.0.113.9:41000"); recorder.Code == http.StatusOK {
			allowed++
		} else if refused == nil {
			refused = recorder
		}
	}
	if refused == nil {
		t.Fatal("a 200-request burst was never refused")
	}
	if refused.Code != http.StatusTooManyRequests || refused.Header().Get("Retry-After") == "" {
		t.Fatalf("refusal = %d, Retry-After %q", refused.Code, refused.Header().Get("Retry-After"))
	}
	if handlerCalls != allowed || allowed >= 200 {
		t.Fatalf("handler calls = %d, allowed = %d: a refused request must not reach the handler", handlerCalls, allowed)
	}

	// One IP's burst does not spend another's window: the limit is per client, not global.
	if recorder := post("198.51.100.4:41000"); recorder.Code != http.StatusOK {
		t.Fatalf("second IP = %d, want it unaffected", recorder.Code)
	}
}

// The limiter key comes from the same resolver the auth interceptor uses, so the configured
// ingress header means one thing across the process.
func TestWebhookRouteKeysOnTheConfiguredClientIPHeader(t *testing.T) {
	throttle := auth.NewThrottle()
	route := throttledRoute(throttle, auth.ThrottleWebhook, "X-Forwarded-For",
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	// Same peer, two forwarded clients: they must not share a window.
	exhaust := func(forwarded string) (allowed int) {
		for range 200 {
			request := httptest.NewRequest(http.MethodPost, "/webhooks/toss", nil)
			request.RemoteAddr = "10.0.0.1:9999"
			request.Header.Set("X-Forwarded-For", forwarded)
			recorder := httptest.NewRecorder()
			route.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusOK {
				allowed++
			}
		}
		return allowed
	}
	first := exhaust("203.0.113.9")
	second := exhaust("198.51.100.4")
	if first == 0 || second == 0 || first != second {
		t.Fatalf("per-header windows = %d and %d, want the same non-zero allowance each", first, second)
	}
}
