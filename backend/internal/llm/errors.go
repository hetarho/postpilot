package llm

import (
	"errors"
	"fmt"
	"strings"
)

// The normalized failures callers can act on. A job layer maps these stable reasons
// to localized recovery guidance at the transport edge.
var (
	// ErrModelUnavailable: the ref is not in the registry, or the provider says the model
	// does not exist (a free model that vanished — PRD §6.5).
	ErrModelUnavailable = errors.New("model unavailable")
	// ErrProviderDisabled: the provider's API key is not configured.
	ErrProviderDisabled = errors.New("provider disabled: API key not configured")
	// ErrRateLimited: the provider answered 429 (the free-tier daily limit case).
	ErrRateLimited = errors.New("rate limited")
	// ErrUnsupported: an image part or a JSON schema was requested on a model that does
	// not declare the capability. Raised before any network call.
	ErrUnsupported = errors.New("unsupported by model")
	// ErrBadOutput: the provider answered, but with nothing usable.
	ErrBadOutput = errors.New("bad model output")
	// ErrOutputTruncated: the provider consumed the completion budget before it
	// produced usable content.
	ErrOutputTruncated = errors.New("model output truncated by completion budget")
)

// Stable failure reasons owned by the LLM context. They are intentionally prose-free:
// callers persist or project them while choosing the user-facing copy at the edge.
const (
	FailureReasonProviderDisabled = "PROVIDER_DISABLED"
	FailureReasonModelUnavailable = "MODEL_UNAVAILABLE"
	FailureReasonModelRateLimited = "MODEL_RATE_LIMITED"
	FailureReasonModelUnsupported = "MODEL_UNSUPPORTED"
	FailureReasonOutputInvalid    = "MODEL_OUTPUT_INVALID"
	FailureReasonOutputTruncated  = "MODEL_OUTPUT_TRUNCATED"
	FailureReasonUnknown          = "UNKNOWN_FAILURE"
)

// Failure is the provider-neutral, durable projection of an LLM error.
//
// Params is nil when there are no display-safe interpolation values. LLM failures
// currently expose none: provider names, status text, and messages are not public
// parameters. TechnicalDetail may contain the provider's original message and must
// remain separate from localized user-facing copy.
//
// The zero value means no failure and is returned only for a nil error.
type Failure struct {
	Reason          string
	Params          map[string]string
	TechnicalDetail string
}

// NormalizeFailure converts provider and adapter failures into the context-owned
// structured contract. It retains errors.Is behavior on the original error; callers
// use this projection only for persistence or transport.
func NormalizeFailure(err error) Failure {
	if err == nil {
		return Failure{}
	}

	failure := Failure{Reason: failureReason(err)}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		// The provider's prose is deliberately technical-only. In particular, do not
		// copy it into Params or make it the stable reason.
		failure.TechnicalDetail = strings.TrimSpace(providerErr.Message)
	}
	return failure
}

func failureReason(err error) string {
	switch {
	case errors.Is(err, ErrProviderDisabled):
		return FailureReasonProviderDisabled
	case errors.Is(err, ErrModelUnavailable):
		return FailureReasonModelUnavailable
	case errors.Is(err, ErrRateLimited):
		return FailureReasonModelRateLimited
	case errors.Is(err, ErrUnsupported):
		return FailureReasonModelUnsupported
	case errors.Is(err, ErrOutputTruncated):
		return FailureReasonOutputTruncated
	case errors.Is(err, ErrBadOutput):
		return FailureReasonOutputInvalid
	default:
		return FailureReasonUnknown
	}
}

// ProviderError is what an adapter returns for an HTTP-level failure. It keeps the
// provider's own message as technical detail and wraps the normalized sentinel it
// maps to, so `errors.Is(err, ErrRateLimited)` still works.
type ProviderError struct {
	Provider string
	Status   int
	Message  string
	// Kind is the sentinel this failure normalizes to, or nil for a generic failure.
	Kind error
}

func (e *ProviderError) Error() string {
	if e.Kind != nil {
		return fmt.Sprintf("%s: %v (HTTP %d): %s", e.Provider, e.Kind, e.Status, e.Message)
	}
	return fmt.Sprintf("%s: HTTP %d: %s", e.Provider, e.Status, e.Message)
}

func (e *ProviderError) Unwrap() error { return e.Kind }
