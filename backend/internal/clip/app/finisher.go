package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// Finisher is the terminal-commit saga: a finished attempt's job row and its
// clip result land in one writer transaction, or neither does. A candidate
// stays staged until then so a crash between upload and commit loses nothing.
type Finisher struct {
	writer *sql.DB
	bind   Binder
	jobs   JobReader
	clips  ClipStore
	now    func() time.Time
}

func NewFinisher(writer *sql.DB, bind Binder, jobs JobReader, clips ClipStore, now func() time.Time) Finisher {
	if writer == nil || bind == nil || jobs == nil || clips == nil {
		panic("clip app: finisher needs writer, binder, jobs and clips")
	}
	if now == nil {
		now = time.Now
	}
	return Finisher{writer: writer, bind: bind, jobs: jobs, clips: clips, now: now}
}

func (f Finisher) resultCommitted(ctx context.Context, c clip.AttemptResult) bool {
	j, err := f.jobs.GetByID(ctx, c.JobID)
	if err != nil || j.Status != job.StatusDone || j.UserID != c.UserID || j.Subject(clip.JobSubject) != c.ProjectID {
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

func (f Finisher) Complete(ctx context.Context, c clip.AttemptResult) error {
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer stop()
	j, err := f.jobs.GetByID(cleanup, c.JobID)
	if err != nil {
		return err
	}
	if j.UserID != c.UserID || j.Subject(clip.JobSubject) != c.ProjectID || !clip.IsJobKind(j.Kind) {
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
	err = WriteTx(ctx, f.writer, f.bind, func(p Ports) error {
		j, err := p.Jobs.GetByID(ctx, c.JobID)
		if err != nil {
			return err
		}
		if j.CancelRequestedAt != nil {
			return context.Canceled
		}
		if j.UserID != c.UserID || j.Subject(clip.JobSubject) != c.ProjectID || j.Status != job.StatusRunning {
			return clip.ErrBusy
		}
		candidate := c
		if !plan {
			candidate, err = p.Clips.GetAttemptResult(ctx, c.JobID)
			if err != nil {
				return err
			}
		}
		if candidate.UserID != j.UserID || candidate.ProjectID != j.Subject(clip.JobSubject) || candidate.Result.Key != c.Result.Key || (j.Kind == clip.JobKindGenerate) != (candidate.EditPlan != "") {
			return clip.ErrInvalid
		}
		if err := p.Clips.ApplyAttemptResult(ctx, candidate); err != nil {
			return err
		}
		if err := p.Jobs.Finish(ctx, j.ID, job.StatusDone, nil, f.now()); err != nil {
			return err
		}
		if plan {
			return nil
		}
		return p.Clips.DeleteAttemptResult(ctx, c.JobID)
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

// Recover sweeps staged candidates whose job has already ended: a crash after
// the stage but before the commit leaves a row nobody will commit.
func (f Finisher) Recover(ctx context.Context) error {
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
