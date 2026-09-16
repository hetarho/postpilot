package clip

import (
	"context"
	"time"
)

type mediaStageKey struct{}

// One record type for both halves of the same sink: a substage's own elapsed
// time and every media command that ran inside it, so a stage total is
// attributed to its operations instead of inferred by subtraction (CLIP-88).
type MediaRecord struct {
	Substage, Operation, Outcome string
	Elapsed                      time.Duration
}
type MediaStageObserver func(MediaRecord)

func WithMediaStageObserver(ctx context.Context, fn MediaStageObserver) context.Context {
	return context.WithValue(ctx, mediaStageKey{}, fn)
}
func reportMedia(ctx context.Context, r MediaRecord) {
	if fn, ok := ctx.Value(mediaStageKey{}).(MediaStageObserver); ok {
		fn(r)
	}
}
func ReportMediaStage(ctx context.Context, stage string, elapsed time.Duration) {
	reportMedia(ctx, MediaRecord{Substage: SafeAttemptCheck(stage), Elapsed: elapsed})
}

// Successful work is timed too, and by the same vocabulary the failure path
// already uses: a code-owned operation name, a code-owned outcome and a
// number. Arguments, filenames and stderr never reach this call (CLIP-88).
func ReportMediaOperation(ctx context.Context, operation, outcome string, elapsed time.Duration) {
	reportMedia(ctx, MediaRecord{Operation: SafeMediaOperation(operation), Outcome: SafeMediaOutcome(outcome), Elapsed: elapsed})
}
func SafeMediaOperation(operation string) string {
	switch operation {
	case "typeset", "probe", "loudness", "encode_final", "compose_audio", "correct_audio", "render_cut", "compose_video", "sample", "prepare", "decode":
		return operation
	}
	return "unknown"
}
func SafeMediaOutcome(outcome string) string {
	switch outcome {
	case "ok", "workspace_limit", "output_limit", "timeout", "canceled", "process_exit", "process_signal", "command_failed", "validation_failed":
		return outcome
	}
	return "unknown"
}
