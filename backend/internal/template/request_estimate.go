package template

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// The token assumptions one template request is priced at (QUOTA-67): about the 형식 안내, a full
// draft and a few thousand characters of request in, and a full body out. Code-owned like the
// post estimate's own assumptions; the hold still prices the worst case of every correction.
const (
	requestEstimatePromptTokens     = 6_000
	requestEstimateCompletionTokens = 3_000
)

// RequestEstimate is what the request box states before it is pressed: 무료, 약 n 크레딧, or
// nothing when the model's price cannot be bounded. It is never a quote.
type RequestEstimate struct {
	Free      bool
	Credits   int
	Available bool
}

// RequestEstimator prices one assumed call on a model at catalog prices, in credits only — no
// cost, price or conversion ever leaves it (QUOTA-65).
type RequestEstimator interface {
	CallCredits(ctx context.Context, info llm.ModelInfo, promptTokens, completionTokens int64) (int, bool)
}

// ConfigureEstimate wires the request box's credit figure (TMPL-62).
func (s *Service) ConfigureEstimate(estimator RequestEstimator) { s.estimator = estimator }

// EstimateRequest is one template request on the write model the client names, priced as the
// catalog estimate of one call (QUOTA-67). A free-classified write registration is 무료; a model
// that is unknown, unclassified, not a writer or unpriced has no figure, which is not an error.
func (s *Service) EstimateRequest(ctx context.Context, writeModel string) (RequestEstimate, error) {
	if s.requests == nil || s.estimator == nil {
		return RequestEstimate{}, errRequestsUnwired
	}
	ref, ok := parseModelRef(writeModel)
	if !ok {
		return RequestEstimate{}, nil
	}
	info, found := s.requests.models.Resolve(ref)
	if !found || info.Disabled || !info.ServesStage(llm.StageNameWrite) {
		return RequestEstimate{}, nil
	}
	switch grade := info.Levels[llm.StageNameWrite]; grade {
	case "":
		return RequestEstimate{}, nil
	case "free":
		return RequestEstimate{Free: true, Available: true}, nil
	}
	credits, ok := s.estimator.CallCredits(ctx, info, requestEstimatePromptTokens, requestEstimateCompletionTokens)
	if !ok {
		return RequestEstimate{}, nil
	}
	return RequestEstimate{Credits: credits, Available: true}, nil
}
