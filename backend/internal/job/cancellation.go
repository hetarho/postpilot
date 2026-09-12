package job

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var ErrCancellationUnavailable = errors.New("clip cancellation policy was not approved")

type ClipCancellationStore interface {
	RequestClipCancellation(context.Context, string, string, string, time.Time) error
	RecoverClipCancellations(context.Context, time.Time) (int64, error)
}

// CancelClipJob first commits the request, then signals its local handler. Queued
// work has no handler, so its conditional request write is also the terminal write.
func (q *Queue) CancelClipJob(ctx context.Context, user, project, id string) (*JobSummary, error) {
	s, ok := q.store.(ClipCancellationStore)
	if !ok {
		return nil, ErrCancellationUnavailable
	}
	j, err := q.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.UserID != user || j.ClipProjectID != project || project == "" || (j.Kind != KindGenerateClip && j.Kind != KindRenderClip) {
		return nil, ErrNotFound
	}
	if Terminal(j.Status) {
		return summarize(j), nil
	}
	if j.Kind == KindGenerateClip && (j.CancellationPolicyVersion != 1 || q.clipGuard == nil) {
		return nil, ErrCancellationUnavailable
	}
	requestErr := s.RequestClipCancellation(ctx, user, project, id, q.now())
	// A lost response is not proof of rollback. Resolve the durable request before
	// signalling or reporting an accepted request to the caller.
	readCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer stop()
	j, err = q.store.GetByID(readCtx, id)
	if err != nil {
		return nil, errors.Join(requestErr, err)
	}
	if j.CancelRequestedAt == nil {
		if Terminal(j.Status) {
			return summarize(j), nil
		}
		if requestErr != nil {
			return nil, requestErr
		}
		return nil, ErrCancellationUnavailable
	}
	q.mu.RLock()
	cancel := q.running[id]
	q.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	if j.Status == StatusCancelled {
		if q.admitter != nil && j.Kind != KindRenderClip {
			q.admitter.Settle(readCtx, id, j.Status)
		}
		if j.FinishedAt != nil {
			if err := q.notifyTerminal(readCtx, j, *j.FinishedAt); err != nil {
				slog.Warn("cancelled clip resource release pending recovery", "job", id)
			}
		}
	}
	return summarize(j), nil
}

func (q *Queue) registerClipExecution(ctx context.Context, j Job) (context.Context, func(), error) {
	runCtx, cancel := context.WithCancel(ctx)
	q.mu.Lock()
	q.running[j.ID] = cancel
	q.mu.Unlock()
	cleanup := func() { q.mu.Lock(); delete(q.running, j.ID); q.mu.Unlock(); cancel() }
	// Registration precedes the read, closing the request-before-registration race.
	current, err := q.store.GetByID(ctx, j.ID)
	if err != nil {
		return runCtx, cleanup, err
	}
	if current.CancelRequestedAt != nil || Terminal(current.Status) {
		cancel()
		return runCtx, cleanup, context.Canceled
	}
	return runCtx, cleanup, nil
}
