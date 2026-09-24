package store

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/store/sqlc"
)

func (s *Store) ConsumeMediaRender(ctx context.Context, c clip.AttemptResult, now time.Time) error {
	if s.writer != nil {
		return errors.New("media publication requires a coordinated transaction")
	}
	stage, err := s.MediaStageForJob(ctx, c.JobID, clip.MediaRender)
	if errors.Is(err, clip.ErrNotFound) {
		return nil
	} // temporary embedded rollout
	if err != nil {
		return err
	}
	if stage.State != clip.MediaSucceeded || stage.UserID != c.UserID || stage.ProjectID != c.ProjectID || stage.ExpectedRevision != c.ExpectedRevision || c.Result.Kind != clip.RenderServer {
		return clip.ErrMediaConflict
	}
	task, err := mediacodec.DecodeTask(stage.Payload)
	if err != nil {
		return err
	}
	batch, err := s.MediaSourceBatch(ctx, c.UserID, c.JobID)
	if err != nil {
		return err
	}
	if err = clip.ValidateMediaSourceBinding(clip.MediaRender, task, batch, now); err != nil {
		return err
	}
	receipt, err := mediacodec.DecodeResult(stage.AcceptedResult)
	if err != nil || len(receipt.Outputs) != 1 {
		return clip.ErrInvalidMedia
	}
	artifacts, err := s.MediaArtifacts(ctx, stage.CurrentAttemptID)
	if err != nil {
		return err
	}
	if len(artifacts) != 1 {
		return clip.ErrInvalidMedia
	}
	a := artifacts[0]
	if a.State != "accepted" || a.Slot != "result" || !reflect.DeepEqual(a.MediaOutput, receipt.Outputs[0]) || a.ObjectKey != c.Result.Key || a.Bytes != c.Result.Bytes || a.ContentType != c.Result.ContentType || a.DurationMS != c.Result.DurationMS || !a.CreatedAt.Equal(c.Result.CreatedAt) {
		return clip.ErrMediaConflict
	}
	n, err := s.write.ConsumeMediaRender(ctx, sqlc.ConsumeMediaRenderParams{AttemptID: stage.CurrentAttemptID, ObjectKey: a.ObjectKey})
	if err == nil && n != 1 {
		return clip.ErrMediaConflict
	}
	return err
}
