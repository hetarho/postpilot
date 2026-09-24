package job

import (
	"context"
	"errors"
	"time"
)

var (
	ErrYield       = errors.New("job yielded to a durable external wait")
	ErrInvalidWait = errors.New("job has no matching durable external wait")
)

const ContinuationKeyMaxBytes = 256

type ResumePolicy string

const (
	FailOnInterrupt ResumePolicy = "fail_on_interrupt"
	ReplaySafe      ResumePolicy = "replay_safe"
)

type ContinuationState string

const (
	ContinuationWaiting ContinuationState = "waiting"
	ContinuationReady   ContinuationState = "ready"
	ContinuationClaimed ContinuationState = "claimed"
)

type Continuation struct {
	JobID, WaitKey string
	State          ContinuationState
	Policy         ResumePolicy
}

// ContinuationStore is optional for a queue that never dispatches external work.
// Store.NewTx exposes the same atomic park/wake primitives to an owning saga.
type ContinuationStore interface {
	Park(context.Context, string, string, ResumePolicy, time.Time) error
	Wake(context.Context, string, string, time.Time) (bool, error)
	HasPendingWait(context.Context, string) (bool, error)
	AcknowledgeWaitCancellation(context.Context, string, string, time.Time) (bool, error)
}

// ExternalRecoveryStore exposes opaque waits to their owner without learning
// anything about the service responsible for the external work.
type ExternalRecoveryStore interface {
	WaitingContinuations(context.Context, string, string) ([]Continuation, error)
	FailWaitingContinuation(context.Context, string, string, Failure, time.Time) (bool, error)
}

func (q *Queue) FailWait(ctx context.Context, id, key string, failure Failure) (bool, error) {
	s, ok := q.store.(ExternalRecoveryStore)
	if !ok {
		return false, ErrInvalidWait
	}
	changed, err := s.FailWaitingContinuation(ctx, id, key, failure, q.now())
	if err != nil || !changed {
		return changed, err
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	j, err := q.store.GetByID(finishCtx, id)
	if err != nil {
		return true, err
	}
	if q.admitter != nil {
		q.admitter.Settle(finishCtx, id, j.Status)
	}
	if j.FinishedAt != nil {
		err = q.notifyTerminal(finishCtx, j, *j.FinishedAt)
	}
	return true, err
}

func (q *Queue) Park(ctx context.Context, id, key string, policy ResumePolicy) error {
	s, ok := q.store.(ContinuationStore)
	if !ok {
		return ErrInvalidWait
	}
	return s.Park(ctx, id, key, policy, q.now())
}

func (q *Queue) Wake(ctx context.Context, id, key string) (bool, error) {
	s, ok := q.store.(ContinuationStore)
	if !ok {
		return false, ErrInvalidWait
	}
	changed, err := s.Wake(ctx, id, key, q.now())
	if changed && err == nil {
		select {
		case q.wake <- struct{}{}:
		default:
		}
	}
	return changed, err
}

// AcknowledgeWaitCancellation is called only after the owning workflow has
// established that external work stopped. A duplicate does not repeat hooks.
func (q *Queue) AcknowledgeWaitCancellation(ctx context.Context, id, key string) (bool, error) {
	s, ok := q.store.(ContinuationStore)
	if !ok {
		return false, ErrInvalidWait
	}
	changed, err := s.AcknowledgeWaitCancellation(ctx, id, key, q.now())
	if err != nil || !changed {
		return changed, err
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishTimeout)
	defer cancel()
	j, err := q.store.GetByID(finishCtx, id)
	if err != nil {
		return true, err
	}
	if q.admitter != nil {
		q.admitter.Settle(finishCtx, id, j.Status)
	}
	if j.FinishedAt != nil {
		err = q.notifyTerminal(finishCtx, j, *j.FinishedAt)
	}
	return true, err
}
