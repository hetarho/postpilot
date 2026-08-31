package job

import (
	"context"
	"fmt"
)

// FailQueued is a compensating action for a caller that persisted its aggregate
// but could not link the newly queued job. It cannot stop work once a worker owns it.
func (q *Queue) FailQueued(ctx context.Context, id, userID string, failure Failure) (bool, error) {
	failed, err := q.store.FailQueued(ctx, id, userID, failure, q.now())
	if err != nil {
		return false, fmt.Errorf("fail queued job: %w", err)
	}
	return failed, nil
}
