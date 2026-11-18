package authoring

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// A request's model failure names the model it ran on in MODEL_UNAVAILABLE's params, next
// to the llm context's cause; every other reason passes through as normalized.
func TestModelErrorAddsTheModelLabelToModelUnavailableOnly(t *testing.T) {
	const prose = "0 endpoints out of 1 requested are available matching your guardrail restrictions and data policy."
	policy := &llm.ProviderError{Provider: "openrouter", Status: 404, Message: prose, Kind: llm.ErrModelUnavailable, Cause: llm.ModelUnavailableCausePolicy}

	var detailed interface{ Failure() llm.Failure }
	err := error(&ModelError{Cause: policy, Label: " Muse Spark 1.3 "})
	if !errors.As(err, &detailed) || !errors.Is(err, llm.ErrModelUnavailable) {
		t.Fatal("ModelError hides its failure or its kind")
	}
	failure := detailed.Failure()
	if failure.Reason != llm.FailureReasonModelUnavailable || len(failure.Params) != 2 || failure.Params["model"] != "Muse Spark 1.3" || failure.Params["cause"] != llm.ModelUnavailableCausePolicy || failure.TechnicalDetail != prose {
		t.Fatalf("failure = %#v", failure)
	}

	if f := (&ModelError{Cause: policy, Label: "  "}).Failure(); len(f.Params) != 1 || f.Params["cause"] != llm.ModelUnavailableCausePolicy {
		t.Fatalf("blank label failure = %#v", f)
	}
	if f := (&ModelError{Cause: llm.ErrModelUnavailable, Label: "Muse"}).Failure(); len(f.Params) != 1 || f.Params["model"] != "Muse" {
		t.Fatalf("causeless failure = %#v", f)
	}
	limited := &llm.ProviderError{Message: "slow down", Kind: llm.ErrRateLimited}
	if f := (&ModelError{Cause: limited, Label: "Muse"}).Failure(); f.Reason != llm.FailureReasonModelRateLimited || f.Params != nil {
		t.Fatalf("rate-limited failure = %#v", f)
	}
	if f := (&ModelError{Cause: errors.New("boom"), Label: "Muse"}).Failure(); f.Reason != llm.FailureReasonUnknown || f.Params != nil {
		t.Fatalf("unknown failure = %#v", f)
	}
}
