package ai

import (
	"context"
	_ "embed"
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
)

//go:embed schemas/spoken-script.schema.json
var spokenScriptSchema []byte

const spokenScriptRule = `Spoken draft version 1: write spoken_lines as the EXACT natural-speed body script to synthesize, in the project's language. At most 32 nonempty lines, at most 500 Unicode characters per line and 2000 in total. Aim for the chosen duration at natural speed, leaving enabled intro/outro time; a bound is not a target to fill. This script and its storyline precede footage selection. Do not write video cuts or visible captions. Intro/outro slots are visible text, never spoken_lines. Facts must come from the supplied observations and authored facts. If following_storyline is supplied, keep its scenes, order and meaning. No tools, extra synthesis, voice imitation, music or audio rate changes. Return the closed response contract including spoken_lines.
`

func BuildSpokenScriptPrompt(in clip.PlanningInput, limits composition.Limits) (string, string) {
	_, user := BuildStorylinePrompt(clip.StorylineInput{PlanningInput: in}, limits)
	if in.FollowStoryline != nil {
		var data map[string]any
		_ = json.Unmarshal([]byte(user), &data)
		data["following_storyline"] = storylinePayload(*in.FollowStoryline)
		user = promptJSON(data)
	}
	return storylinePrompt + spokenScriptRule + compactContract(spokenScriptSchema) + videoGuidelineBlock(in.Guidelines), user
}
func parseSpokenScript(cfg Config, in clip.StorylineInput, raw string) (clip.SpokenDraft, error) {
	var wire struct {
		Storyline   []flowStorylineJSON `json:"storyline"`
		RegionSlots []regionSlotJSON    `json:"region_slots"`
		Lines       []string            `json:"spoken_lines"`
	}
	if err := decode(raw, cfg.MaxResponseBytes, readShape(spokenScriptSchema), &wire); err != nil {
		return clip.SpokenDraft{}, err
	}
	data, _ := json.Marshal(storylineJSON{Storyline: wire.Storyline, RegionSlots: wire.RegionSlots})
	storyline, err := parseStoryline(cfg, in, string(data))
	if err != nil {
		return clip.SpokenDraft{}, err
	}
	if in.FollowStoryline != nil {
		storyline = *in.FollowStoryline
	}
	return clip.NewSpokenDraft(wire.Lines, &storyline)
}

func (s *Service) SpokenScript(ctx context.Context, model llm.ModelRef, in clip.PlanningInput) (clip.SpokenDraft, llm.Usage, error) {
	input := clip.StorylineInput{PlanningInput: in}
	if err := ctx.Err(); err != nil {
		return clip.SpokenDraft{}, llm.Usage{}, err
	}
	if !nativeComposition(input.PlanningInput) || len(input.Analyses) == 0 ||
		input.Request != "" && (input.Current == nil || !within(input.Request, 1, s.cfg.Template.InstructionChars)) {
		return clip.SpokenDraft{}, llm.Usage{}, stageError("script", clip.ErrInvalid)
	}
	if err := validateInput(s.cfg, input.PlanningInput); err != nil {
		return clip.SpokenDraft{}, llm.Usage{}, err
	}
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return clip.SpokenDraft{}, llm.Usage{}, stageError("script", err)
	}
	execution, err := executionPolicy(input.Policy, model, llm.StageNameWrite, s.cfg.FlowCompletionTokens, llm.ExecutionTextOnly)
	if err != nil {
		return clip.SpokenDraft{}, llm.Usage{}, err
	}
	system, user := BuildSpokenScriptPrompt(in, compositionLimits(s.cfg, in))
	request := llm.Request{System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.SpokenDraft{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = structuralSchema(spokenScriptSchema)
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, input.Policy.InputTokenLimit()); err != nil {
		return clip.SpokenDraft{}, llm.Usage{}, err
	}
	var retained clip.SpokenDraft
	result, usage, err := completeValidated(ctx, s, model, request, user, input.Policy, func(raw string) (clip.SpokenDraft, error) {
		d, e := parseSpokenScript(s.cfg, input, raw)
		if d.Version == 1 && len(d.Narration.Segments) > 0 {
			retained = d
		}
		return d, e
	})
	if err != nil && retained.Version == 1 {
		result = retained
	}
	return result, usage, stageError("script", err)
}
