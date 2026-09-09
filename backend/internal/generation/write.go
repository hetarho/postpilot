package generation

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

func (s *Service) write(ctx context.Context, post PostInput, observations []Observation, model llm.ModelRef) (PostContent, error) {
	if !post.TargetLanguage.Valid() {
		return PostContent{}, ErrLanguageRequired
	}
	profile, err := s.profileForTopic(ctx, post.UserID, post.Voice.ID, post.TargetLanguage, post.Title+" "+post.Memo, contentTags(post.Content))
	if err != nil {
		return PostContent{}, fmt.Errorf("load voice profile: %w", err)
	}
	content, _, err := s.writeCandidate(ctx, post, profile, observations, model)
	return content, err
}

func (s *Service) writeCandidate(ctx context.Context, post PostInput, profile Profile, observations []Observation, model llm.ModelRef) (PostContent, llm.Usage, error) {
	photos, videos := AttachmentNames(post.Images)
	// A snapshot frozen before the member existed carries 0 here; the prompt and the parser
	// must agree on one number, so it is resolved once.
	tagCount := resolveTagCount(post.TagCount)
	system, user := BuildWritePromptForLanguage(post.TargetLanguage, profile, observations, post.Memo, post.Title, photos, videos, post.TargetLength, tagCount, post.Template, post.Guidelines)
	request := llm.Request{
		System:    system,
		Messages:  []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}},
		Reasoning: s.reasoning.Write,
		Stage:     llm.StageNameWrite,
		MaxTokens: s.budget.Write(post.TargetLength, post.WriteNativeEffort),
	}
	if info, ok := s.models.Resolve(model); ok && info.StructuredOutput {
		request.JSONSchema = PostContentSchema()
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return PostContent{}, response.Usage, providerCallError("글 작성", err)
	}
	content, err := ParseContent(response.Text, tagCount)
	if err != nil {
		return PostContent{}, response.Usage, responseParseError(response, err)
	}
	content.Blocks = ValidateBlocks(content.Blocks)
	// Slot resolution runs LAST, after the attachment filter: a slot block carries no file,
	// so filtering first keeps that pass unaware of templates entirely.
	return ApplyTemplateSlots(FilterAttachments(*content, photos, videos), post.Template), response.Usage, nil
}

func contentTags(content *PostContent) []string {
	if content == nil {
		return nil
	}
	return content.Tags
}

func (s *Service) profileForTopic(ctx context.Context, userID, voiceID string, target Language, topic string, tags []string) (Profile, error) {
	if contextual, ok := s.profiles.(TopicProfiles); ok {
		return contextual.ProfileForPromptForTopic(ctx, userID, voiceID, target, topic, tags)
	}
	return s.profiles.ProfileForPrompt(ctx, userID, voiceID, target)
}
