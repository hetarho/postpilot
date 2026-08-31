package experiment

import (
	"errors"

	"github.com/postpilot/backend/internal/llm"
)

const (
	FailureReasonInterrupted         = "JOB_INTERRUPTED"
	FailureReasonSnapshotUnavailable = "EXPERIMENT_SNAPSHOT_UNAVAILABLE"
	FailureReasonLanguageRequired    = "POST_TARGET_LANGUAGE_REQUIRED"
	FailureReasonUnknown             = "UNKNOWN_FAILURE"
)

// Failure is the experiment context's durable failure projection. Candidate provider
// prose may be retained only in TechnicalDetail; Params is limited to display-safe
// interpolation values.
type Failure struct {
	Reason          string
	Params          map[string]string
	TechnicalDetail string
}

func normalizeFailure(err error) Failure {
	switch {
	case errors.Is(err, ErrSnapshotUnavailable):
		return Failure{Reason: FailureReasonSnapshotUnavailable}
	case errors.Is(err, ErrLanguageRequired):
		return Failure{Reason: FailureReasonLanguageRequired}
	}
	normalized := llm.NormalizeFailure(err)
	return Failure{
		Reason:          normalized.Reason,
		Params:          cloneFailureParams(normalized.Params),
		TechnicalDetail: normalized.TechnicalDetail,
	}
}

func cloneFailure(value *Failure) *Failure {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Params = cloneFailureParams(value.Params)
	return &copy
}

func cloneFailureParams(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

var interruptedFailure = Failure{Reason: FailureReasonInterrupted}

var errAllCandidatesFailed = errors.New("all experiment candidates failed")
