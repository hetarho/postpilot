package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func (s *Store) MediaRecoveryStages(ctx context.Context, after string) ([]clip.MediaStage, error) {
	rows, err := s.read.MediaRecoveryStages(ctx, after)
	if err != nil {
		return nil, err
	}
	out := make([]clip.MediaStage, 0, len(rows))
	for _, row := range rows {
		stage, err := mediaStageRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, stage)
	}
	return out, nil
}

func (s *Store) MediaRecoveryState(ctx context.Context, id string) (clip.MediaRecovery, error) {
	stage, err := s.GetMediaStage(ctx, id)
	out := clip.MediaRecovery{Stage: stage}
	if err != nil || stage.CurrentAttemptID == "" {
		return out, err
	}
	a, err := s.read.GetMediaAttempt(ctx, stage.CurrentAttemptID)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, dbError(err)
	}
	out.Outcome = a.Outcome.String
	out.LeaseExpiresAt, err = time.Parse(time.RFC3339Nano, a.LeaseExpiresAt)
	return out, err
}

func (s *Store) SetMediaRecoveryState(ctx context.Context, id string, state clip.MediaStageState, failure clip.MediaFailure, retry time.Time) error {
	var at string
	if !retry.IsZero() {
		at = stamp(retry)
	}
	return s.write.SetMediaRecoveryState(ctx, sqlc.SetMediaRecoveryStateParams{ID: id, State: string(state), Failure: nullable(string(failure)), RetryNotBefore: nullable(at)})
}
func (s *Store) StopMediaAttempt(ctx context.Context, id, outcome string, now time.Time) error {
	return s.write.StopMediaAttempt(ctx, sqlc.StopMediaAttemptParams{ID: id, Outcome: nullable(outcome), Now: nullable(stamp(now))})
}
func (s *Store) RetireMediaArtifacts(ctx context.Context, id, current string, all bool) error {
	var allAttempts int64
	if all {
		allAttempts = 1
	}
	return s.write.RetireMediaStageArtifacts(ctx, sqlc.RetireMediaStageArtifactsParams{StageID: id, CurrentAttemptID: current, AllAttempts: allAttempts})
}
func (s *Store) MarkMediaReconciled(ctx context.Context, id string, now time.Time) error {
	return s.write.MarkMediaReconciled(ctx, sqlc.MarkMediaReconciledParams{ID: id, ReconciledAt: nullable(stamp(now))})
}
func (s *Store) RetireSupersededMediaResults(ctx context.Context) error {
	return s.write.RetireSupersededMediaResults(ctx)
}
func (s *Store) DueMediaDeletions(ctx context.Context, cutoff time.Time) ([]clip.MediaDeletion, error) {
	rows, err := s.read.DueMediaDeletions(ctx, stamp(cutoff))
	if err != nil {
		return nil, err
	}
	out := make([]clip.MediaDeletion, 0, len(rows))
	for _, r := range rows {
		out = append(out, clip.MediaDeletion{Key: r.ObjectKey})
	}
	return out, nil
}
func (s *Store) RemoveMediaDeletion(ctx context.Context, key string) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (bool, error) {
		if err := q.RemoveDeletion(ctx, key); err != nil {
			return false, err
		}
		return true, q.RemoveMediaDeletion(ctx, key)
	})
	return err
}
func (s *Store) QueueOrphanMediaDeletion(ctx context.Context, key string, now time.Time) error {
	return s.write.QueueOrphanMediaDeletion(ctx, sqlc.QueueOrphanMediaDeletionParams{ObjectKey: key, Now: stamp(now)})
}
