package job

import (
	"context"
	"errors"
	"fmt"
)

// Get returns an ownership-checked public view.
func (q *Queue) Get(ctx context.Context, id, userID string) (*JobSummary, error) {
	found, err := q.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	if found.UserID != userID {
		return nil, ErrForbidden
	}
	return summarize(found), nil
}

// ActiveFor is the queued/running job a subject currently has, if any. The caller names
// the dimension it owns and narrows by user or kind when its rule does.
func (q *Queue) ActiveFor(ctx context.Context, subject Subject, filter Filter) (*JobSummary, error) {
	found, err := q.store.ActiveFor(ctx, subject, filter)
	if err != nil || found == nil {
		return nil, err
	}
	return summarize(*found), nil
}

// HasActiveFor answers the "is this busy" question a context asks before it deletes or
// replaces the subject: any queued/running job, whatever its kind.
func (q *Queue) HasActiveFor(ctx context.Context, subject Subject, filter Filter) (bool, error) {
	found, err := q.store.ActiveFor(ctx, subject, filter)
	return found != nil, err
}

// ActiveUnattached guards work that belongs to no subject: one per user and kind.
func (q *Queue) ActiveUnattached(ctx context.Context, userID, kind string) (*JobSummary, error) {
	found, err := q.store.ActiveUnattached(ctx, userID, kind)
	if err != nil || found == nil {
		return nil, err
	}
	return summarize(*found), nil
}

// LatestFor is the most recent job of a subject, terminal or not.
func (q *Queue) LatestFor(ctx context.Context, subject Subject, filter Filter) (*JobSummary, error) {
	found, err := q.store.LatestFor(ctx, subject, filter)
	if err != nil || found == nil {
		return nil, err
	}
	return summarize(*found), nil
}

// Snapshot is an owner-scoped read of one job including its durable payload, checked
// against the subject the caller expects it to belong to. The payload is never added to
// the public summary or the RPC projection.
func (q *Queue) Snapshot(ctx context.Context, userID string, subject Subject, id string) (*Job, error) {
	found, err := q.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if found.UserID != userID || !subject.valid() || found.Subject(subject.Dimension) != subject.ID {
		return nil, ErrNotFound
	}
	return &found, nil
}

// LatestSnapshot is the same owner-scoped read for the subject's most recent job, which is
// how a recovering owner finds the approval it left behind.
func (q *Queue) LatestSnapshot(ctx context.Context, userID string, subject Subject) (*Job, error) {
	found, err := q.store.LatestFor(ctx, subject, Filter{UserID: userID})
	if err != nil || found == nil {
		return nil, err
	}
	if found.UserID != userID || found.Subject(subject.Dimension) != subject.ID {
		return nil, ErrNotFound
	}
	return found, nil
}

// Activate releases a job whose dispatch was deferred until its owner approved the work.
func (q *Queue) Activate(ctx context.Context, userID, id string) error {
	released, err := q.store.Activate(ctx, userID, id)
	if err != nil {
		return err
	}
	if !released {
		return ErrNotFound
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return nil
}

// SweepUnactivated runs at boot only, before any worker starts: a job still waiting for
// its activation has made no model call, so failing it costs nothing.
func (q *Queue) SweepUnactivated(ctx context.Context) (int64, error) {
	return q.store.SweepUnactivated(ctx, interruptedFailure)
}
