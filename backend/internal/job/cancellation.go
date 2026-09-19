package job

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

var ErrCancellationUnavailable = errors.New("job cancellation was not approved")

// Cancellation is the root's answer to what an owner may stop. Kind reports whether a
// kind takes part in cancellation at all — anything else is simply not found — and
// Allowed decides one particular job, which is where an approved policy version is read.
type Cancellation interface {
	Kind(kind string) bool
	Allowed(kind string, cancellationPolicyVersion int) bool
}

// CancellationStore is the conditional writer statement that makes a request durable
// before anyone is told it was accepted.
type CancellationStore interface {
	RequestCancellation(ctx context.Context, user, subject, id string, now time.Time) error
	RecoverCancellations(ctx context.Context, now time.Time) (int64, error)
}

// Cancel first commits the request, then signals its local handler. Queued work has no
// handler, so its conditional request write is also the terminal write.
func (q *Queue) Cancel(ctx context.Context, user string, subject Subject, id string) (*JobSummary, error) {
	s, ok := q.store.(CancellationStore)
	if !ok || q.cancellation == nil {
		return nil, ErrCancellationUnavailable
	}
	j, err := q.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if j.UserID != user || !subject.valid() || j.Subject(subject.Dimension) != subject.ID || !q.cancellation.Kind(j.Kind) {
		return nil, ErrNotFound
	}
	if Terminal(j.Status) {
		return q.summarize(j), nil
	}
	if !q.cancellation.Allowed(j.Kind, j.CancellationPolicyVersion) {
		return nil, ErrCancellationUnavailable
	}
	requestErr := s.RequestCancellation(ctx, user, subject.ID, id, q.now())
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
			return q.summarize(j), nil
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
		// A job that took no hold settles to nothing: the ledger writes nothing when it
		// finds no open admission.
		if q.admitter != nil {
			q.admitter.Settle(readCtx, id, j.Status)
		}
		if j.FinishedAt != nil {
			if err := q.notifyTerminal(readCtx, j, *j.FinishedAt); err != nil {
				slog.Warn("cancelled clip resource release pending recovery", "job", id)
			}
		}
	}
	return q.summarize(j), nil
}

// cancellable reports whether this job's run must be registered so a cancellation
// request can reach it.
func (q *Queue) cancellable(j Job) bool {
	return q.cancellation != nil && q.cancellation.Kind(j.Kind)
}

func (q *Queue) registerCancellableExecution(ctx context.Context, j Job) (context.Context, func(), error) {
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
