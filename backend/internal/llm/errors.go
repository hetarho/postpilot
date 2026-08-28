package llm

import (
	"errors"
	"fmt"
)

// The normalized failures callers can act on. A job layer maps these to what the user
// sees (PRD §7: 한도 초과 → 다른 모델 선택 유도, 사라진 모델 → 다시 선택).
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
)

// ProviderError is what an adapter returns for an HTTP-level failure. It keeps the
// provider's own message — the user is told the cause verbatim (PRD §7) — and wraps the
// normalized sentinel it maps to, so `errors.Is(err, ErrRateLimited)` still works.
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
