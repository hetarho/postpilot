package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
)

// These primitives compose with the caller's reservations on NewTx. The caller
// supplies the kind and bounds; the queue owns no product policy.
func (s *Store) LockAdmission(ctx context.Context) error { return s.write.LockAdmission(ctx) }
func (s *Store) ActiveCount(ctx context.Context, f job.Filter) (int, error) {
	if f.Kind == "" {
		return 0, job.ErrInvalidTarget
	}
	n, err := s.read.ActiveCount(ctx, sqlc.ActiveCountParams{Kind: f.Kind, UserID: f.UserID})
	return int(n), err
}
func (s *Store) SetWaitExpiry(ctx context.Context, id, stage string, expires time.Time) error {
	n, err := s.write.SetWaitExpiry(ctx, sqlc.SetWaitExpiryParams{ID: id, Stage: nullString(stage), ExpiresAt: nullString(formatTime(expires))})
	if err != nil {
		return err
	}
	if n != 1 {
		return job.ErrNotFound
	}
	return nil
}
