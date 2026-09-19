package job

import "errors"

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

// Reporting is how the contexts whose work the queue runs describe that work: the
// durable failure an error becomes, the provider detail that may be logged beside it, and
// which stage names are safe to log for a kind. The queue owns three reasons of its own
// (interrupted, panicked, handler missing) and no vocabulary beyond them — a provider's
// error classes and a product's stage names are not the queue's to know.
//
// A queue built without one is a legal, tested mode: every other failure reports
// UNKNOWN_FAILURE and no stage line is written.
type Reporting interface {
	Failure(err error) Failure
	LogAttrs(err error) []any
	// SafeStage returns the stage name that may be logged for this kind, and false when
	// this kind is not stage-logged at all.
	SafeStage(kind, stage string) (string, bool)
	// Redacted marks a kind whose errors and panics may wrap user content, provider
	// bodies or file paths: the queue then logs only normalized metadata about them.
	Redacted(kind string) bool
}

func (q *Queue) failureFromError(err error) Failure {
	switch {
	case errors.Is(err, errHandlerPanicked):
		return Failure{Reason: FailureReasonPanicked}
	case errors.Is(err, errHandlerMissing):
		return Failure{Reason: FailureReasonHandlerMissing}
	}
	if q == nil || q.reporting == nil {
		return Failure{Reason: FailureReasonUnknown}
	}
	failure := q.reporting.Failure(err)
	if failure.Reason == "" {
		failure.Reason = FailureReasonUnknown
	}
	failure.Params = cloneParams(failure.Params)
	return failure
}
