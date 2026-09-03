package job

import (
	"context"
	"errors"
	"fmt"
)

func (q *Queue) SweepRunning(ctx context.Context) (int64, error) {
	n, err := q.store.SweepRunning(ctx, interruptedFailure, q.now())
	if err != nil {
		return 0, fmt.Errorf("sweep running jobs: %w", err)
	}
	return n, nil
}

// SweepQueuedPersonalization prevents boot's worker drain from becoming the user
// action that starts a provider call.
func (q *Queue) SweepQueuedPersonalization(ctx context.Context) (int64, error) {
	n, err := q.store.SweepQueuedPersonalization(ctx, interruptedFailure, q.now())
	if err != nil {
		return 0, fmt.Errorf("sweep queued personalization jobs: %w", err)
	}
	return n, nil
}

// SweepOpenHolds settles every hold left open behind a job that has already finished.
//
// The gap it closes is a crash between the terminal write and the settle that follows it:
// the job is done, its ledger rows are written, but the credits it did not use are still
// reserved. A hold behind a job that is still queued or running is left alone — that one
// is doing its job.
func (q *Queue) SweepOpenHolds(ctx context.Context) (int, error) {
	if q.admitter == nil {
		return 0, nil
	}
	ids, err := q.admitter.OpenHolds(ctx)
	if err != nil {
		return 0, fmt.Errorf("list open holds: %w", err)
	}
	settled := 0
	for _, id := range ids {
		found, err := q.store.GetByID(ctx, id)
		if errors.Is(err, ErrNotFound) {
			// The hold outlived its job row: an insert that failed after the hold was taken
			// and whose release did not land. Returning it is exactly what release would have.
			q.admitter.Release(ctx, id)
			settled++
			continue
		}
		if err != nil {
			return settled, fmt.Errorf("read job %s: %w", id, err)
		}
		if found.Status != StatusDone && found.Status != StatusFailed {
			continue
		}
		q.admitter.Settle(ctx, id)
		settled++
	}
	return settled, nil
}
