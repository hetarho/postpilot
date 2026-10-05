package app

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
)

type spokenRecoveryCorrector interface {
	CorrectSpokenRecovery(context.Context, string, string, string, *clip.NarrationPlan) (*clip.RecoveryState, error)
}

func (s *GenerationService) SpokenRecovery(ctx context.Context, owner, project string) (*clip.RecoveryState, error) {
	if _, e := s.projects.store.GetProject(ctx, owner, project); e != nil {
		return nil, e
	}
	return s.loadRecovery(ctx, owner, project)
}
func (s *GenerationService) CorrectSpokenRecovery(ctx context.Context, owner, project, digest string, n *clip.NarrationPlan) (*clip.RecoveryState, error) {
	p, e := s.projects.store.GetProject(ctx, owner, project)
	if e != nil {
		return nil, e
	}
	if p.EditPlan != "" {
		return nil, clip.ErrPlanConflict
	}
	if p.Finalized != nil {
		return nil, clip.ErrFinalized
	}
	if j, e := s.jobs.Active(ctx, owner, project); e != nil {
		return nil, e
	} else if j != nil {
		return nil, clip.ErrBusy
	}
	store, ok := s.store.(spokenRecoveryCorrector)
	if !ok {
		return nil, clip.ErrCompositionUnavailable
	}
	return store.CorrectSpokenRecovery(ctx, owner, project, digest, n)
}
