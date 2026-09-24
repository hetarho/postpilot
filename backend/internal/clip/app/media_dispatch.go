package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/job"
)

// MediaDispatchTx is the stage admission/read behavior used by an owning job.
// It shares the writer transaction with job continuation and source ownership.
type MediaDispatchTx interface {
	CreateMediaStage(context.Context, clip.MediaStageInput, time.Time) (clip.MediaStage, error)
	MediaStageForJob(context.Context, string, clip.MediaOperation) (clip.MediaStage, error)
}
type MediaDispatch struct {
	writer *sql.DB
	bind   Binder
	limits clip.MediaStageLimits
	cfg    clip.MediaConfig
	now    func() time.Time
}

func NewMediaDispatch(writer *sql.DB, bind Binder, limits clip.MediaStageLimits, cfg clip.MediaConfig, now func() time.Time) (*MediaDispatch, error) {
	if writer == nil || bind == nil {
		return nil, clip.ErrCompositionUnavailable
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &MediaDispatch{writer, bind, limits, cfg, now}, nil
}
func MediaWaitKey(stage string) string { return "clip-media:" + stage }

type MediaDispatchRequest struct {
	UserID, JobID, ProjectID string
	Revision                 int
	Operation                clip.MediaOperation
	Task                     clip.MediaTask
}

// Request parks before yielding, or consumes a receipt only after the queue has
// claimed its durable continuation. The approved parent payload never changes.
func (d *MediaDispatch) Request(ctx context.Context, in MediaDispatchRequest) (stage clip.MediaStage, artifacts []clip.MediaArtifact, err error) {
	if err = clip.ValidateMediaTask(in.Operation, in.Task, d.cfg); err != nil {
		return
	}
	payload, err := mediacodec.EncodeTask(in.Task)
	if err != nil {
		return
	}
	pending := false
	err = WriteTx(ctx, d.writer, d.bind, func(p Ports) error {
		if p.Stages == nil || p.Waits == nil || p.Media == nil {
			return clip.ErrCompositionUnavailable
		}
		now := d.now().UTC()
		parent, err := p.Jobs.GetByID(ctx, in.JobID)
		if err != nil {
			return err
		}
		if parent.CancelRequestedAt != nil {
			return clip.ErrMediaCancelled
		}
		if parent.UserID != in.UserID || parent.Subject(clip.JobSubject) != in.ProjectID || parent.Status != job.StatusRunning || !parent.DispatchReady || !clip.IsJobKind(parent.Kind) {
			return clip.ErrMediaLeaseLost
		}
		project, err := p.Clips.GetProject(ctx, in.UserID, in.ProjectID)
		if err != nil {
			return err
		}
		if project.Finalized != nil || project.EditPlanRevision != in.Revision {
			return clip.ErrMediaCancelled
		}
		batch, err := p.Media.MediaSourceBatch(ctx, in.UserID, in.JobID)
		if err != nil {
			return err
		}
		if batch.ProjectID != in.ProjectID || batch.AccessDenied || len(batch.Sources) != len(in.Task.Sources) {
			return clip.ErrSourceState
		}
		for i, s := range batch.Sources {
			frozen := in.Task.Sources[i]
			if s.ID != frozen.ID || s.SourceMetadata != frozen.SourceMetadata || s.ActualBytes != s.Bytes || s.State != "ready" || s.CleanupPending || !s.ExpiresAt.After(now) {
				return clip.ErrSourceState
			}
		}
		stage, err = p.Stages.MediaStageForJob(ctx, in.JobID, in.Operation)
		if errors.Is(err, clip.ErrNotFound) {
			stage, err = p.Stages.CreateMediaStage(ctx, clip.MediaStageInput{ID: newID(), ParentJobID: in.JobID, UserID: in.UserID, ProjectID: in.ProjectID, ExpectedRevision: in.Revision, Operation: in.Operation, ContractVersion: clip.MediaContractVersion, InputDigest: clip.MediaPayloadDigest(payload), Payload: payload, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Limits: d.limits}, now)
			if err != nil {
				return err
			}
			if err = p.Waits.Park(ctx, in.JobID, MediaWaitKey(stage.ID), job.FailOnInterrupt, now); err != nil {
				return err
			}
			pending = true
			return nil
		}
		if err != nil {
			return err
		}
		if stage.UserID != in.UserID || stage.ProjectID != in.ProjectID || stage.ExpectedRevision != in.Revision || stage.InputDigest != clip.MediaPayloadDigest(payload) || stage.Payload != payload {
			return clip.ErrMediaConflict
		}
		if stage.ContractVersion != clip.MediaContractVersion || stage.RendererVersion != clip.MediaRendererVersion || stage.AssetVersion != clip.MediaAssetVersion {
			return clip.ErrMediaIncompatible
		}
		switch stage.State {
		case clip.MediaQueued, clip.MediaRunning:
			pending = true
			return nil
		case clip.MediaSucceeded:
			continuation, err := p.Waits.Continuation(ctx, in.JobID)
			if err != nil {
				return err
			}
			if continuation.WaitKey != MediaWaitKey(stage.ID) || continuation.Policy != job.FailOnInterrupt {
				return job.ErrInvalidWait
			}
			if continuation.State != job.ContinuationClaimed {
				pending = true
				return nil
			}
			artifacts, err = p.Media.MediaArtifacts(ctx, stage.CurrentAttemptID)
			return err
		case clip.MediaCancelled:
			return clip.ErrMediaCancelled
		default:
			return clip.ErrInvalidMedia
		}
	})
	if err == nil && pending {
		err = job.ErrYield
	}
	return
}
