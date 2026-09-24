package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/job"
)

type MediaArtifactTx interface {
	AuthorizeMediaLease(context.Context, clip.MediaLeaseCredentials, time.Time, bool) (clip.MediaStage, error)
	MediaSourceBatch(context.Context, string, string) (clip.SourceBatch, error)
	ReserveMediaOutput(context.Context, clip.MediaLeaseCredentials, clip.MediaOutput, string, int64, time.Time, time.Time) (clip.MediaArtifact, error)
	MediaArtifacts(context.Context, string) ([]clip.MediaArtifact, error)
	AcceptMediaArtifacts(context.Context, clip.MediaLeaseCredentials, string, []clip.MediaOutput, time.Time) (clip.MediaStage, error)
}
type MediaArtifactObjects interface {
	PresignMediaRead(context.Context, string, time.Duration) (clip.MediaArtifactAccess, error)
	PresignMediaWrite(context.Context, string, string, int64, time.Duration) (clip.MediaArtifactAccess, error)
	HeadMediaArtifact(context.Context, string) (clip.SourceObjectInfo, error)
}

type MediaArtifacts struct {
	writer  *sql.DB
	bind    Binder
	objects MediaArtifactObjects
	config  clip.MediaConfig
	now     func() time.Time
}

func NewMediaArtifacts(writer *sql.DB, bind Binder, objects MediaArtifactObjects, cfg clip.MediaConfig, now func() time.Time) *MediaArtifacts {
	if writer == nil || bind == nil || objects == nil {
		panic("clip app: media artifacts need writer, binder and object access")
	}
	if now == nil {
		now = time.Now
	}
	return &MediaArtifacts{writer: writer, bind: bind, objects: objects, config: cfg, now: now}
}

func (a *MediaArtifacts) authorized(ctx context.Context, p Ports, auth clip.MediaLeaseCredentials, allowAccepted bool) (clip.MediaStage, clip.MediaTask, error) {
	stage, err := p.Media.AuthorizeMediaLease(ctx, auth, a.now().UTC(), allowAccepted)
	if err != nil {
		return stage, clip.MediaTask{}, err
	}
	task, err := mediacodec.DecodeTask(stage.Payload)
	if err != nil {
		return stage, task, err
	}
	if err = clip.ValidateMediaTask(stage.Operation, task, a.config); err != nil {
		return stage, task, err
	}
	if allowAccepted && stage.State == clip.MediaSucceeded {
		return stage, task, nil
	}
	err = authorizeMediaParent(ctx, p, stage, task, a.now())
	return stage, task, err
}

func authorizeMediaParent(ctx context.Context, p Ports, stage clip.MediaStage, task clip.MediaTask, now time.Time) error {
	j, err := p.Jobs.GetByID(ctx, stage.ParentJobID)
	if err != nil {
		if errors.Is(err, job.ErrNotFound) {
			return clip.ErrMediaLeaseLost
		}
		return err
	}
	if j.CancelRequestedAt != nil {
		return clip.ErrMediaCancelled
	}
	if j.Status != job.StatusRunning || !j.DispatchReady || j.UserID != stage.UserID || j.Subject(clip.JobSubject) != stage.ProjectID || !clip.IsJobKind(j.Kind) {
		return clip.ErrMediaLeaseLost
	}
	project, err := p.Clips.GetProject(ctx, stage.UserID, stage.ProjectID)
	if err != nil {
		if errors.Is(err, clip.ErrNotFound) {
			return clip.ErrMediaLeaseLost
		}
		return err
	}
	if project.Finalized != nil || project.EditPlanRevision != stage.ExpectedRevision {
		return clip.ErrMediaCancelled
	}
	batch, err := p.Media.MediaSourceBatch(ctx, stage.UserID, stage.ParentJobID)
	if err != nil {
		return err
	}
	if batch.ProjectID != stage.ProjectID {
		return clip.ErrSourceState
	}
	return clip.ValidateMediaSourceBinding(stage.Operation, task, batch, now)
}

