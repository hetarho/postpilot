package openaicompat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// The strict path replaces the provider's prose with a fixed sentence; the cause, a
// code-owned word, survives that sanitizing.
func TestStrictGuardrailRefusalKeepsItsCauseButNotItsProse(t *testing.T) {
	const refusal = "0 endpoints out of 1 requested are available matching your guardrail restrictions and data policy. We removed them for the following reasons (an endpoint may have matched multiple reasons):\nPaid model training violation (account settings): 1 endpoint excluded; configurable at https://openrouter.ai/settings/privacy"
	req := strictRequest()
	c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			endpointResponse(w, req.Model, strictEndpointFixture(req.Model))
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"message":"`+strings.ReplaceAll(refusal, "\n", `\n`)+`","code":403}}`)
	})
	_, err := c.Complete(context.Background(), req)
	if !errors.Is(err, llm.ErrModelUnavailable) {
		t.Fatalf("err = %v, want ErrModelUnavailable", err)
	}
	var perr *llm.ProviderError
	if !errors.As(err, &perr) || perr.Cause != llm.ModelUnavailableCausePolicy {
		t.Fatalf("ProviderError = %+v", perr)
	}
	if strings.Contains(err.Error(), "guardrail") || strings.Contains(err.Error(), "openrouter.ai") {
		t.Fatalf("strict error carries provider prose: %v", err)
	}
	failure := llm.NormalizeFailure(err)
	if failure.Reason != llm.FailureReasonModelUnavailable || failure.Params["cause"] != llm.ModelUnavailableCausePolicy || len(failure.Params) != 1 {
		t.Fatalf("normalized failure = %#v", failure)
	}
}
