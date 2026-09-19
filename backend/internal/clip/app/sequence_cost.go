package app

import (
	"context"

	"github.com/postpilot/backend/internal/clip"
)

// SequenceCost answers what one project's approval surfaces show. A project
// whose stored plan cannot be read is quoted from its selection alone rather
// than refused: nothing here gates an action.
func (s *GenerationService) SequenceCost(ctx context.Context, user, id string) (clip.SequenceCaptionCost, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.SequenceCaptionCost{}, err
	}
	return s.sequenceCostOf(p), nil
}

func (s *GenerationService) sequenceCostOf(p clip.Project) clip.SequenceCaptionCost {
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	return clip.SequenceCostOf(p, plan, err == nil && p.EditPlan != "", s.cfg.Render)
}
