package app

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func (s *Service) FinalizeProject(ctx context.Context, req clip.FinalizationRequest) (clip.Project, error) {
	if s.finalizer == nil {
		return clip.Project{}, clip.ErrFinalizationInvalid
	}
	p, err := s.finalizer.Finalize(ctx, req)
	if err != nil {
		return clip.Project{}, err
	}
	// The transaction has already fenced originals and persisted cleanup. Storage
	// outages cannot undo confirmation; the source sweeper owns further retries.
	if s.sources != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.sources.RevokeProject(cleanup, req.UserID, req.ProjectID)
	}
	return p, nil
}

func (s *GenerationService) FinalizationRefusal(p clip.Project, busy bool) string {
	if p.Finalized != nil {
		return "finalized"
	}
	if busy {
		return "busy"
	}
	if p.Result == nil || p.Result.ID == "" {
		return "missing_render"
	}
	err := clip.ValidateFinalization(p, clip.FinalizationRequest{UserID: p.UserID, ProjectID: p.ID, ExpectedRevision: p.EditPlanRevision, ExpectedResultID: p.Result.ID}, s.cfg.Render)
	if errors.Is(err, clip.ErrFinalizationConflict) {
		return "stale_render"
	}
	if err != nil {
		return "invalid_plan"
	}
	return ""
}
