package generation

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

func (s *Service) write(ctx context.Context, post PostInput, observations []Observation, model llm.ModelRef) (PostContent, error) {
	profile, err := s.profiles.ProfileForPrompt(ctx, post.UserID)
	if err != nil {
		return PostContent{}, fmt.Errorf("load voice profile: %w", err)
	}
	content, _, err := s.writeCandidate(ctx, post, profile, observations, model)
	return content, err
}

func (s *Service) writeCandidate(ctx context.Context, post PostInput, profile Profile, observations []Observation, model llm.ModelRef) (PostContent, llm.Usage, error) {
	filenames := make([]string, 0, len(post.Images))
	for _, image := range post.Images {
		filenames = append(filenames, image.Filename)
	}
	system, user := BuildWritePrompt(profile, observations, post.Memo, post.Title, filenames)
	request := llm.Request{
		System:   system,
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}},
	}
	if info, ok := s.models.Resolve(model); ok && info.StructuredOutput {
		request.JSONSchema = PostContentSchema()
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return PostContent{}, llm.Usage{}, providerCallError("글 작성", err)
	}
	content, err := ParseContent(response.Text)
	if err != nil {
		return PostContent{}, response.Usage, err
	}
	content.Blocks = ValidateBlocks(content.Blocks)
	return FilterAttachments(*content, filenames), response.Usage, nil
}
