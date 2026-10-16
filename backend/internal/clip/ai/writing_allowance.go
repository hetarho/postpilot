package ai

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// These paths run on frozen material before the worker acquires any credit hold.
// Actual calls still measure their exact requests, including bounded correction.
func (s *Service) validateWritingAllowance(model llm.ModelRef, in clip.PlanningInput, system, user string, schema []byte) error {
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return err
	}
	var output json.RawMessage
	if in.Policy.StructuredOutput {
		if !info.StructuredOutput {
			return clip.ErrPricingUnavailable
		}
		output = schema
	}
	return validatePrompt(system, user, output, llm.ExecutionTextOnly, in.Policy.InputTokenLimit())
}
func (s *Service) ValidateStorylineRequest(model llm.ModelRef, in clip.StorylineInput) error {
	system, user := BuildStorylinePrompt(in, compositionLimits(s.cfg, in.PlanningInput))
	return s.validateWritingAllowance(model, in.PlanningInput, system, user, StorylineSchema())
}
func (s *Service) ValidateRevision(model llm.ModelRef, in clip.RevisionInput) error {
	limits := compositionLimits(s.cfg, in.PlanningInput)
	flow := in.Current
	if in.Target != clip.RevisionNarration {
		system, user := buildFlowRevisionPrompt(in, s.cfg.Render.FadeMS, limits)
		if err := s.validateWritingAllowance(model, in.PlanningInput, system, user, RevisionFlowSchema()); err != nil {
			return err
		}
		// A revised flow is not yet known. Reserve room for the largest one the
		// admitted plan may return, exactly as generation preparation does.
		flow = WidestFlow(s.cfg, in.PlanningInput)
	}
	if in.Current.Narration != nil {
		system, user := buildSpokenRevisionPrompt(in, flow, limits)
		return s.validateWritingAllowance(model, in.PlanningInput, system, user, spokenRevisionSchema)
	}
	system, user := buildNarrationRevisionPrompt(in, flow, limits)
	return s.validateWritingAllowance(model, in.PlanningInput, system, user, NarrationSchema())
}
