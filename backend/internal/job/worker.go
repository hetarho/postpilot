package job

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// A terminal write is one SQLite update. It gets a fresh, bounded context so a
// shutdown arriving after the handler has completed cannot relabel completed work as
// an interrupted job on the next boot.
const finishTimeout = 5 * time.Second

// Run consumes queued rows until its context is cancelled. One Run call is one worker.
func (q *Queue) Run(ctx context.Context) {
	ticker := time.NewTicker(q.pollInterval)
	defer ticker.Stop()

	q.drain(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
			q.drain(ctx)
		case <-ticker.C:
			q.drain(ctx)
		}
	}
}

func (q *Queue) drain(ctx context.Context) {
	for ctx.Err() == nil {
		found, err := q.store.PickNextQueued(ctx, q.now())
		if errors.Is(err, ErrNotFound) {
			return
		}
		if err != nil {
			slog.Error("pick queued job failed", "err", err)
			return
		}
		q.run(ctx, found)
	}
}

func (q *Queue) run(ctx context.Context, found Job) {
	handler := q.handler(found.Kind)
	var runErr error
	if handler == nil {
		runErr = errHandlerMissing
	} else {
		runErr = callHandler(ctx, handler, found, func(stage string, done, total int) {
			if err := q.store.UpdateProgress(ctx, found.ID, stage, done, total, q.now()); err != nil && ctx.Err() == nil {
				slog.Error("update job progress failed", "job", found.ID, "err", err)
			}
		})
	}

	// A handler that observed the worker cancellation did not complete and deliberately
	// leaves the row running for the next boot sweep. A successful handler (or an
	// ordinary failure) has reached a terminal result even if shutdown raced its return,
	// so that result must still be committed.
	if ctx.Err() != nil && errors.Is(runErr, ctx.Err()) {
		return
	}

	status := StatusDone
	var failure *Failure
	if runErr != nil {
		status = StatusFailed
		normalized := failureFromError(runErr)
		failure = &normalized
		logJobFailure(found, normalized, runErr)
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	if err := q.store.Finish(finishCtx, found.ID, status, failure, q.now()); err != nil {
		slog.Error("finish job failed", "job", found.ID, "status", status, "err", err)
		// The write may have committed despite returning an error. Only a persisted
		// terminal outcome can authorize settlement; otherwise recovery owns the hold.
		persisted, readErr := q.store.GetByID(finishCtx, found.ID)
		if readErr != nil || (persisted.Status != StatusDone && persisted.Status != StatusFailed) {
			return
		}
		status = persisted.Status
	}
	// Settling after the terminal write, on the same detached context: the job's ledger
	// rows are all written by now, so this is the first moment the hold can be reconciled
	// against what the work actually cost. A failure here strands credits until the boot
	// sweep, which is why it must not also fail the job.
	if q.admitter != nil && found.Kind != KindRenderClip {
		q.admitter.Settle(finishCtx, found.ID, status)
	}
}

func callHandler(ctx context.Context, handler Handler, found Job, progress Progress) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			attrs := []any{"job", found.ID, "kind", found.Kind}
			if found.Kind != KindGenerateClip && found.Kind != KindRenderClip {
				attrs = append(attrs, "panic", recovered)
			}
			slog.Error("job handler panicked", attrs...)
			err = errHandlerPanicked
		}
	}()
	return handler(ctx, found, progress)
}

// Clip failures may wrap subprocess stderr, media paths or provider bodies. Log
// only the normalized reason even if an unexpected handler bypasses StageFailure.
func logJobFailure(found Job, failure Failure, err error) {
	attrs := []any{"job", found.ID, "kind", found.Kind, "reason", failure.Reason}
	if found.Kind != KindGenerateClip && found.Kind != KindRenderClip {
		attrs = append(attrs, "err", err)
	}
	slog.Error("job failed", attrs...)
}
