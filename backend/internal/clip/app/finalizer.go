package app

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/clip"
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
		active, err := p.Jobs.ActiveForClip(ctx, req.UserID, req.ProjectID)
		if err != nil {
			return err
		}
		if active != nil {
			return clip.ErrBusy
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
