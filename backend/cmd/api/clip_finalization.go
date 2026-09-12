package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/clip"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	jobstore "github.com/postpilot/backend/internal/job/store"
)

type clipFinalizer struct {
	writer *sql.DB
	clips  *clipstore.Store
	cfg    clip.RenderConfig
}

func (f clipFinalizer) Finalize(ctx context.Context, req clip.FinalizationRequest) (clip.Project, error) {
	var result clip.Project
	err := clipWriteTx(ctx, f.writer, func(conn *sql.Conn) error {
		clips, jobs := clipstore.NewTx(conn), jobstore.NewTx(conn)
		p, err := clips.GetProject(ctx, req.UserID, req.ProjectID)
		if err != nil {
			return err
		}
		if err := clip.ValidateFinalization(p, req, f.cfg); err != nil {
			return err
		}
		if p.Finalized != nil {
			result = p
			return nil
		}
		active, err := jobs.ActiveForClip(ctx, req.UserID, req.ProjectID)
		if err != nil {
			return err
		}
		if active != nil {
			return clip.ErrBusy
		}
		result, err = clips.RecordFinalization(ctx, req, time.Now())
		return err
	})
	if err == nil {
		return result, nil
	}
	// A lost commit response cannot reopen editing or change the chosen result.
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	p, readErr := f.clips.GetProject(readCtx, req.UserID, req.ProjectID)
	if readErr == nil && p.Finalized != nil && p.Finalized.PlanRevision == req.ExpectedRevision && p.Finalized.ResultID == req.ExpectedResultID {
		return p, nil
	}
	return clip.Project{}, err
}
