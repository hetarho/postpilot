package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func (s *Store) MediaSourceBatch(ctx context.Context, user, job string) (clip.SourceBatch, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.SourceBatch, error) {
		a, err := q.GetSourceAttempt(ctx, sqlc.GetSourceAttemptParams{UserID: user, JobID: job})
		if err != nil {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		if a.ReleasedAt.Valid {
			return clip.SourceBatch{}, clip.ErrSourceState
		}
		return (&Store{read: q, write: q}).BatchForJob(ctx, user, job)
	})
}

// AuthorizeMediaLease proves the opaque attempt credential before exposing even
// the parent identity to the application saga. Only an identical terminal receipt
// may use allowAccepted; artifact access always requires a live execution lease.
func (s *Store) AuthorizeMediaLease(ctx context.Context, auth clip.MediaLeaseCredentials, now time.Time, allowAccepted bool) (clip.MediaStage, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.MediaStage, error) {
		r, err := authenticatedMediaStage(ctx, q, auth)
		if err != nil {
			return clip.MediaStage{}, err
		}
		stage, err := mediaStageRow(r)
		if err != nil {
			return stage, err
		}
		if allowAccepted && stage.State == clip.MediaSucceeded {
			return stage, nil
		}
		closed, err := q.SourceProjectWritable(ctx, sqlc.SourceProjectWritableParams{ID: stage.ProjectID, UserID: stage.UserID})
		if err != nil {
			return stage, leaseError(err)
		}
		if closed != 0 {
			return stage, clip.ErrMediaCancelled
		}
		a, err := q.GetMediaAttempt(ctx, auth.AttemptID)
		if err != nil {
			return stage, leaseError(err)
		}
		end, err := time.Parse(time.RFC3339Nano, a.LeaseExpiresAt)
		if err != nil {
			return stage, err
		}
		if stage.State != clip.MediaRunning || a.Outcome.Valid || !end.After(now) || !stage.DeadlineAt.After(now) {
			return stage, clip.ErrMediaLeaseLost
		}
		return stage, nil
	})
}

func mediaArtifactRow(r sqlc.ClipMediaArtifact) (clip.MediaArtifact, error) {
	out := clip.MediaArtifact{AttemptID: r.AttemptID, ObjectKey: r.ObjectKey, State: r.State}
	if r.MetadataJson == "" || json.Unmarshal([]byte(r.MetadataJson), &out.MediaOutput) != nil {
		return out, clip.ErrInvalid
	}
	var err error
	out.CreatedAt, err = time.Parse(time.RFC3339Nano, r.CreatedAt)
	if err != nil {
		return out, err
	}
	if r.PutExpiresAt.Valid {
		out.PutExpiresAt, err = time.Parse(time.RFC3339Nano, r.PutExpiresAt.String)
	}
	return out, err
}

// ReserveMediaOutput is called on the saga's transaction-bound store, after the
// parent and lease checks. Repeating a lost response preserves its original key.
func (s *Store) ReserveMediaOutput(ctx context.Context, auth clip.MediaLeaseCredentials, output clip.MediaOutput, key string, limit int64, expires, now time.Time) (clip.MediaArtifact, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.MediaArtifact, error) {
		if _, err := (&Store{read: q, write: q}).AuthorizeMediaLease(ctx, auth, now, false); err != nil {
			return clip.MediaArtifact{}, err
		}
		if output.Bytes <= 0 || output.Bytes > limit || !expires.After(now) {
			return clip.MediaArtifact{}, clip.ErrInvalid
		}
		metadata, err := json.Marshal(output)
		if err != nil {
			return clip.MediaArtifact{}, clip.ErrInvalid
		}
		err = q.InsertMediaOutput(ctx, sqlc.InsertMediaOutputParams{AttemptID: auth.AttemptID, Slot: output.Slot, ObjectKey: key, ContentType: output.ContentType, MaxBytes: limit, ExactBytes: output.Bytes, MetadataJson: string(metadata), WorkerDigest: output.Digest, PutExpiresAt: nullable(stamp(expires)), CreatedAt: stamp(now)})
		if err != nil {
			return clip.MediaArtifact{}, err
		}
		r, err := q.GetMediaArtifact(ctx, sqlc.GetMediaArtifactParams{AttemptID: auth.AttemptID, Slot: output.Slot})
		if err != nil {
			return clip.MediaArtifact{}, err
		}
		if r.MetadataJson != string(metadata) || r.State != "reserved" {
			return clip.MediaArtifact{}, clip.ErrMediaConflict
		}
		if err = q.ExtendMediaPutExpiry(ctx, sqlc.ExtendMediaPutExpiryParams{AttemptID: auth.AttemptID, Slot: output.Slot, ExpiresAt: stamp(expires)}); err != nil {
			return clip.MediaArtifact{}, err
		}
		r, err = q.GetMediaArtifact(ctx, sqlc.GetMediaArtifactParams{AttemptID: auth.AttemptID, Slot: output.Slot})
		if err != nil {
			return clip.MediaArtifact{}, err
		}
		out, err := mediaArtifactRow(r)
		out.StageID = auth.StageID
		return out, err
	})
}

func (s *Store) MediaArtifacts(ctx context.Context, attempt string) ([]clip.MediaArtifact, error) {
	rows, err := s.read.ListMediaArtifacts(ctx, attempt)
	if err != nil {
		return nil, err
	}
	out := make([]clip.MediaArtifact, 0, len(rows))
	for _, r := range rows {
		artifact, err := mediaArtifactRow(r)
		if err != nil {
			return nil, err
		}
		out = append(out, artifact)
	}
	return out, nil
}

// AcceptMediaArtifacts checks every reservation again in the same writer
// transaction as the immutable result receipt. Storage HEADs happened before it.
func (s *Store) AcceptMediaArtifacts(ctx context.Context, auth clip.MediaLeaseCredentials, result string, outputs []clip.MediaOutput, now time.Time) (clip.MediaStage, error) {
	return transact(ctx, s, func(q *sqlc.Queries) (clip.MediaStage, error) {
		stage, err := (&Store{read: q, write: q}).AuthorizeMediaLease(ctx, auth, now, true)
		if err != nil {
			return stage, err
		}
		if stage.State == clip.MediaSucceeded {
			return (&Store{read: q, write: q}).AcceptMediaResult(ctx, auth, result, now)
		}
		rows, err := q.ListMediaArtifacts(ctx, auth.AttemptID)
		if err != nil {
			return stage, err
		}
		if len(rows) != len(outputs) {
			return stage, clip.ErrInvalid
		}
		for _, out := range outputs {
			metadata, err := json.Marshal(out)
			if err != nil {
				return stage, clip.ErrInvalid
			}
			n, err := q.AcceptMediaArtifact(ctx, sqlc.AcceptMediaArtifactParams{AttemptID: auth.AttemptID, Slot: out.Slot, Metadata: string(metadata), Now: nullable(stamp(now))})
			if err != nil {
				return stage, err
			}
			if n != 1 {
				return stage, clip.ErrMediaConflict
			}
		}
		return (&Store{read: q, write: q}).AcceptMediaResult(ctx, auth, result, now)
	})
}
