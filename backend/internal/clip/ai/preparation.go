package ai

import (
	"encoding/json"
	"strings"

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
	in.Analyses = nil
	if nativeComposition(in) {
		// BOTH writing calls are checked, the narration against the largest
		// flow the flow call may hand it: the allowance has to hold the bigger
		// of the two requests, not the smaller one (CLIP-90, CLIP-135).
		limits := compositionLimits(s.cfg, in)
		requests := [][2]string{}
		system, user := BuildFlowPrompt(in, s.cfg.Render.FadeMS, limits)
		requests = append(requests, [2]string{system, user})
		system, user = BuildNarrationPrompt(clip.NarrationInput{PlanningInput: in, Flow: WidestFlow(s.cfg, in)}, limits)
		requests = append(requests, [2]string{system, user})
		for i, request := range requests {
			schema := json.RawMessage(nil)
			if writer.StructuredOutput {
				schema = FlowSchema()
				if i == 1 {
					schema = NarrationSchema()
				}
			}
			if err := validatePrompt(request[0], request[1], schema, llm.ExecutionTextOnly, in.Policy.InputTokenLimit()); err != nil {
				return err
			}
		}
		return nil
	}
	schema = nil
	if writer.StructuredOutput {
		schema = PlanSchema()
	}
	system, user := BuildPlanPrompt(in, s.cfg.Render.FadeMS, compositionLimits(s.cfg, in))
	return validatePrompt(system, user, schema, llm.ExecutionTextOnly, in.Policy.InputTokenLimit())
}

// widestFlow is the largest footage flow the narration request can be asked to
// carry: the cut ceiling, each cut with the longest identity a writer may mint.
// The frozen allowance is checked against that, never against the flow one
// particular generation happens to produce.
func WidestFlow(cfg Config, in clip.PlanningInput) clip.EditPlan {
	source := clip.AnalysisSource{}
	if len(in.Analyses) > 0 {
		source = in.Analyses[0].Source
	}
	flow := clip.EditPlan{Ratio: in.Ratio, DurationMS: cfg.Render.MaxDurationMS}
	length := max(1, cfg.Render.MaxDurationMS/max(1, cfg.Render.MaxCuts))
	for i := range cfg.Render.MaxCuts {
		flow.Cuts = append(flow.Cuts, clip.Cut{ID: strings.Repeat("c", 64), SourceID: source.ID, Fingerprint: source.Fingerprint,
			StartMS: i * length, EndMS: (i + 1) * length, PlaybackRatePermille: clip.RateUnitPermille})
	}
	return flow
}
