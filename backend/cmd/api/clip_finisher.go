package main

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

type clipFinisher struct {
	writer *sql.DB
	clips  *clipstore.Store
	jobs   *jobstore.Store
}

func (f clipFinisher) resultCommitted(ctx context.Context, c clip.AttemptResult) bool {
	j, err := f.jobs.GetByID(ctx, c.JobID)
	if err != nil || j.Status != job.StatusDone || j.UserID != c.UserID || j.ClipProjectID != c.ProjectID {
		return false
	}
	p, err := f.clips.GetProject(ctx, c.UserID, c.ProjectID)
	if err != nil {
		return false
	}
	// A generation commits a plan and no file, so the plan it saved is what says
	// the completion already landed (CLIP-151).
	if c.Result.Key == "" {
		return p.EditPlan == c.EditPlan && p.EditPlanRevision == c.ExpectedRevision+1
	}
	return p.Result != nil && p.Result.Key == c.Result.Key
}

func (f clipFinisher) Complete(ctx context.Context, c clip.AttemptResult) error {
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	j, err := f.jobs.GetByID(cleanup, c.JobID)
	if err != nil {
		return err
	}
	if j.UserID != c.UserID || j.ClipProjectID != c.ProjectID || !job.ClipKind(j.Kind) {
		return clip.ErrNotFound
	}
	if f.resultCommitted(cleanup, c) {
		return nil
	}
	// The staging row exists to remember an uploaded FILE across a crash, so
	// there is nothing to stage for a completion that produced none: the plan is
	// applied inside the job's own terminal transaction and nothing is left
	// behind to sweep (CLIP-151).
	plan := c.Result.Key == "" && c.EditPlan != ""
	if !plan {
		if err := f.clips.StageAttemptResult(cleanup, c); err != nil {
			// A committed stage may have lost its response. The immutable row decides.
			staged, readErr := f.clips.GetAttemptResult(cleanup, c.JobID)
			if readErr != nil || !clip.SameAttemptResult(staged, c) {
				return errors.Join(err, readErr)
			}
		}
	}
	err = clipWriteTx(ctx, f.writer, func(tx *sql.Tx) error {
		jobs, clips := jobstore.NewTx(tx), clipstore.NewTx(tx)
		j, err := jobs.GetByID(ctx, c.JobID)
		if err != nil {
			return err
		}
		if j.CancelRequestedAt != nil {
			return context.Canceled
		}
		if j.UserID != c.UserID || j.ClipProjectID != c.ProjectID || j.Status != job.StatusRunning {
			return clip.ErrBusy
		}
		candidate := c
		if !plan {
			candidate, err = clips.GetAttemptResult(ctx, c.JobID)
			if err != nil {
				return err
			}
		}
		if candidate.UserID != j.UserID || candidate.ProjectID != j.ClipProjectID || candidate.Result.Key != c.Result.Key || (j.Kind == job.KindGenerateClip) != (candidate.EditPlan != "") {
			return clip.ErrInvalid
		}
		if err := clips.ApplyAttemptResult(ctx, candidate); err != nil {
			return err
		}
		if err := jobs.Finish(ctx, j.ID, job.StatusDone, nil, time.Now()); err != nil {
			return err
		}
		if plan {
			return nil
		}
		return clips.DeleteAttemptResult(ctx, c.JobID)
	})
	if err == nil || f.resultCommitted(cleanup, c) {
		return nil
	}
	j, readErr := f.jobs.GetByID(cleanup, c.JobID)
	if readErr == nil && (job.Terminal(j.Status) || j.CancelRequestedAt != nil) {
		if !plan {
			err = errors.Join(err, f.clips.DiscardAttemptResult(cleanup, c.JobID))
		}
		if j.CancelRequestedAt != nil {
			return errors.Join(context.Canceled, err)
		}
	}
	return err
}

func (f clipFinisher) Recover(ctx context.Context) error {
	rows, err := f.clips.PendingAttemptResults(ctx)
	if err != nil {
		return err
	}
	for _, c := range rows {
		j, err := f.jobs.GetByID(ctx, c.JobID)
		if err != nil && !errors.Is(err, job.ErrNotFound) {
			return err
		}
		if err == nil && !job.Terminal(j.Status) {
			continue
		}
		if err := f.clips.DiscardAttemptResult(ctx, c.JobID); err != nil {
			return err
		}
	}
	return nil
}
