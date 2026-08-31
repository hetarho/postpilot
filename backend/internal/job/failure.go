package job

import (
	"errors"

	"github.com/postpilot/backend/internal/llm"
)

const (
	FailureReasonInterrupted    = "JOB_INTERRUPTED"
	FailureReasonPanicked       = "JOB_PANICKED"
	FailureReasonHandlerMissing = "JOB_HANDLER_MISSING"
	FailureReasonUnknown        = "UNKNOWN_FAILURE"
)

// Failure is the job context's durable, prose-free failure contract. Params contains
// only display-safe interpolation values; TechnicalDetail is reserved for external
// provider detail and is never populated from an arbitrary handler error.
type Failure struct {
	Reason          string
	Params          map[string]string
	TechnicalDetail string
}

func cloneFailure(value *Failure) *Failure {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Params = cloneParams(value.Params)
	return &copy
}

func cloneParams(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

func failureFromError(err error) Failure {
	switch {
	case errors.Is(err, errHandlerPanicked):
		return Failure{Reason: FailureReasonPanicked}
	case errors.Is(err, errHandlerMissing):
		return Failure{Reason: FailureReasonHandlerMissing}
	}
	normalized := llm.NormalizeFailure(err)
	return Failure{
		Reason:          normalized.Reason,
		Params:          cloneParams(normalized.Params),
		TechnicalDetail: normalized.TechnicalDetail,
	}
}
