package clip

import (
	"context"
	"time"
)

type mediaStageKey struct{}
type MediaStageObserver func(string, time.Duration)

func WithMediaStageObserver(ctx context.Context, fn MediaStageObserver) context.Context {
	return context.WithValue(ctx, mediaStageKey{}, fn)
}
func ReportMediaStage(ctx context.Context, stage string, elapsed time.Duration) {
	if fn, ok := ctx.Value(mediaStageKey{}).(MediaStageObserver); ok {
		fn(SafeAttemptCheck(stage), elapsed)
	}
}
