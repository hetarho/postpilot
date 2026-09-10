package openaicompat

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestStrictDiagnosticsKeepHTTPAndUpstreamCodesSeparate(t *testing.T) {
	for _, status := range []int{200, 400, 403, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			req := strictRequest()
			var posts atomic.Int32
			client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
					return
				}
				posts.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("X-Request-ID", "req-0123456789abcdef")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":{"code":403,"message":"private-canary data:video/mp4;base64,secret"},"usage":{"cost":0.004,"prompt_tokens":10}}`)
			})
			out, err := client.Complete(t.Context(), req)
			info, ok := llm.DiagnosticOf(err)
			class := "http_error"
			if status == 200 {
				class = "provider_error"
			}
			if !ok || info != (llm.CallDiagnostic{Operation: "response", Class: class, HTTPStatus: status, UpstreamCode: 403, RequestID: "req-0123456789abcdef"}) || posts.Load() != 1 {
				t.Fatal(info, err, posts.Load())
			}
			if out.Text != "" || !out.Usage.CostReported || out.Usage.CostMicrousd != 4000 || strings.Contains(err.Error(), "canary") || strings.Contains(err.Error(), info.RequestID) {
				t.Fatal("diagnostics leaked content or changed usage", out, err)
			}
			var provider *llm.ProviderError
			if !errors.As(err, &provider) || provider.Status != status || provider.Code != 403 {
				t.Fatal("typed provider cause was lost", err)
			}
		})
	}
}

func TestStrictDiagnosticsMetadataFailureNeverPosts(t *testing.T) {
	var calls atomic.Int32
	client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" {
			t.Error("metadata failure reached completion")
		}
		w.Header().Set("X-Request-ID", "https://private.test/secret")
		w.Header().Set("CF-Ray", "aabbccdd00112233-ICN")
		w.WriteHeader(403)
	})
	_, err := client.Complete(t.Context(), strictRequest())
	info, ok := llm.DiagnosticOf(err)
	if !ok || info != (llm.CallDiagnostic{Operation: "metadata", Class: "http_error", HTTPStatus: 403, RequestID: "aabbccdd00112233-ICN"}) || !errors.Is(err, llm.ErrUnsupported) || calls.Load() != 1 {
		t.Fatal(info, err, calls.Load())
	}
}

type diagnosticTransport func(*http.Request) (*http.Response, error)

func (f diagnosticTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStrictDiagnosticsDistinguishTransportAndLocalReadFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		err   error
		class string
	}{
		"canceled": {context.Canceled, "canceled"},
		"deadline": {context.DeadlineExceeded, "timeout"},
		"network":  {&net.DNSError{Err: "private-canary", Name: "private.test"}, "network_error"},
		"timeout":  {&net.DNSError{Err: "private-canary", IsTimeout: true}, "timeout"},
	} {
		t.Run(name, func(t *testing.T) {
			req := strictRequest()
			client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
			})
			transport := client.http.Transport
			client.http.Transport = diagnosticTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method == "GET" {
					return transport.RoundTrip(r)
				}
				_ = r.Body.Close()
				return nil, tc.err
			})
			_, err := client.Complete(t.Context(), req)
			info, ok := llm.DiagnosticOf(err)
			if !ok || info.Operation != "transport" || info.Class != tc.class || info.HTTPStatus != 0 || info.RequestID != "" || strings.Contains(err.Error(), "canary") {
				t.Fatal(info, err)
			}
			if (name == "canceled" || name == "deadline") && !errors.Is(err, tc.err) {
				t.Fatal("cancellation classification changed", err)
			}
		})
	}
	req := strictRequest()
	req.Messages[0].Parts[1].InlineVideo.Open = func(context.Context) (io.ReadCloser, error) { return nil, errors.New("private-canary") }
	client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("failed local open reached completion")
		}
		endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
	})
	_, err := client.Complete(t.Context(), req)
	info, _ := llm.DiagnosticOf(err)
	if info.Operation != "body" || info.Class != "body_read" || strings.Contains(err.Error(), "canary") {
		t.Fatal(info, err)
	}
}

func TestStrictDiagnosticsResponseFailuresAndIDRedaction(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		req := strictRequest()
		client := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
				return
			}
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("X-Request-ID", "req-0123456789abcdef")
			w.Header().Set("Request-ID", "sk-or-private-canary")
			body := "private-canary"
			if oversized {
				body = strings.Repeat("x", int(clipLimits.ResponseBytes)+1)
			}
			_, _ = io.WriteString(w, body)
		})
		client.apiKey = "req-0123456789abcdef" // Even a matching ID-shaped credential is forbidden.
		_, err := client.Complete(t.Context(), req)
		info, ok := llm.DiagnosticOf(err)
		class := "invalid_response"
		if oversized {
			class = "response_limit"
		}
		if !ok || info.Operation != "response" || info.HTTPStatus != 200 || info.Class != class || info.RequestID != "" || !errors.Is(err, llm.ErrBadOutput) || strings.Contains(err.Error(), "canary") {
			t.Fatal(info, err)
		}
	}
}

func TestDiagnosticErrorCodeIsOnlyABoundedNumber(t *testing.T) {
	for _, value := range []any{nil, "private-canary", "400\n", "999", float64(400.5), float64(999), map[string]any{"code": 400}} {
		if numericErrorCode(value) != 0 {
			t.Fatal("untrusted code accepted", value)
		}
	}
	for _, value := range []any{float64(403), "403"} {
		if numericErrorCode(value) != 403 {
			t.Fatal(value)
		}
	}
}