func accessTTL(deadline, now time.Time) (time.Duration, error) {
	ttl := min(clip.MediaArtifactAccessTTL, deadline.Sub(now)).Truncate(time.Second)
	if ttl < time.Second {
		return 0, clip.ErrMediaLeaseLost
	}
	return ttl, nil
}

func (a *MediaArtifacts) Read(ctx context.Context, auth clip.MediaLeaseCredentials, slot string) (clip.MediaArtifactAccess, error) {
	var source clip.SourceLease
	var deadline time.Time
	err := WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		stage, task, err := a.authorized(ctx, p, auth, false)
		if err != nil {
			return err
		}
		if output, ok := strings.CutPrefix(slot, "output/"); ok {
			artifacts, err := p.Media.MediaArtifacts(ctx, auth.AttemptID)
			if err != nil {
				return err
			}
			for _, artifact := range artifacts {
				if artifact.Slot == output {
					source.Key, source.ActualBytes, source.ContentType = artifact.ObjectKey, artifact.Bytes, artifact.ContentType
					deadline = stage.DeadlineAt
					return nil
				}
			}
			return clip.ErrInvalid
		}
		if !strings.HasPrefix(slot, "source/") {
			return clip.ErrInvalid
		}
		id := strings.TrimPrefix(slot, "source/")
		var frozen *clip.MediaTaskSource
		for i := range task.Sources {
			if task.Sources[i].ID == id {
				frozen = &task.Sources[i]
				break
			}
		}
		if frozen == nil {
			return clip.ErrInvalid
		}
		batch, err := p.Media.MediaSourceBatch(ctx, stage.UserID, stage.ParentJobID)
		if err != nil || batch.ProjectID != stage.ProjectID || batch.AccessDenied {
			return clip.ErrMediaLeaseLost
		}
		source, err = clip.ResolveMediaSource(stage.Operation, *frozen, batch, a.now())
		if err != nil {
			return err
		}
		deadline = stage.DeadlineAt
		if source.ExpiresAt.Before(deadline) {
			deadline = source.ExpiresAt
		}
		return nil
	})
	if err != nil {
		return clip.MediaArtifactAccess{}, err
	}
	ttl, err := accessTTL(deadline, a.now())
	if err != nil {
		return clip.MediaArtifactAccess{}, err
	}
	out, err := a.objects.PresignMediaRead(ctx, source.Key, ttl)
	if err != nil {
		return clip.MediaArtifactAccess{}, err
	}
	out.Slot, out.Bytes, out.ContentType = slot, source.ActualBytes, source.ContentType
	return out, nil
}

