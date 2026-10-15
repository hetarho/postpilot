package app

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// The metadata result and its paid job become terminal in the same writer transaction.
func (f Finisher) CompleteUploadText(ctx context.Context, in clip.UploadTextSave) (clip.UploadText, error) {
	var result clip.UploadText
	committed := func(ctx context.Context) (clip.UploadText, bool) {
		j, e := f.jobs.GetByID(ctx, in.JobID)
		if e != nil || j.Status != job.StatusDone || j.Kind != clip.JobKindUploadText || j.UserID != in.UserID || j.Subject(clip.JobSubject) != in.ProjectID {
			return clip.UploadText{}, false
		}
		store, ok := f.clips.(clip.UploadTextStore)
		if !ok {
			return clip.UploadText{}, false
		}
		rows, e := store.ListUploadTexts(ctx, in.UserID, in.ProjectID)
		if e != nil {
			return clip.UploadText{}, false
		}
		for _, row := range rows {
			if row.Channel == in.Channel && row.LastJobID == in.JobID {
				return row, true
			}
		}
		return clip.UploadText{}, false
	}
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	if row, ok := committed(cleanup); ok {
		return row, nil
	}
	err := WriteTx(ctx, f.writer, f.bind, func(p Ports) error {
		j, e := p.Jobs.GetByID(ctx, in.JobID)
		if e != nil {
			return e
		}
		if j.CancelRequestedAt != nil {
			return context.Canceled
		}
		if j.UserID != in.UserID || j.Subject(clip.JobSubject) != in.ProjectID || j.Kind != clip.JobKindUploadText || j.Status != job.StatusRunning {
			return clip.ErrBusy
		}
		var frozen clip.UploadTextSnapshot
		if clip.StrictJSON(string(j.Payload), &frozen) != nil || frozen.Version != 1 || frozen.UserID != in.UserID || frozen.ProjectID != in.ProjectID || frozen.Input.Channel != in.Channel || frozen.TextRevision != in.ExpectedRevision || frozen.PlanRevision != in.ExpectedPlanRevision || frozen.Approval == nil || !frozen.Approval.Pricing.UploadText {
			return clip.ErrInvalid
		}
		store, ok := p.Clips.(clip.UploadTextStore)
		if !ok {
			return clip.ErrCompositionUnavailable
		}
		in.Now = f.now()
		in.Edited = false
		result, e = store.SaveUploadText(ctx, in)
		if e != nil {
			return e
		}
		return p.Jobs.Finish(ctx, in.JobID, job.StatusDone, nil, f.now())
	})
	if err == nil {
		return result, nil
	}
	if row, ok := committed(cleanup); ok {
		return row, nil
	}
	return clip.UploadText{}, errors.Join(err, ctx.Err())
}
