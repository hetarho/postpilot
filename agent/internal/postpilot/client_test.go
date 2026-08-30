package postpilot

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestBearerTransportBindsCredentialToConfiguredOrigin(t *testing.T) {
	origin, _ := url.Parse("https://api.postpilot.example")
	seen := ""
	transport := bearerTransport{
		token:  "raw-secret",
		origin: origin,
		base: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			seen = request.Header.Get("Authorization")
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}
	allowed, _ := http.NewRequest(http.MethodPost, "https://api.postpilot.example/postpilot.v1.PublishingAgentService/ClaimPublishJob", nil)
	if _, err := transport.RoundTrip(allowed); err != nil {
		t.Fatal(err)
	}
	if seen != "Bearer raw-secret" {
		t.Fatalf("authorization = %q", seen)
	}

	redirected, _ := http.NewRequest(http.MethodPost, "https://attacker.example/collect", nil)
	redirected.Header.Set("Authorization", "caller-value")
	if _, err := transport.RoundTrip(redirected); err == nil {
		t.Fatal("cross-origin request received the bearer transport")
	}
	if redirected.Header.Get("Authorization") != "caller-value" {
		t.Fatal("rejected request was mutated")
	}
}

func TestAgentClientRejectsRedirects(t *testing.T) {
	if err := rejectRedirect(nil, nil); err == nil {
		t.Fatal("redirect was accepted")
	}
}
