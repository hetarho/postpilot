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
	// does not exist — a curated row the provider stopped offering (PRD §6.5).
	ErrModelUnavailable = errors.New("model unavailable")
	// ErrProviderDisabled: the provider's API key is not configured.
	ErrProviderDisabled = errors.New("provider disabled: API key not configured")
	// ErrRateLimited: the provider refused for rate reasons — the caller's quota, the
	// account's, or the gateway's own upstream pool. It says nothing about a tier: the
	// 2026-09-03 case arrived as an upstream `code: 429` inside an HTTP 200 on a catalog row
	// priced at $0.75/$3.75 per million, with nothing free anywhere in the path.
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
	var truncated *TruncatedError
	if errors.As(err, &truncated) {
		failure.TechnicalDetail = truncated.Error()
	}
	return failure
}

// TruncatedError is ErrOutputTruncated carrying the provider's own account of where the
// completion budget went. It exists because the two causes have opposite remedies and were
// indistinguishable from the message: a body that filled the budget wants a larger budget,
// while a body the model never wrote because it spent the budget reasoning wants a lower
// effort for this purpose, or another model. Only a provider that reports the split can be
// told apart, so one that reports none keeps the bare sentinel and today's message.
//
// It wraps the sentinel, so every `errors.Is(err, ErrOutputTruncated)` check and the
// MODEL_OUTPUT_TRUNCATED reason are unaffected, and no user-facing string changes — this
// text is TechnicalDetail, the field reserved for external prose.
type TruncatedError struct {
	ReasoningTokens  int
	CompletionTokens int
}

func (e *TruncatedError) Error() string {
	visible := e.CompletionTokens - e.ReasoningTokens
	if visible < 0 {
		visible = 0
	}
	return fmt.Sprintf(
		"completion budget exhausted: %d of %d completion tokens went to reasoning, %d to visible output",
		e.ReasoningTokens, e.CompletionTokens, visible,
	)
}

func (e *TruncatedError) Unwrap() error { return ErrOutputTruncated }

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
