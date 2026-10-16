package app

import (
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func (s *GenerationService) validateRevisionAllowance(in clip.RevisionInput) error {
	if validator, ok := s.planner.(interface {
		ValidateRevision(llm.ModelRef, clip.RevisionInput) error
	}); ok {
		return validator.ValidateRevision(in.Policy.Ref, in)
	}
	return nil
}
func (s *GenerationService) validateStorylineAllowance(in clip.StorylineInput) error {
	if validator, ok := s.planner.(interface {
		ValidateStorylineRequest(llm.ModelRef, clip.StorylineInput) error
	}); ok {
		return validator.ValidateStorylineRequest(in.Policy.Ref, in)
	}
	return nil
}
