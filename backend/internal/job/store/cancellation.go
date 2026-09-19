package store

import (
	"context"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
	"time"
)

func (s *Store) RequestCancellation(ctx context.Context, user, subject, id string, at time.Time) error {
	kinds, err := kindsJSON(s.kinds.Cancellable)
	if err != nil {
		return err
	}
	_, err = s.write.RequestCancellation(ctx, sqlc.RequestCancellationParams{ID: id, UserID: user, ProjectID: nullString(subject), Now: nullString(formatTime(at)), Kinds: kinds})
	return err
}

func (s *Store) RecoverCancellations(ctx context.Context, at time.Time) (int64, error) {
	kinds, err := kindsJSON(s.kinds.Cancellable)
	if err != nil {
		return 0, err
	}
	return s.write.RecoverCancellations(ctx, sqlc.RecoverCancellationsParams{Now: nullString(formatTime(at)), Kinds: kinds})
}

// AuthorizeDispatch is the conditional writer statement that serializes one model
// call against a cancellation request: a refused row means the owner got there first.
func (s *Store) AuthorizeDispatch(ctx context.Context, user, id string) error {
	kinds, err := kindsJSON(s.kinds.Authorized)
	if err != nil {
		return err
	}
	n, err := s.write.AuthorizeDispatch(ctx, sqlc.AuthorizeDispatchParams{ID: id, UserID: user, Kinds: kinds})
	if err != nil {
		return err
	}
	if n != 1 {
		return job.ErrDispatchRefused
	}
	return nil
}
