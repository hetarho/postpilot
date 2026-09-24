package store

import (
	"context"
	"crypto/subtle"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

// Authenticate even a cancellation response, so another worker cannot inspect it.
func authenticatedMediaStage(ctx context.Context, q *sqlc.Queries, auth clip.MediaLeaseCredentials) (sqlc.ClipMediaStage, error) {
	r, err := q.GetMediaStage(ctx, auth.StageID)
	if err != nil {
		return r, leaseError(err)
	}
	a, err := q.GetMediaAttempt(ctx, auth.AttemptID)
	if err != nil {
		return r, leaseError(err)
	}
	if r.CurrentAttemptID.String != auth.AttemptID || a.StageID != auth.StageID || a.WorkerID != auth.WorkerID || subtle.ConstantTimeCompare([]byte(a.TokenHash), []byte(mediaTokenHash(auth.Token))) != 1 {
		return r, clip.ErrMediaLeaseLost
	}
	if r.State == string(clip.MediaCancelled) {
		return r, clip.ErrMediaCancelled
	}
	return r, nil
}

func (s *Store) FailMediaStage(ctx context.Context, auth clip.MediaLeaseCredentials, failure clip.MediaFailure, now time.Time) error {
	if now.IsZero() || !failure.Valid() {
		return clip.ErrInvalid
	}
	_, err := transact(ctx, s, func(q *sqlc.Queries) (bool, error) {
		r, err := authenticatedMediaStage(ctx, q, auth)
		if err != nil {
			return false, err
		}
		if r.State == string(clip.MediaFailed) {
			if r.Failure.String != string(failure) {
				return false, clip.ErrMediaConflict
			}
			return true, nil
		}
		_, err = q.FailMediaStage(ctx, sqlc.FailMediaStageParams{StageID: auth.StageID, AttemptID: nullable(auth.AttemptID), WorkerID: auth.WorkerID, TokenHash: mediaTokenHash(auth.Token), Failure: nullable(string(failure)), Now: stamp(now)})
		if err != nil {
			return false, leaseError(err)
		}
		return true, q.FailMediaAttempt(ctx, sqlc.FailMediaAttemptParams{FinishedAt: nullable(stamp(now)), ID: auth.AttemptID})
	})
	return err
}

func (s *Store) MediaRuntimeStatus(ctx context.Context, worker string, now time.Time) (clip.MediaRuntimeStatus, error) {
	// One short writer transaction supplies a consistent aggregate snapshot.
	return transact(ctx, s, func(q *sqlc.Queries) (clip.MediaRuntimeStatus, error) {
		var out clip.MediaRuntimeStatus
		var err error
		out.Waiting, err = q.MediaWaitingCount(ctx, stamp(now))
		if err != nil {
			return out, err
		}
		out.Active, err = q.MediaActiveCount(ctx, stamp(now))
		if err != nil {
			return out, err
		}
		out.OwnActive, err = q.MediaOwnActiveCount(ctx, sqlc.MediaOwnActiveCountParams{Now: stamp(now), WorkerID: worker})
		return out, err
	})
}

func (s *Store) MediaIncompatible(ctx context.Context, p clip.MediaWorkerProfile, now time.Time) (bool, error) {
	n, err := s.read.MediaIncompatibleCount(ctx, sqlc.MediaIncompatibleCountParams{Operation: string(p.Operation), ContractVersion: int64(p.ContractVersion), RendererVersion: p.RendererVersion, AssetVersion: p.AssetVersion, Now: stamp(now)})
	return n > 0, err
}
