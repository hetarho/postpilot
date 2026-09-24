package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func mediaStageRow(r sqlc.ClipMediaStage) (clip.MediaStage, error) {
	created, err := time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return clip.MediaStage{}, err
	}
	wait, err := time.Parse(time.RFC3339Nano, r.QueueDeadlineAt)
	if err != nil {
		return clip.MediaStage{}, err
	}
	deadline, err := time.Parse(time.RFC3339Nano, r.DeadlineAt)
	if err != nil {
		return clip.MediaStage{}, err
	}
	return clip.MediaStage{
		MediaStageInput: clip.MediaStageInput{ID: r.ID, ParentJobID: r.ParentJobID, UserID: r.UserID, ProjectID: r.ProjectID,
			ExpectedRevision: int(r.ExpectedRevision), Operation: clip.MediaOperation(r.Operation), ContractVersion: int(r.ContractVersion),
			InputDigest: r.InputDigest, Payload: r.InputPayload, RendererVersion: r.RendererVersion, AssetVersion: r.AssetVersion,
			Limits: clip.MediaStageLimits{LeaseTTL: time.Duration(r.LeaseTtlNs), WaitTimeout: wait.Sub(created), StageTimeout: deadline.Sub(created), MaxAttempts: int(r.AttemptLimit)}},
		State: clip.MediaStageState(r.State), CurrentAttemptID: r.CurrentAttemptID.String, AttemptCount: int(r.AttemptCount),
		CreatedAt: created, QueueDeadlineAt: wait, DeadlineAt: deadline, AcceptedResult: r.AcceptedResult.String, Failure: clip.MediaFailure(r.Failure.String),
	}, nil
}

// CreateMediaStage is also usable on NewTx: admission and its parent continuation
// can be committed together. The caller authorizes the parent through job's port.
func (s *Store) CreateMediaStage(ctx context.Context, in clip.MediaStageInput, now time.Time) (clip.MediaStage, error) {
	if err := in.Validate(); err != nil {
		return clip.MediaStage{}, err
	}
	if now.IsZero() || !json.Valid([]byte(in.Payload)) {
		return clip.MediaStage{}, clip.ErrInvalid
	}
	return transact(ctx, s, func(q *sqlc.Queries) (clip.MediaStage, error) {
		// An existing id must never be rebound to a different parent or operation.
		old, e := q.GetMediaStage(ctx, in.ID)
		if e == nil && (old.ParentJobID != in.ParentJobID || old.StageKey != string(in.Operation)) {
			return clip.MediaStage{}, clip.ErrMediaConflict
		}
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return clip.MediaStage{}, e
		}
		project, e := q.GetClipProject(ctx, sqlc.GetClipProjectParams{ID: in.ProjectID, UserID: in.UserID})
		if e != nil {
			return clip.MediaStage{}, dbError(e)
		}
		if project.EditPlanRevision != int64(in.ExpectedRevision) {
			return clip.MediaStage{}, clip.ErrMediaConflict
		}
		err := q.InsertMediaStage(ctx, sqlc.InsertMediaStageParams{ID: in.ID, ParentJobID: in.ParentJobID, UserID: in.UserID, ProjectID: in.ProjectID, ExpectedRevision: int64(in.ExpectedRevision), StageKey: string(in.Operation), Operation: string(in.Operation), ContractVersion: int64(in.ContractVersion), InputDigest: in.InputDigest, InputPayload: in.Payload, RendererVersion: in.RendererVersion, AssetVersion: in.AssetVersion, CreatedAt: stamp(now), QueueDeadlineAt: stamp(now.Add(in.Limits.WaitTimeout)), DeadlineAt: stamp(now.Add(in.Limits.StageTimeout)), LeaseTtlNs: int64(in.Limits.LeaseTTL), AttemptLimit: int64(in.Limits.MaxAttempts)})
		if err != nil {
			return clip.MediaStage{}, err
		}
		r, err := q.GetMediaStageByParent(ctx, sqlc.GetMediaStageByParentParams{ParentJobID: in.ParentJobID, StageKey: string(in.Operation)})
		if err != nil {
			return clip.MediaStage{}, err
		}
		stage, err := mediaStageRow(r)
		if err != nil {
			return stage, err
		}
		in.ID = stage.ID // the durable business key is parent + operation
		if stage.MediaStageInput != in {
			return clip.MediaStage{}, clip.ErrMediaConflict
		}
		return stage, nil
	})
}

func (s *Store) GetMediaStage(ctx context.Context, id string) (clip.MediaStage, error) {
	r, err := s.read.GetMediaStage(ctx, id)
	if err != nil {
		return clip.MediaStage{}, dbError(err)
	}
	return mediaStageRow(r)
}

func mediaTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func mediaLeaseEnd(stage clip.MediaStage, now time.Time) time.Time {
	end := now.Add(stage.Limits.LeaseTTL)
	if end.After(stage.DeadlineAt) {
		return stage.DeadlineAt
	}
	return end
}

