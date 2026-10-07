package store

import (
	"context"
	"time"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
)

func (s *Store) GetEnqueueIdentity(ctx context.Context, id string) (job.EnqueueIdentity, error) {
	r, err := s.read.GetEnqueueReceipt(ctx, id)
	if err != nil {
		return job.EnqueueIdentity{}, mapNotFound(err, "get enqueue identity")
	}
	return job.EnqueueIdentity{JobID: r.JobID, UserID: r.UserID, Fingerprint: r.Fingerprint, Committed: r.State == "committed", Abandoned: r.State == "abandoned"}, nil
}

func (s *Store) InsertWithIdentity(ctx context.Context, found job.Job, identity job.EnqueueIdentity) error {
	if identity.JobID != found.ID || identity.UserID != found.UserID || identity.Fingerprint == "" {
		return job.ErrEnqueueIdentityConflict
	}
	_, err := withWriter(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		txStore := &Store{write: q, read: q, kinds: s.kinds}
		if err := txStore.Insert(ctx, found); err != nil {
			return struct{}{}, err
		}
		n, err := q.CommitEnqueueReceipt(ctx, sqlc.CommitEnqueueReceiptParams{JobID: identity.JobID, UserID: identity.UserID, Fingerprint: identity.Fingerprint})
		if err == nil && n != 1 {
			err = job.ErrEnqueueIdentityConflict
		}
		return struct{}{}, err

	})
	return err
}

func (s *Store) ClaimEnqueueIdentity(ctx context.Context, want job.EnqueueIdentity, at time.Time) error {
	_, err := withWriter(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		if err := q.InsertEnqueueReceipt(ctx, sqlc.InsertEnqueueReceiptParams{JobID: want.JobID, UserID: want.UserID, Fingerprint: want.Fingerprint, CreatedAt: formatTime(at)}); err != nil {
			return struct{}{}, err
		}
		got, err := q.GetEnqueueReceipt(ctx, want.JobID)
		if err != nil {
			return struct{}{}, err
		}
		if got.UserID != want.UserID || got.Fingerprint != want.Fingerprint || got.State == "abandoned" {
			return struct{}{}, job.ErrEnqueueIdentityConflict
		}
		return struct{}{}, nil
	})
	return err
}

func (s *Store) AbandonEnqueueIdentity(ctx context.Context, id string) (bool, error) {
	n, err := s.write.AbandonEnqueueReceipt(ctx, id)
	return n == 1, err
}
