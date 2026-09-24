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
	r, err := mediaCredentialStage(ctx, q, auth)
	if err == nil && r.State == string(clip.MediaCancelled) {
		return r, clip.ErrMediaCancelled
	}
	return r, err
}

func mediaCredentialStage(ctx context.Context, q *sqlc.Queries, auth clip.MediaLeaseCredentials) (sqlc.ClipMediaStage, error) {
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
		if failure == clip.MediaFailureCancelled {
			return false, clip.ErrInvalid
		}
		if r.State == string(clip.MediaQueued) && storedMediaFailure(r) == failure {
			return true, nil // lost retry reply must not extend its backoff
		}
		if r.State == string(clip.MediaFailed) {
			if failure == clip.MediaFailureWorkerLost && r.Failure.String == string(clip.MediaFailureAttemptsExhausted) {
				return true, nil
			}
			if storedMediaFailure(r) != failure {
				return false, clip.ErrMediaConflict
			}
			return true, nil
		}
		family, detail := failure, ""
		switch failure {
		case clip.MediaFailureWorkspaceLimit, clip.MediaFailureInputTooLarge, clip.MediaFailureAnalysisTooLarge:
			family, detail = clip.MediaFailureInvalidInput, string(failure)
		}
		_, err = q.FailMediaStage(ctx, sqlc.FailMediaStageParams{StageID: auth.StageID, AttemptID: nullable(auth.AttemptID), WorkerID: auth.WorkerID, TokenHash: mediaTokenHash(auth.Token), Failure: nullable(string(family)), FailureDetail: nullable(detail), Now: stamp(now)})
		if err != nil {
			return false, leaseError(err)
		}
		if err = q.FailMediaAttempt(ctx, sqlc.FailMediaAttemptParams{FinishedAt: nullable(stamp(now)), ID: auth.AttemptID}); err != nil {
			return false, err
		}
		if failure == clip.MediaFailureWorkerLost {
			state, why, retry := clip.MediaQueued, failure, now.Add(clip.MediaRetryDelay(int(r.AttemptCount)))
			if r.AttemptCount >= r.AttemptLimit {
				state, why, retry = clip.MediaFailed, clip.MediaFailureAttemptsExhausted, time.Time{}
			}
			return true, (&Store{read: q, write: q}).SetMediaRecoveryState(ctx, r.ID, state, why, retry)
		}
		return true, nil
	})
	return err
}

// The application calls this only after the owning cancellation was accepted.
// A stopped worker may acknowledge even after the reconciler fenced its stage.
func (s *Store) AcknowledgeMediaStop(ctx context.Context, auth clip.MediaLeaseCredentials, now time.Time) error {
	_, err := transact(ctx, s, func(q *sqlc.Queries) (bool, error) {
		r, err := mediaCredentialStage(ctx, q, auth)
		if err != nil {
			return false, err
		}
		if err = (&Store{read: q, write: q}).SetMediaRecoveryState(ctx, r.ID, clip.MediaCancelled, "", time.Time{}); err != nil {
			return false, err
		}
		return true, q.StopMediaAttempt(ctx, sqlc.StopMediaAttemptParams{ID: auth.AttemptID, Outcome: nullable("cancelled"), Now: nullable(stamp(now))})
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

func storedMediaFailure(r sqlc.ClipMediaStage) clip.MediaFailure {
	if r.FailureDetail.Valid {
		return clip.MediaFailure(r.FailureDetail.String)
	}
	return clip.MediaFailure(r.Failure.String)
}
