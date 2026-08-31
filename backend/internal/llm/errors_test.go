package llm_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestNormalizeFailureMapsStableReasonsWithoutPublicParams(t *testing.T) {
	tests := map[string]struct {
		err    error
		reason string
	}{
		"provider disabled": {fmt.Errorf("registry: %w", llm.ErrProviderDisabled), llm.FailureReasonProviderDisabled},
		"model unavailable": {fmt.Errorf("registry: %w", llm.ErrModelUnavailable), llm.FailureReasonModelUnavailable},
		"rate limited":      {fmt.Errorf("adapter: %w", llm.ErrRateLimited), llm.FailureReasonModelRateLimited},
		"unsupported":       {fmt.Errorf("registry: %w", llm.ErrUnsupported), llm.FailureReasonModelUnsupported},
		"invalid output":    {fmt.Errorf("adapter: %w", llm.ErrBadOutput), llm.FailureReasonOutputInvalid},
		"truncated output":  {fmt.Errorf("adapter: %w", llm.ErrOutputTruncated), llm.FailureReasonOutputTruncated},
		"unknown":           {errors.New("dial tcp: private upstream detail"), llm.FailureReasonUnknown},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			failure := llm.NormalizeFailure(test.err)
			if failure.Reason != test.reason {
				t.Fatalf("reason = %q, want %q", failure.Reason, test.reason)
			}
			if failure.Params != nil {
				t.Fatalf("params = %#v, want nil: LLM failures expose no interpolation values", failure.Params)
			}
			if failure.TechnicalDetail != "" {
				t.Fatalf("technical detail = %q, want empty for a non-provider error", failure.TechnicalDetail)
			}
		})
	}
}

func TestNormalizeFailureKeepsProviderMessageTechnicalOnly(t *testing.T) {
	providerErr := &llm.ProviderError{
		Provider: "private-provider",
		Status:   429,
		Message:  "  raw quota detail with <private prompt>  ",
		Kind:     llm.ErrRateLimited,
	}

	failure := llm.NormalizeFailure(fmt.Errorf("complete: %w", providerErr))
	if got, want := failure.Reason, llm.FailureReasonModelRateLimited; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
	if failure.Params != nil {
		t.Fatalf("provider prose entered params: %#v", failure.Params)
	}
	if got, want := failure.TechnicalDetail, "raw quota detail with <private prompt>"; got != want {
		t.Fatalf("technical detail = %q, want %q", got, want)
	}
	if !errors.Is(providerErr, llm.ErrRateLimited) {
		t.Fatal("normalization contract must preserve ProviderError errors.Is behavior")
	}
}

func TestNormalizeFailureGenericProviderErrorIsUnknown(t *testing.T) {
	providerErr := &llm.ProviderError{
		Provider: "private-provider",
		Status:   503,
		Message:  "  upstream incident id=private  ",
	}

	failure := llm.NormalizeFailure(providerErr)
	if got, want := failure.Reason, llm.FailureReasonUnknown; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
	if failure.Params != nil {
		t.Fatalf("params = %#v, want nil", failure.Params)
	}
	if got, want := failure.TechnicalDetail, "upstream incident id=private"; got != want {
		t.Fatalf("technical detail = %q, want %q", got, want)
	}
}

func TestNormalizeFailureNilHasZeroValueAndNoSharedParams(t *testing.T) {
	if got := llm.NormalizeFailure(nil); !reflect.DeepEqual(got, llm.Failure{}) {
		t.Fatalf("nil error = %#v, want zero Failure", got)
	}

	first := llm.NormalizeFailure(llm.ErrRateLimited)
	second := llm.NormalizeFailure(llm.ErrRateLimited)
	if first.Params != nil || second.Params != nil {
		t.Fatalf("params = %#v / %#v, want immutable-by-absence nil maps", first.Params, second.Params)
	}
}
