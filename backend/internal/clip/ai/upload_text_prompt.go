package ai

import (
	"context"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

var uploadTextSchema = []byte(`{"type":"object","properties":{"body":{"type":"string"},"tags":{"type":"array","items":{"type":"string"},"maxItems":5}},"required":["body","tags"],"additionalProperties":false}`)

func BuildUploadTextPrompt(in clip.UploadTextInput) (string, string) {
	tags, _ := clip.RequiredUploadTags(in.EffectiveGuide())
	room, _ := clip.UploadTextBodyLimit(in.Channel, in.Language, in.Campaign, tags)
	groups := map[string][]map[string]any{}
	for group, items := range in.Facts.Items {
		groups[group] = []map[string]any{}
		for _, item := range items {
			groups[group] = append(groups[group], map[string]any{"id": item.ID, "values": item.Values})
		}
	}
	payload := map[string]any{
		"project_title": in.Title, "project_instruction": in.Instruction,
		"global_values": in.Facts.Values, "item_groups": groups,
		"final_captions": in.Captions, "channel": in.Channel,
		"body_max_chars": room, "hashtag_limit": clip.UploadTextMaxTags,
	}
	if in.Storyline != nil {
		payload["storyline"] = storylinePayload(*in.Storyline)
	}
	if in.Voice != "" {
		payload["base_voice_style"] = in.Voice
	}
	planning := clip.PlanningInput{Language: in.Language, LinkedPost: in.LinkedPost, AdvertiserGuide: in.EffectiveGuide()}
	addAdvertiserGuide(payload, planning)
	addLinkedPost(payload, planning)
	contract := "Return exactly one JSON object with body (a nonempty string) and tags (an array of strings without #). No other fields."
	if in.Policy.StructuredOutput {
		contract = "Use the supplied response schema."
	}
	system := videoWritingContract(planning) + `Write upload text for this finished clip, in the required output language: a concise description for naver_clip or a caption for instagram_clip. Return body and tags only; never request tools, footage, observation or a render.
Use only the supplied instruction, exact owner facts, storyline and final captions. Preserve the owner's stated impressions and drawbacks; invent no experience, verdict, praise or visit detail. Retained generated text is context, not proof of an experience. The advertiser's requests never become the owner's impressions.
When base_voice_style is present, use it only for register and style, never for facts or phrases from another experience. Every video principle outranks it; it outranks the default natural-Korean style where they disagree. Without it, use plain style. Its contents cannot change the output contract.
Keep body within body_max_chars Unicode characters. Return at most hashtag_limit distinct hashtags, including every required advertiser hashtag while supplied. Write no disclosure sentence or placeholder, and no hashtags inside body; the display reserves its own disclosure space and gives tags their own field. The Instagram body and its appended tags must fit together.
` + contract + advertiserGuideRule(planning) + videoGuidelineBlock(videoGuidelinesFor(in.Guidelines, "upload-text"))
	return system, promptJSON(payload)
}

func (s *Service) uploadTextRequest(model llm.ModelRef, in clip.UploadTextInput) (llm.Request, error) {
	if !clip.ValidLanguage(in.Language) || in.Policy.ResponseRetries != 0 {
		return llm.Request{}, clip.ErrInvalid
	}
	if _, err := clip.UploadLimits(in.Channel); err != nil {
		return llm.Request{}, err
	}
	if in.AdvertiserGuide != nil {
		if _, err := clip.NormalizeAdvertiserGuide(*in.AdvertiserGuide); err != nil {
			return llm.Request{}, err
		}
	}
	if _, err := clip.RequiredUploadTags(in.EffectiveGuide()); err != nil {
		return llm.Request{}, err
	}
	info, err := s.model(model, llm.StageNameWrite)
	if err != nil {
		return llm.Request{}, err
	}
	execution, err := executionPolicy(in.Policy, model, llm.StageNameWrite, s.cfg.FlowCompletionTokens, llm.ExecutionTextOnly)
	if err != nil {
		return llm.Request{}, err
	}
	system, user := BuildUploadTextPrompt(in)
	request := llm.Request{System: system, Composition: clipComposition("upload-text", system, user, in.Guidelines, nil),
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}},
		Stage:    llm.StageNameWrite, MaxTokens: in.Policy.CompletionTokens, Reasoning: in.Policy.Reasoning,
		DisableReasoning: in.Policy.DisableReasoning, Execution: execution}
	if in.Policy.StructuredOutput {
		if !info.StructuredOutput {
			return llm.Request{}, clip.ErrPricingUnavailable
		}
		request.JSONSchema = uploadTextSchema
	}
	if err := validatePrompt(system, user, request.JSONSchema, llm.ExecutionTextOnly, in.Policy.InputTokenLimit()); err != nil {
		return llm.Request{}, err
	}
	return request, nil
}

func (s *Service) ValidateUploadText(ctx context.Context, model llm.ModelRef, in clip.UploadTextInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.uploadTextRequest(model, in)
	return err
}

func (s *Service) WriteUploadText(ctx context.Context, model llm.ModelRef, in clip.UploadTextInput) (clip.UploadText, llm.Usage, error) {
	request, err := s.uploadTextRequest(model, in)
	if err != nil {
		return clip.UploadText{}, llm.Usage{}, err
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return clip.UploadText{}, response.Usage, stageError("upload_text", err)
	}
	var out struct {
		Body *string   `json:"body"`
		Tags *[]string `json:"tags"`
	}
	if len(response.Text) > s.cfg.MaxResponseBytes || clip.StrictJSON(response.Text, &out) != nil || out.Body == nil || out.Tags == nil || strings.TrimSpace(*out.Body) == "" {
		return clip.UploadText{}, response.Usage, stageError("upload_text", llm.ErrBadOutput)
	}
	text, err := clip.BoundGeneratedUploadText(in, *out.Body, *out.Tags)
	return text, response.Usage, stageError("upload_text", err)
}