func (s *Store) ClaimMediaStage(ctx context.Context, profile clip.MediaWorkerProfile, now time.Time) (*clip.MediaLease, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if now.IsZero() || !json.Valid([]byte(profile.RuntimeManifest)) {
		return nil, clip.ErrInvalid
	}
	return transact(ctx, s, func(q *sqlc.Queries) (*clip.MediaLease, error) {
		attempt, token := rand.Text(), rand.Text()
		r, err := q.ClaimMediaStage(ctx, sqlc.ClaimMediaStageParams{AttemptID: nullable(attempt), Operation: string(profile.Operation), ContractVersion: int64(profile.ContractVersion), RendererVersion: profile.RendererVersion, AssetVersion: profile.AssetVersion, Now: stamp(now)})
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		stage, err := mediaStageRow(r)
		if err != nil {
			return nil, err
		}
		if err = q.ExpireMediaAttempts(ctx, sqlc.ExpireMediaAttemptsParams{Now: nullable(stamp(now)), StageID: stage.ID}); err != nil {
			return nil, err
		}
		end := mediaLeaseEnd(stage, now)
		if err = q.InsertMediaAttempt(ctx, sqlc.InsertMediaAttemptParams{ID: attempt, StageID: stage.ID, Ordinal: int64(stage.AttemptCount), WorkerID: profile.WorkerID, TokenHash: mediaTokenHash(token), LeaseExpiresAt: stamp(end), StartedAt: stamp(now), SelectedProfile: profile.Profile, RuntimeManifest: profile.RuntimeManifest}); err != nil {
			return nil, err
		}
		return &clip.MediaLease{Stage: stage, Credentials: clip.MediaLeaseCredentials{StageID: stage.ID, AttemptID: attempt, WorkerID: profile.WorkerID, Token: token}, Profile: profile, ExpiresAt: end, HeartbeatAfter: min(clip.MediaHeartbeatHint, end.Sub(now)/4)}, nil
	})
}

func leaseError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return clip.ErrMediaLeaseLost
	}
	return err
}

func (s *Store) RenewMediaLease(ctx context.Context, auth clip.MediaLeaseCredentials, progress int, now time.Time) (time.Time, error) {
	if progress < 0 || progress > clip.MediaProgressMax || now.IsZero() {
		return time.Time{}, clip.ErrInvalid
	}
	return transact(ctx, s, func(q *sqlc.Queries) (time.Time, error) {
		r, err := q.GetMediaStage(ctx, auth.StageID)
		if err != nil {
			return time.Time{}, leaseError(err)
		}
		stage, err := mediaStageRow(r)
		if err != nil {
			return time.Time{}, err
		}
		end := mediaLeaseEnd(stage, now)
		_, err = q.RenewMediaAttempt(ctx, sqlc.RenewMediaAttemptParams{ExpiresAt: stamp(end), Progress: int64(progress), AttemptID: auth.AttemptID, StageID: auth.StageID, WorkerID: auth.WorkerID, TokenHash: mediaTokenHash(auth.Token), Now: stamp(now)})
		if err != nil {
			return time.Time{}, leaseError(err)
		}
		return end, nil
	})
}

func (s *Store) AcceptMediaResult(ctx context.Context, auth clip.MediaLeaseCredentials, result string, now time.Time) (clip.MediaStage, error) {
	if now.IsZero() || len(result) > clip.MediaPayloadMaxBytes || !json.Valid([]byte(result)) {
		return clip.MediaStage{}, clip.ErrInvalid
	}
	return transact(ctx, s, func(q *sqlc.Queries) (clip.MediaStage, error) {
		r, err := q.GetMediaStage(ctx, auth.StageID)
		if err != nil {
			return clip.MediaStage{}, leaseError(err)
		}
		if r.State == string(clip.MediaSucceeded) {
			a, err := q.GetMediaAttempt(ctx, auth.AttemptID)
			if err != nil {
				return clip.MediaStage{}, leaseError(err)
			}
			if r.CurrentAttemptID.String != auth.AttemptID || a.StageID != auth.StageID || a.WorkerID != auth.WorkerID || subtle.ConstantTimeCompare([]byte(a.TokenHash), []byte(mediaTokenHash(auth.Token))) != 1 {
				return clip.MediaStage{}, clip.ErrMediaLeaseLost
			}
			if r.AcceptedResult.String != result {
				return clip.MediaStage{}, clip.ErrMediaConflict
			}
			return mediaStageRow(r)
		}
		r, err = q.AcceptMediaStage(ctx, sqlc.AcceptMediaStageParams{Result: nullable(result), StageID: auth.StageID, AttemptID: nullable(auth.AttemptID), Now: stamp(now), WorkerID: auth.WorkerID, TokenHash: mediaTokenHash(auth.Token)})
		if err != nil {
			return clip.MediaStage{}, leaseError(err)
		}
		if err = q.FinishMediaAttempt(ctx, sqlc.FinishMediaAttemptParams{FinishedAt: nullable(stamp(now)), ID: auth.AttemptID}); err != nil {
			return clip.MediaStage{}, err
		}
		return mediaStageRow(r)
	})
}

func (s *Store) ReserveMediaArtifact(ctx context.Context, auth clip.MediaLeaseCredentials, artifact clip.MediaArtifactReservation, now time.Time) error {
	if now.IsZero() || !clip.ValidMediaLabel(artifact.Slot) || !clip.ValidMediaLabel(artifact.ObjectKey) || !clip.ValidMediaLabel(artifact.ContentType) || artifact.MaxBytes <= 0 {
		return clip.ErrInvalid
	}
	n, err := s.write.ReserveMediaArtifact(ctx, sqlc.ReserveMediaArtifactParams{Slot: artifact.Slot, ObjectKey: artifact.ObjectKey, ContentType: artifact.ContentType, MaxBytes: artifact.MaxBytes, Now: stamp(now), StageID: auth.StageID, AttemptID: auth.AttemptID, WorkerID: auth.WorkerID, TokenHash: mediaTokenHash(auth.Token)})
	if err != nil {
		return err
	}
	if n == 0 {
		return clip.ErrMediaLeaseLost
	}
	return nil
}
