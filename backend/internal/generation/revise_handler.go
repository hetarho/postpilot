package generation

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

// Revise handles one durable revise job. It reloads both the current content and the
// complete voice profile for every pass; no earlier job's prompt state is reused.
func (s *Service) Revise(ctx context.Context, job RevisionJob, progress Progress) error {
	payload, err := parseRevisionPayload(job.Payload)
	if err != nil {
		return err
	}
	post, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("load revision input: %w", err)
	}
	if post.Content == nil {
		return ErrRevisionContentRequired
	}
	profile, err := s.profileForTopic(ctx, job.UserID, post.Title+" "+post.Memo, contentTags(post.Content))
	if err != nil {
		return fmt.Errorf("load voice profile: %w", err)
	}
	model, ok := parseModelRef(job.WriteModel)
	if !ok {
		return ErrWriteModelRequired
	}
	filenames := make([]string, 0, len(post.Images))
	for _, image := range post.Images {
		filenames = append(filenames, image.Filename)
	}
	system, user := BuildRevisePrompt(profile, *post.Content, filenames, payload.Instruction, post.TargetLength)
	request := llm.Request{
		System:    system,
		Messages:  []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}},
		Reasoning: s.reasoning.Write,
	}
	if info, found := s.models.Resolve(model); found && info.StructuredOutput {
		request.JSONSchema = PostContentSchema()
	}
	progress("write", 0, 1)
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return providerCallError("글 수정", err)
	}
	content, err := ParseContent(response.Text)
	if err != nil {
		return responseParseError(response, err)
	}
	content.Blocks = ValidateBlocks(content.Blocks)
	// Attachments can change while the provider call is in flight. Filter against a
	// fresh snapshot so a concurrently deleted photo can never become a dangling IMAGE
	// reference in the persisted draft.
	current, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("reload revision attachments: %w", err)
	}
	currentFilenames := make([]string, 0, len(current.Images))
	for _, image := range current.Images {
		currentFilenames = append(currentFilenames, image.Filename)
	}
	filtered := FilterAttachments(*content, currentFilenames)
	if err := s.posts.SetGeneratedContent(ctx, current.UserID, current.Slug, filtered); err != nil {
		return fmt.Errorf("persist revised content: %w", err)
	}
	progress("write", 1, 1)
	return nil
}
