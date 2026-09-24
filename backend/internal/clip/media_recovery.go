package clip

import "time"

// Recovery metadata never includes a credential or private object URL.
type MediaRecovery struct {
	Stage          MediaStage
	LeaseExpiresAt time.Time
	Outcome        string
}

func (r MediaRecovery) Stopped(now time.Time) bool {
	return r.Stage.CurrentAttemptID == "" || r.Outcome != "" || !r.LeaseExpiresAt.After(now)
}

type MediaDeletion struct{ Key string }

func MediaRetryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 5 * time.Second
	}
	return 15 * time.Second
}

// A media stage never returns raw worker errors to the owner.
type MediaStageFailure struct{ Code MediaFailure }

func (e *MediaStageFailure) Error() string { return "media stage: " + string(e.Code) }
func (e *MediaStageFailure) Unwrap() error {
	switch e.Code {
	case MediaFailureWaitExpired:
		return ErrMediaUnavailable
	case MediaFailureInvalidInput, MediaFailureInvalidOutput:
		return ErrInvalidMedia
	}
	return nil
}
