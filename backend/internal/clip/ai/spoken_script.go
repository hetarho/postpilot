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

const spokenScriptRule = `Spoken draft version 1: write spoken_lines as the EXACT natural-speed body script to synthesize, in the required output language. At most 32 nonempty lines, at most 500 Unicode characters per line and 2000 in total. Aim for the chosen duration at natural speed, leaving enabled intro/outro time; a bound is not a target to fill. Text cannot predict the synthesized duration: measured audio supplies it later. Do not write video cuts or visible captions. Intro/outro slots are visible text, never spoken_lines. Facts must come from the supplied observations and authored facts. No tools, extra synthesis, voice imitation, music or audio rate changes.
`

var frozenSpokenScriptParseSchema = withoutRequirement(withoutRequirement(spokenScriptSchema, "storyline"), "region_slots")

func spokenScriptContract(in clip.PlanningInput) []byte {
	if in.FollowStoryline != nil {
		return spokenRevisionSchema
	}
	return spokenScriptSchema
}

func BuildSpokenScriptPrompt(in clip.PlanningInput, limits composition.Limits) (string, string) {
	_, user := BuildStorylinePrompt(clip.StorylineInput{PlanningInput: in}, limits)
	prompt := storylineContentRule + regionSlotRule
	mode := "spoken-script"
	if in.FollowStoryline != nil {
		mode = "spoken-script-follow-storyline"
		var data map[string]any
		_ = json.Unmarshal([]byte(user), &data)
		data["following_storyline"] = storylinePayload(*in.FollowStoryline)
		// These slots already belong to the reviewed story. New slot words would
		// be discarded with a replacement story, so only retained words are data.
		delete(data, "intro_outro")
		if shown := regionTextPayload(in, limits); len(shown) > 0 {
			data["intro_outro"] = shown
		}
		user = promptJSON(data)
		prompt = "Follow following_storyline's scenes, order and meaning. Return ONLY spoken_lines; do not generate a replacement storyline or region_slots. The reviewed story and its visible slot words stay unchanged.\n"
	}
	contract := compactContract(spokenScriptContract(in))
	if in.Policy.StructuredOutput {
		contract = "Use the supplied response schema."
		if in.FollowStoryline == nil {
			contract += " Additional bounds: region_slots at most 30, each text and short_text at most 500 characters."
		}
	}
	return videoWritingContract(in.Language) + prompt + spokenScriptRule + responseContract + contract + videoGuidelineBlock(videoGuidelinesFor(in.Guidelines, mode)), user
}
func parseSpokenScript(cfg Config, in clip.StorylineInput, raw string) (clip.SpokenDraft, error) {
	var wire struct {
		Storyline   []flowStorylineJSON `json:"storyline"`
		RegionSlots []regionSlotJSON    `json:"region_slots"`
		Lines       []string            `json:"spoken_lines"`
	}
	contract := spokenScriptSchema
	if in.FollowStoryline != nil {
		contract = frozenSpokenScriptParseSchema
	}
	if err := decode(raw, cfg.MaxResponseBytes, readShape(contract), &wire); err != nil {
		return clip.SpokenDraft{}, err
	}
	if in.FollowStoryline != nil {
		return clip.NewSpokenDraft(wire.Lines, in.FollowStoryline)
	}
	data, _ := json.Marshal(storylineJSON{Storyline: wire.Storyline, RegionSlots: wire.RegionSlots})
	storyline, err := parseStoryline(cfg, in, string(data))
	if err != nil {
		return clip.SpokenDraft{}, err
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
	mode := "spoken-script"
	if in.FollowStoryline != nil {
		mode = "spoken-script-follow-storyline"
	}
	request := llm.Request{Composition: clipComposition(mode, system, user, in.Guidelines, planningSourceRefs(in)), System: system, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}}, Stage: llm.StageNameWrite, Reasoning: input.Policy.Reasoning, DisableReasoning: input.Policy.DisableReasoning, MaxTokens: input.Policy.CompletionTokens, Execution: execution}
	if execution.Call.StructuredOutput {
		if !info.StructuredOutput {
			return clip.SpokenDraft{}, llm.Usage{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = structuralSchema(spokenScriptContract(in))
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
