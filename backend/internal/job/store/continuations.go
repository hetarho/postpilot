package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/job/store/sqlc"
)

func withWriter[T any](ctx context.Context, s *Store, fn func(*sqlc.Queries) (T, error)) (T, error) {
	if s.writer == nil {
		return fn(s.write)
	}
	var zero T
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	value, err := fn(s.write.WithTx(tx))
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return value, nil
}

func (s *Store) Park(ctx context.Context, id, key string, policy job.ResumePolicy, now time.Time) error {
	if policy == "" {
		policy = job.FailOnInterrupt
	}
	if now.IsZero() || strings.TrimSpace(key) != key || key == "" || len(key) > job.ContinuationKeyMaxBytes || (policy != job.FailOnInterrupt && policy != job.ReplaySafe) {
		return job.ErrInvalidWait
	}
	_, err := withWriter(ctx, s, func(q *sqlc.Queries) (struct{}, error) {
		j, err := q.GetJobByID(ctx, id)
		if err != nil {
			return struct{}{}, mapNotFound(err, "park job")
		}
		if j.Status != job.StatusRunning || j.CancelRequestedAt.Valid || j.DispatchReady != 1 {
			return struct{}{}, job.ErrDispatchRefused
		}
		old, err := q.GetContinuation(ctx, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return struct{}{}, err
		}
		if err == nil && old.WaitKey == key {
			if old.ResumePolicy == string(policy) && (old.State == string(job.ContinuationWaiting) || old.State == string(job.ContinuationReady)) {
				return struct{}{}, nil
			}
			return struct{}{}, job.ErrInvalidWait
		}
		n, err := q.ParkJob(ctx, sqlc.ParkJobParams{WaitKey: key, ResumePolicy: string(policy), Now: formatTime(now), JobID: id})
		if err == nil && n != 1 {
			err = job.ErrInvalidWait
		}
		return struct{}{}, err
	})
	return err
}

func (s *Store) Wake(ctx context.Context, id, key string, now time.Time) (bool, error) {
	n, err := s.write.WakeContinuation(ctx, sqlc.WakeContinuationParams{Now: nullString(formatTime(now)), JobID: id, WaitKey: key})
	return n == 1, err
}
func (s *Store) HasPendingWait(ctx context.Context, id string) (bool, error) {
	n, err := s.read.HasPendingWait(ctx, id)
	return n, err
}
func (s *Store) AcknowledgeWaitCancellation(ctx context.Context, id, key string, now time.Time) (bool, error) {
	n, err := s.write.AcknowledgeWaitCancellation(ctx, sqlc.AcknowledgeWaitCancellationParams{Now: nullString(formatTime(now)), JobID: id, WaitKey: key})
	return n == 1, err
}

// Continuation describes the durable handoff to its owning workflow without
// teaching the queue about any product's payload or artifact state.
func (s *Store) Continuation(ctx context.Context, id string) (job.Continuation, error) {
	c, err := s.read.GetContinuation(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return job.Continuation{}, job.ErrInvalidWait
	}
	if err != nil {
		return job.Continuation{}, err
	}
	return job.Continuation{JobID: c.JobID, WaitKey: c.WaitKey, State: job.ContinuationState(c.State), Policy: job.ResumePolicy(c.ResumePolicy)}, nil
}

func (s *Store) WaitingContinuations(ctx context.Context, prefix, after string) ([]job.Continuation, error) {
	rows, err := s.read.WaitingContinuations(ctx, sqlc.WaitingContinuationsParams{Prefix: prefix, AfterID: after})
	if err != nil {
		return nil, err
	}
	out := make([]job.Continuation, 0, len(rows))
	for _, c := range rows {
		out = append(out, job.Continuation{JobID: c.JobID, WaitKey: c.WaitKey, State: job.ContinuationState(c.State), Policy: job.ResumePolicy(c.ResumePolicy)})
	}
	return out, nil
}
func (s *Store) FailWaitingContinuation(ctx context.Context, id, key string, failure job.Failure, now time.Time) (bool, error) {
	reason, params, _, err := failureColumns(&failure)
	if err != nil {
		return false, err
	}
	n, err := s.write.FailWaitingContinuation(ctx, sqlc.FailWaitingContinuationParams{JobID: id, WaitKey: key, Reason: reason, Params: params, Now: nullString(formatTime(now))})
	return n == 1, err
}
