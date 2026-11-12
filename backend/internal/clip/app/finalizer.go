package app

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
)

// Finalizer is the clip.ProjectFinalizer port: confirming a result fences
// every later mutation, and it refuses while a job on the project is active.
type Finalizer struct {
	writer *sql.DB
	bind   Binder
	clips  ClipStore
	cfg    clip.RenderConfig
	now    func() time.Time
}

func NewFinalizer(writer *sql.DB, bind Binder, clips ClipStore, cfg clip.RenderConfig, now func() time.Time) Finalizer {
	if writer == nil || bind == nil || clips == nil {
		panic("clip app: finalizer needs writer, binder and clips")
	}
	if now == nil {
		now = time.Now
	}
	return Finalizer{writer: writer, bind: bind, clips: clips, cfg: cfg, now: now}
}

func (f Finalizer) Finalize(ctx context.Context, req clip.FinalizationRequest) (clip.Project, error) {
	var result clip.Project
	err := WriteTx(ctx, f.writer, f.bind, func(p Ports) error {
		found, err := p.Clips.GetProject(ctx, req.UserID, req.ProjectID)
		if err != nil {
			return err
		}
		if err := clip.ValidateFinalization(found, req, f.cfg); err != nil {
			return err
		}
		if found.Finalized != nil {
			result = found
			return nil
		}
		if req.PublishedURL != "" {
			if req.PublishedURL, err = clip.NormalizePublishedURL(found.Channel, req.PublishedURL); err != nil {
				return err
			}
		}
		if err := f.idle(ctx, p, req.UserID, req.ProjectID); err != nil {
			return err
		}
		result, err = p.Clips.RecordFinalization(ctx, req, f.now())
		return err
	})
	if err == nil {
		return result, nil
	}
	// A lost commit response cannot reopen editing or change the chosen result.
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	found, readErr := f.clips.GetProject(readCtx, req.UserID, req.ProjectID)
	if readErr == nil && found.Finalized != nil && found.Finalized.PlanRevision == req.ExpectedRevision && found.Finalized.ResultID == req.ExpectedResultID {
		return found, nil
	}
	return clip.Project{}, err
}

// Confirm records 확정하기 (CLIP-75, CLIP-229) under the same job exclusion as finalization.
// The same revision confirmed again succeeds without a write.
func (f Finalizer) Confirm(ctx context.Context, req clip.ConfirmationRequest) (clip.Project, error) {
	var result clip.Project
	err := WriteTx(ctx, f.writer, f.bind, func(p Ports) error {
		found, err := p.Clips.GetProject(ctx, req.UserID, req.ProjectID)
		if err != nil {
			return err
		}
		if err := clip.ValidateConfirmation(found, req, f.cfg); err != nil {
			return err
		}
		if clip.Confirmed(found) {
			result = found
			return nil
		}
		if err := f.idle(ctx, p, req.UserID, req.ProjectID); err != nil {
			return err
		}
		result, err = p.Clips.RecordConfirmation(ctx, req, f.now())
		return err
	})
	return result, err
}

// SavePublishedURL saves or replaces a finalized project's posted URL; a 완료 project that
// saves one becomes 게시됨, and nothing else changes (CLIP-232).
func (f Finalizer) SavePublishedURL(ctx context.Context, user, id, url string) (clip.Project, error) {
	var result clip.Project
	err := WriteTx(ctx, f.writer, f.bind, func(p Ports) error {
		found, err := p.Clips.GetProject(ctx, user, id)
		if err != nil {
			return err
		}
		if found.Finalized == nil {
			return clip.ErrFinalizationInvalid
		}
		normalized, err := clip.NormalizePublishedURL(found.Channel, url)
		if err != nil {
			return err
		}
		if err := f.idle(ctx, p, user, id); err != nil {
			return err
		}
		result, err = p.Clips.RecordPublishedURL(ctx, user, id, normalized, f.now())
		return err
	})
	return result, err
}

// idle refuses while a job or server render of the project runs, the predicate finalization uses.
func (f Finalizer) idle(ctx context.Context, p Ports, user, id string) error {
	active, err := p.Jobs.ActiveFor(ctx, job.Subject{Dimension: clip.JobSubject, ID: id}, job.Filter{UserID: user})
	if err != nil {
		return err
	}
	if active != nil {
		return clip.ErrBusy
	}
	return nil
}
