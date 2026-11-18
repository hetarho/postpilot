package openaicompat_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// The refusal OpenRouter sent, verbatim, for a model whose only endpoint the account's
// privacy setting excludes (2026-09, muse-spark *-contributor).
const guardrailRefusal = "0 endpoints out of 1 requested are available matching your guardrail restrictions and data policy. We removed them for the following reasons (an endpoint may have matched multiple reasons):\nPaid model training violation (account settings): 1 endpoint excluded; configurable at https://openrouter.ai/settings/privacy"

func envelope(message string, code int) string {
	body, _ := json.Marshal(map[string]any{"error": map[string]any{"message": message, "code": code}})
	return string(body)
}

// assertModelUnavailable checks the error and its durable projection: the reason, the
// code-owned cause as the only param, and the provider's text kept as technical detail.
func assertModelUnavailable(t *testing.T, err error, cause, message string) {
	t.Helper()
	if !errors.Is(err, llm.ErrModelUnavailable) {
		t.Fatalf("err = %v, want ErrModelUnavailable", err)
	}
	var perr *llm.ProviderError
	if !errors.As(err, &perr) || perr.Cause != cause || perr.Message != message {
		t.Fatalf("ProviderError = %+v, want cause %q and the provider's message", perr, cause)
	}
	failure := llm.NormalizeFailure(err)
	if failure.Reason != llm.FailureReasonModelUnavailable || len(failure.Params) != 1 || failure.Params["cause"] != cause || failure.TechnicalDetail != message {
		t.Fatalf("normalized failure = %#v", failure)
	}
}

func TestComplete_GuardrailRefusalIsAPolicyUnavailableModelAtAnyStatus(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest, http.StatusForbidden, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				io.WriteString(w, envelope(guardrailRefusal, status))
			})
			_, err := client.Complete(context.Background(), llm.Request{Model: "meta/muse-spark-1.3:contributor"})
			assertModelUnavailable(t, err, llm.ModelUnavailableCausePolicy, guardrailRefusal)
		})
	}
	t.Run("plain text body", func(t *testing.T) {
		client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, guardrailRefusal)
		})
		_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
		assertModelUnavailable(t, err, llm.ModelUnavailableCausePolicy, guardrailRefusal)
	})
}

func TestComplete_GuardrailRefusalInsideA200Body(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"json answer with upstream 404": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, envelope(guardrailRefusal, 404))
		},
		"json answer with upstream 400": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, envelope(guardrailRefusal, 400))
		},
		"stream error chunk": func(w http.ResponseWriter, r *http.Request) {
			sse(w, envelope(guardrailRefusal, 403))
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			client, _ := newClient(t, handler)
			_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
			assertModelUnavailable(t, err, llm.ModelUnavailableCausePolicy, guardrailRefusal)
		})
	}
}

func TestComplete_NoEndpointRefusalsTellPolicyFromGone(t *testing.T) {
	cases := []struct {
		status  int
		message string
		cause   string
	}{
		{http.StatusNotFound, "No endpoints found for meta/muse-spark-1.3.", llm.ModelUnavailableCauseGone},
		{http.StatusBadRequest, "No allowed providers are available for the selected model.", llm.ModelUnavailableCauseGone},
		{http.StatusNotFound, "No endpoints found matching your data policy (Free model publication). Configure: https://openrouter.ai/settings/privacy", llm.ModelUnavailableCausePolicy},
		{http.StatusBadRequest, "No endpoints available matching your data policy", llm.ModelUnavailableCausePolicy},
		{http.StatusServiceUnavailable, "No endpoints available matching your data policy", llm.ModelUnavailableCausePolicy},
		{http.StatusServiceUnavailable, "No endpoints available for this model", llm.ModelUnavailableCauseGone},
		{http.StatusForbidden, "No endpoints found that respect your privacy settings", llm.ModelUnavailableCausePolicy},
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status)+" "+tc.message, func(t *testing.T) {
			client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, envelope(tc.message, tc.status))
			})
			_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
			assertModelUnavailable(t, err, tc.cause, tc.message)
		})
		t.Run("in body "+tc.message, func(t *testing.T) {
			client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				sse(w, `{"choices":[{"delta":{"content":"par"}}]}`, envelope(tc.message, tc.status))
			})
			_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
			assertModelUnavailable(t, err, tc.cause, tc.message)
		})
	}
}

// A 200 body's upstream 404 without any message about endpoints is still the model gone;
// a refusal for another reason stays generic and carries no cause.
func TestComplete_InBodyRefusalsKeepTheirOtherKinds(t *testing.T) {
	client, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w, envelope("upstream said not found", 404))
	})
	_, err := client.Complete(context.Background(), llm.Request{Model: "m"})
	assertModelUnavailable(t, err, llm.ModelUnavailableCauseGone, "upstream said not found")

	client, _ = newClient(t, func(w http.ResponseWriter, r *http.Request) {
		sse(w, envelope("No endpoints available matching your data policy", 429))
	})
	_, err = client.Complete(context.Background(), llm.Request{Model: "m"})
	if !errors.Is(err, llm.ErrRateLimited) {
		t.Fatalf("an upstream 429 = %v, want ErrRateLimited", err)
	}

	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		client, _ = newClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			io.WriteString(w, envelope("Key is not allowed to use this route", status))
		})
		_, err = client.Complete(context.Background(), llm.Request{Model: "m"})
		var perr *llm.ProviderError
		if !errors.As(err, &perr) || perr.Kind != nil || perr.Cause != "" {
			t.Fatalf("%d generic refusal = %+v", status, perr)
		}
		if failure := llm.NormalizeFailure(err); failure.Reason != llm.FailureReasonUnknown || failure.Params != nil {
			t.Fatalf("%d normalized = %#v", status, failure)
		}
	}
}
