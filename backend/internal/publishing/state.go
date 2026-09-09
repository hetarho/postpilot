package publishing

import "fmt"

// stageRank is the order PUB-13 fixes. It is the only place that order lives: the protobuf
// enum numbers append and the stage column stores names, so neither carries it.
var stageRank = map[Stage]int{
	StageQueued: 0, StageClaimed: 1, StagePreparing: 2, StageOpeningEditor: 3,
	StageFillingContent: 4, StageUploadingPhotos: 5, StageFillingSettings: 6,
	StageCommitting: 7, StageVerifying: 8, StagePublished: 9,
}

// ValidateProgress accepts an idempotent same-stage heartbeat or the next legal
// forward stage, but never a skip, regression, or post-commit reset.
func ValidateProgress(current, next Stage, currentSeq, nextSeq int64) error {
	if nextSeq <= currentSeq {
		return fmt.Errorf("%w: sequence %d is not newer than %d", ErrTransition, nextSeq, currentSeq)
	}
	from, ok := stageRank[current]
	if !ok {
		return fmt.Errorf("%w: unknown current stage %q", ErrTransition, current)
	}
	to, ok := stageRank[next]
	if !ok || to < from || to > from+1 || next == StageQueued || next == StagePublished {
		return fmt.Errorf("%w: %s -> %s", ErrTransition, current, next)
	}
	return nil
}

func FailureStatus(stage Stage, kind FailureKind) Status {
	if stageRank[stage] >= stageRank[StageCommitting] {
		return StatusOutcomeUnknown
	}
	switch kind {
	case FailureLoginExpired, FailureCaptcha, FailureTwoFactor, FailureAccountMismatch:
		return StatusNeedsAttention
	default:
		return StatusFailed
	}
}

func CanCancel(job Job) bool {
	return (job.Status == StatusQueued || job.Status == StatusRunning || job.Status == StatusNeedsAttention) &&
		job.CommittedAt == nil && stageRank[job.Stage] < stageRank[StageCommitting]
}
