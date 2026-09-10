package llm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCallDiagnosticsPreserveClassificationWithoutPublicMetadata(t *testing.T) {
	cause := &ProviderError{Status: 429, Code: 429, Message: "sanitized provider failure", Kind: ErrRateLimited}
	info := CallDiagnostic{Operation: "response", Class: "http_error", HTTPStatus: 429, UpstreamCode: 429, RequestID: "req-0123456789abcdef"}
	err := WithCallDiagnostic(cause, info)
	var provider *ProviderError
	if err.Error() != cause.Error() || !errors.Is(err, ErrRateLimited) || !errors.As(err, &provider) || provider != cause || !reflect.DeepEqual(NormalizeFailure(err), NormalizeFailure(cause)) {
		t.Fatal("diagnostics changed public or typed failure behavior")
	}
	got, ok := DiagnosticOf(fmt.Errorf("outer: %w", err))
	if !ok || got != info || strings.Contains(err.Error(), info.RequestID) {
		t.Fatal(got, ok, err)
	}
	if WithCallDiagnostic(nil, info) != nil {
		t.Fatal("successful request became a failure")
	}
}

func TestCallDiagnosticsRejectUntrustedMetadata(t *testing.T) {
	for _, id := range []string{"", "https://private.test/?token=secret", "data:video/mp4;base64,secret", "sk-or-private-secret", "req-12345678\nprivate", strings.Repeat("a", 129), "private body", "gen-secret", "/private/source.mp4"} {
		info, ok := DiagnosticOf(WithCallDiagnostic(ErrUnsupported, CallDiagnostic{Operation: id, Class: id, HTTPStatus: 999, UpstreamCode: -1, RequestID: id}))
		if !ok || info != (CallDiagnostic{Operation: "unknown", Class: "unknown"}) {
			t.Fatal("unsafe diagnostic accepted", info)
		}
	}
	for _, id := range []string{"req-0123456789abcdef", "req_0123456789abcdef", "gen-1789050000-0123456789abcdef", "aabbccdd00112233-ICN", "aabbccdd-0011-2233-4455-66778899aabb"} {
		info, _ := DiagnosticOf(WithCallDiagnostic(ErrUnsupported, CallDiagnostic{RequestID: id}))
		if info.RequestID != id {
			t.Fatal("opaque request ID lost", id)
		}
	}
}
