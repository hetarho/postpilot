package store

import (
	"context"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
	"time"
)

func (s *Store) RequestClipCancellation(ctx context.Context, user, project, id string, at time.Time) error {
	_, err := s.write.RequestClipCancellation(ctx, sqlc.RequestClipCancellationParams{ID: id, UserID: user, ProjectID: nullString(project), Now: nullString(formatTime(at))})
	return err
}
func (s *Store) RecoverClipCancellations(ctx context.Context, at time.Time) (int64, error) {
	return s.write.RecoverClipCancellations(ctx, nullString(formatTime(at)))
}
func (s *Store) AuthorizeClipDispatch(ctx context.Context, user, id string) error {
	n, err := s.write.AuthorizeClipDispatch(ctx, sqlc.AuthorizeClipDispatchParams{ID: id, UserID: user})
	if err != nil {
		return err
	}
	if n != 1 {
		return job.ErrCreditAllowance
	}
	return nil
}
