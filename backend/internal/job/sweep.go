package job

import (
	"context"
	"fmt"
)

func (q *Queue) SweepRunning(ctx context.Context) (int64, error) {
	n, err := q.store.SweepRunning(ctx, RestartMessage, q.now())
	if err != nil {
		return 0, fmt.Errorf("sweep running jobs: %w", err)
	}
	return n, nil
}

// SweepQueuedPersonalization prevents boot's worker drain from becoming the user
// action that starts a provider call.
func (q *Queue) SweepQueuedPersonalization(ctx context.Context) (int64, error) {
	n, err := q.store.SweepQueuedPersonalization(ctx, PersonalizationRestartMessage, q.now())
	if err != nil {
		return 0, fmt.Errorf("sweep queued personalization jobs: %w", err)
	}
	return n, nil
}
