package ai

import (
	"encoding/json"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// Refuse oversized known inputs before a hold or observation. Facts returned by
// later calls remain bounded separately; no input is silently shortened to fit.
func (s *Service) ValidatePreparation(observe llm.ModelRef, in clip.PlanningInput, sources []clip.AnalysisSource) error {
	if err := validateSettings(s.cfg, in); err != nil {
		return inputFailure(err, "input_settings", 0)
	}
	if err := clip.ValidateAnalysisSources(s.cfg.Analysis, sources); err != nil {
		return inputFailure(err, "input_sources", 0)
	}
	observer, err := s.model(observe, llm.StageNameObserve)
	if err != nil {
		return err
	}
	writer, err := s.model(in.Policy.Ref, llm.StageNameWrite)
	if err != nil {
		return err
	}
	var schema json.RawMessage
	if observer.StructuredOutput {
		schema = ChunkSchema()
	}
	for _, source := range sources {
		system, user := BuildObservePrompt(clip.ChunkInput{Source: source, DurationMS: min(s.cfg.Analysis.ChunkMS, source.Info.DurationMS), Language: in.Language})
		if err := validatePrompt(system, user, schema, llm.ExecutionInlineStatic, llm.ClipInputUnits); err != nil {
			return err
		}
	}
	schema = nil
	if writer.StructuredOutput {
		schema = PlanSchema()
		if nativeComposition(in) {
			schema = CompositionPlanSchema()
		}
	}
	in.Analyses = nil
	system, user := BuildPlanPrompt(in, s.cfg.Render.FadeMS)
	if err := validatePrompt(system, user, schema, llm.ExecutionTextOnly, in.Policy.InputTokenLimit()); err != nil {
		return err
	}
	return nil
}
