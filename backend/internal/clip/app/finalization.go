package app

import (
	"context"
	"errors"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

// ConfirmProject is 확정하기 (CLIP-75): it records the current plan revision and nothing else.
func (s *Service) ConfirmProject(ctx context.Context, req clip.ConfirmationRequest) (clip.Project, error) {
	return s.finalizer.Confirm(ctx, req)
}

// SavePublishedURL saves or replaces a finalized clip's posted URL (CLIP-232).
func (s *Service) SavePublishedURL(ctx context.Context, user, id, url string) (clip.Project, error) {
	return s.finalizer.SavePublishedURL(ctx, user, id, url)
}

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
	if !clip.Confirmed(p) {
		return "unconfirmed"
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

// ConfirmationRefusal is the project's 확정하기 verdict for the detail read.
func (s *GenerationService) ConfirmationRefusal(p clip.Project, busy bool) string {
	return clip.ConfirmationRefusal(p, busy, s.cfg.Render)
}