func (a *MediaArtifacts) Reserve(ctx context.Context, auth clip.MediaLeaseCredentials, outputs []clip.MediaOutput) ([]clip.MediaArtifactAccess, error) {
	if len(outputs) == 0 || len(outputs) > a.config.Sources.MaxCount+a.config.Sources.MaxDurationMS/a.config.ChunkDurationMS {
		return nil, clip.ErrInvalid
	}
	var artifacts []clip.MediaArtifact
	var deadline time.Time
	err := WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		stage, task, err := a.authorized(ctx, p, auth, false)
		if err != nil {
			return err
		}
		deadline = stage.DeadlineAt
		now := a.now().UTC()
		ttl, err := accessTTL(deadline, now)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, out := range outputs {
			if seen[out.Slot] {
				return clip.ErrInvalid
			}
			seen[out.Slot] = true
			if err := clip.ValidateMediaOutput(stage.Operation, task, out, a.config); err != nil {
				return err
			}
			prefix, limit := clip.MediaAnalysisPrefix, a.config.AnalysisMaxBytes
			if stage.Operation == clip.MediaRender {
				prefix, limit = clip.ResultPrefix, a.config.Sources.MaxFileBytes
			}
			key := prefix + url.PathEscape(stage.UserID) + "/" + url.PathEscape(stage.ProjectID) + "/" + auth.AttemptID + "/" + rand.Text() + ".mp4"
			artifact, err := p.Media.ReserveMediaOutput(ctx, auth, out, key, limit, now.Add(ttl), now)
			if err != nil {
				return err
			}
			artifacts = append(artifacts, artifact)
		}
		if stage.Operation == clip.MediaPrepare {
			reserved, err := p.Media.MediaArtifacts(ctx, auth.AttemptID)
			if err != nil {
				return err
			}
			var total int64
			for _, artifact := range reserved {
				if artifact.Bytes > a.config.PreparedMaxBytes-total {
					return clip.ErrAnalysisTooLarge
				}
				total += artifact.Bytes
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	access := make([]clip.MediaArtifactAccess, 0, len(artifacts))
	for _, artifact := range artifacts {
		ttl, err := accessTTL(artifact.PutExpiresAt, a.now())
		if err != nil {
			return nil, err
		}
		out, err := a.objects.PresignMediaWrite(ctx, artifact.ObjectKey, artifact.ContentType, artifact.Bytes, ttl)
		if err != nil {
			return nil, err
		}
		out.Slot = artifact.Slot
		access = append(access, out)
	}
	return access, nil
}

func (a *MediaArtifacts) Complete(ctx context.Context, auth clip.MediaLeaseCredentials, raw string) error {
	_, err := a.Accept(ctx, auth, raw)
	return err
}

func (a *MediaArtifacts) Accept(ctx context.Context, auth clip.MediaLeaseCredentials, raw string) (clip.MediaStage, error) {
	result, err := mediacodec.DecodeResult(raw)
	if err != nil {
		return clip.MediaStage{}, err
	}
	canonical, err := mediacodec.EncodeResult(result)
	if err != nil {
		return clip.MediaStage{}, err
	}
	var stage clip.MediaStage
	var artifacts []clip.MediaArtifact
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		var task clip.MediaTask
		stage, task, err = a.authorized(ctx, p, auth, true)
		if err != nil {
			return err
		}
		if stage.State == clip.MediaSucceeded {
			if stage.AcceptedResult != canonical {
				return clip.ErrMediaConflict
			}
			return nil
		}
		if err := clip.ValidateMediaResult(stage.Operation, task, result, a.config); err != nil {
			return err
		}
		artifacts, err = p.Media.MediaArtifacts(ctx, auth.AttemptID)
		if err != nil {
			return err
		}
		if len(artifacts) != len(result.Outputs) {
			return clip.ErrInvalid
		}
		bySlot := map[string]clip.MediaOutput{}
		for _, out := range result.Outputs {
			bySlot[out.Slot] = out
		}
		for _, artifact := range artifacts {
			if artifact.State != "reserved" || !reflect.DeepEqual(artifact.MediaOutput, bySlot[artifact.Slot]) {
				return clip.ErrMediaConflict
			}
		}
		return nil
	})
	if err != nil || stage.State == clip.MediaSucceeded {
		return stage, err
	}
	// No network I/O holds the sole SQLite writer. Conditional PUT means a
	// still-valid upload URL cannot replace a checked object after this HEAD.
	for _, artifact := range artifacts {
		info, err := a.objects.HeadMediaArtifact(ctx, artifact.ObjectKey)
		if err != nil {
			return stage, err
		}
		if info.Bytes != artifact.Bytes || info.ContentType != artifact.ContentType {
			return stage, clip.ErrInvalidMedia
		}
	}
	err = WriteTx(ctx, a.writer, a.bind, func(p Ports) error {
		if _, _, err := a.authorized(ctx, p, auth, true); err != nil {
			return err
		}
		stage, err = p.Media.AcceptMediaArtifacts(ctx, auth, canonical, result.Outputs, a.now().UTC())
		if err != nil {
			return err
		}
		// Receipt and wake are one commit. An exact replay never reopens a
		// claimed continuation or re-enters paid work.
		if p.Waits != nil {
			_, err = p.Waits.Wake(ctx, stage.ParentJobID, MediaWaitKey(stage.ID), a.now().UTC())
		}
		return err
	})
	return stage, err
}
