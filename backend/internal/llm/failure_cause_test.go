package llm_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// MODEL_UNAVAILABLE carries its code-owned cause and nothing else; no other reason does,
// and the provider's prose stays technical detail.
func TestNormalizeFailureCarriesOnlyTheModelUnavailableCause(t *testing.T) {
	const prose = "0 endpoints out of 1 requested are available matching your guardrail restrictions"
	cases := []struct {
		name   string
		err    error
		reason string
		params map[string]string
	}{
		{"policy", &llm.ProviderError{Provider: "openrouter", Status: 404, Message: prose, Kind: llm.ErrModelUnavailable, Cause: llm.ModelUnavailableCausePolicy}, llm.FailureReasonModelUnavailable, map[string]string{"cause": "provider_policy"}},
		{"gone", &llm.ProviderError{Provider: "openrouter", Status: 404, Message: "No endpoints found", Kind: llm.ErrModelUnavailable, Cause: llm.ModelUnavailableCauseGone}, llm.FailureReasonModelUnavailable, map[string]string{"cause": "gone"}},
		{"wrapped", fmt.Errorf("authoring: %w", &llm.ProviderError{Message: prose, Kind: llm.ErrModelUnavailable, Cause: llm.ModelUnavailableCausePolicy}), llm.FailureReasonModelUnavailable, map[string]string{"cause": "provider_policy"}},
		{"no cause", &llm.ProviderError{Message: "model x does not exist", Kind: llm.ErrModelUnavailable}, llm.FailureReasonModelUnavailable, nil},
		{"cause on another kind", &llm.ProviderError{Message: "slow down", Kind: llm.ErrRateLimited, Cause: llm.ModelUnavailableCausePolicy}, llm.FailureReasonModelRateLimited, nil},
		{"registry miss", llm.ErrModelUnavailable, llm.FailureReasonModelUnavailable, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := llm.NormalizeFailure(tc.err)
			if failure.Reason != tc.reason || len(failure.Params) != len(tc.params) {
				t.Fatalf("failure = %#v", failure)
			}
			for key, value := range tc.params {
				if failure.Params[key] != value {
					t.Fatalf("params = %v, want %v", failure.Params, tc.params)
				}
			}
			for _, value := range failure.Params {
				if value == prose {
					t.Fatal("provider prose reached params")
				}
			}
			var perr *llm.ProviderError
			if errors.As(tc.err, &perr) && failure.TechnicalDetail != perr.Message {
				t.Fatalf("technical detail = %q", failure.TechnicalDetail)
			}
		})
	}
}
