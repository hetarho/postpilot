package generation

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

func (s *Service) write(ctx context.Context, post PostInput, observations []Observation, model llm.ModelRef) (WriteAnswer, error) {
	if !post.TargetLanguage.Valid() {
		return WriteAnswer{}, ErrLanguageRequired
	}
	profile, err := s.profileForTopic(ctx, post.UserID, post.Voice.ID, post.TargetLanguage, post.Title+" "+post.Memo, contentTags(post.Content))
	if err != nil {
		return WriteAnswer{}, fmt.Errorf("load voice profile: %w", err)
	}
	answer, _, err := s.writeCandidate(ctx, post, profile, observations, model)
	return answer, err
}

func (s *Service) writeCandidate(ctx context.Context, post PostInput, profile Profile, observations []Observation, model llm.ModelRef) (WriteAnswer, llm.Usage, error) {
	photos, videos := AttachmentNames(post.Images)
	// A snapshot frozen before the member existed carries 0 here; the prompt and the parser
	// must agree on one number, so it is resolved once.
	tagCount := resolveTagCount(post.TagCount)
	system, user := BuildWritePromptForLanguage(WritePromptInput{
		Language: post.TargetLanguage, Profile: profile, Observations: observations,
		Memo: post.Memo, Title: post.Title, Photos: photos, Videos: videos,
		TargetLength: post.TargetLength, TagCount: tagCount, Template: post.Template,
		Guidelines: post.Guidelines, Memories: post.Memories, QualityRules: post.QualityRules, FieldPhrases: post.FieldPhrases,
	})
	request := llm.Request{
		System:    system,
		Messages:  []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}},
		Reasoning: s.reasoning.Write,
		Stage:     llm.StageNameWrite,
		MaxTokens: s.budget.Write(post.TargetLength, post.WriteNativeEffort),
	}
	if info, ok := s.models.Resolve(model); ok && info.StructuredOutput {
		request.JSONSchema = WriteAnswerSchema()
		if len(post.FieldPhrases) > 0 {
			request.JSONSchema = WriteAnswerReplacementsSchema()
		}
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return WriteAnswer{}, response.Usage, providerCallError("글 작성", err)
	}
	answer, err := ParseWriteAnswer(response.Text, tagCount)
	if err != nil {
		return WriteAnswer{}, response.Usage, responseParseError(response, err)
	}
	answer.Content.Blocks = ValidateBlocks(answer.Content.Blocks)
	// Slot resolution runs LAST, after the attachment filter: a slot block carries no file,
	// so filtering first keeps that pass unaware of templates entirely.
	answer.Content = ApplyTemplateSlots(FilterAttachments(answer.Content, photos, videos), post.Template)
	// Judged against the FINAL content, which is what gets stored: an index names what stands
	// there now. A run that froze no phrases asked for none, so whatever came back is ignored.
	if len(post.FieldPhrases) > 0 {
		answer.Replacements = ValidateReplacements(answer.Replacements, answer.Content, post.FieldPhrases)
	} else {
		answer.Replacements = nil
	}
	return *answer, response.Usage, nil
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
