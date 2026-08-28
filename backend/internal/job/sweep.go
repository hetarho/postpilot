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
