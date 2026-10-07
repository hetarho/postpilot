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
	if err := s.validateFrozenProfile(ctx, post.UserID, post.Voice.ID, &profile); err != nil {
		return WriteAnswer{}, llm.Usage{}, err
	}
	photos, videos := AttachmentNames(post.Images)
	// A snapshot frozen before the member existed carries 0 here; the prompt and the parser
	// must agree on one number, so it is resolved once.
	tagCount := resolveTagCount(post.TagCount)
	system, user := BuildWritePromptForLanguage(WritePromptInput{
		Language: post.TargetLanguage, Profile: profile, Observations: observations,
		Memo: post.Memo, Title: post.Title, Photos: photos, Videos: videos,
		TargetLength: post.TargetLength, TagCount: tagCount, Template: post.Template,
		DefaultGuidelines: post.DefaultGuidelines, Guidelines: post.Guidelines, Memories: post.Memories, QualityRules: post.QualityRules,
		FollowStoryline: post.FollowStoryline, Portraits: PhotoPortraits(post.Images, observations),
	})
	request := llm.Request{
		System:    system,
		Messages:  []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}},
		Reasoning: s.reasoning.Write,
		Stage:     llm.StageNameWrite,
		MaxTokens: s.budget.Write(post.TargetLength, post.WriteNativeEffort),
	}
	following := len(post.FollowStoryline) > 0
	if info, ok := s.models.Resolve(model); ok && info.StructuredOutput {
		request.JSONSchema = WriteAnswerSchema()
		if following {
			request.JSONSchema = WriteAlongStorylineAnswerSchema()
		}
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return WriteAnswer{}, response.Usage, providerCallError("글 작성", err)
	}
	shown := append(append([]string(nil), photos...), videos...)
	answer, err := ParseWriteAnswer(response.Text, tagCount, shown)
	if err != nil {
		return WriteAnswer{}, response.Usage, responseParseError(response, err)
	}
	if following {
		// The run followed the stored storyline and answered none: nil keeps it exactly as it
		// was, the owner's edit mark included (GEN-70, GEN-71).
		answer.Storyline = nil
	} else {
		// What the writing stage was shown is what the post reads a later attachment against.
		answer.Storyline.MadeWith = shown
	}
	answer.Content.Blocks = ValidateBlocks(answer.Content.Blocks)
	answer.Content = FilterAttachments(answer.Content, photos, videos, PhotoPortraits(post.Images, observations))
	return *answer, response.Usage, nil
}

func contentTags(content *PostContent) []string {
	if content == nil {
		return nil
	}
	return content.Tags
}

func (s *Service) profileForTopic(ctx context.Context, userID, voiceID string, target Language, topic string, tags []string) (Profile, error) {
	// 말투 없음 reads no voice: there is nothing to project (GEN-74).
	if voiceID == "" {
		return Profile{NoVoice: true}, nil
	}
	if contextual, ok := s.profiles.(TopicProfiles); ok {
		return contextual.ProfileForPromptForTopic(ctx, userID, voiceID, target, topic, tags)
	}
	return s.profiles.ProfileForPrompt(ctx, userID, voiceID, target)
}
