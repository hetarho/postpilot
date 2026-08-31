package job

import "errors"

var (
	errHandlerPanicked = errors.New("job handler panicked")
	errHandlerMissing  = errors.New("job handler missing")
)

var interruptedFailure = Failure{Reason: FailureReasonInterrupted}
