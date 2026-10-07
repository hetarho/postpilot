package generation

import (
	"context"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	postdomain "github.com/postpilot/backend/internal/post"
	"unicode/utf8"
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
	ctx, finishCapture := s.beginRequestCapture(ctx, job.ID, post)
	defer finishCapture()
	// The same backstop as the generate handler's.
	if post.Published {
		return ErrPostPublished
	}
	if post.Content == nil {
		return ErrRevisionContentRequired
	}
	voiceID, err := frozenVoice(post, job.VoiceID)
	if err != nil {
		return err
	}
	var profile Profile
	if frozen := decodeProfile(payload.Profile); frozen != nil {
		if err := s.validateFrozenProfile(ctx, job.UserID, voiceID, frozen); err != nil {
			return err
		}
		profile = *frozen
	} else {
		profile, err = s.profileForTopic(ctx, job.UserID, voiceID, payload.ContentLanguage, post.Title+" "+post.Memo, contentTags(post.Content))
		if err != nil {
			return fmt.Errorf("load voice profile: %w", err)
		}
	}
	model, ok := parseModelRef(job.WriteModel)
	if !ok {
		return ErrWriteModelRequired
	}
	tagCount := resolveTagCount(payload.TagCount)
	request, sources := s.prepareRevisionRequest(post, profile, payload, model)
	progress("write", 0, 1)
	response, err := s.completePostRequest(ctx, model, request, post.Images)
	if err != nil {
		return providerCallError("글 수정", err)
	}
	content, candidates, err := ParseRevisionContentWithOrigins(response.Text, tagCount, *post.Content)
	if err != nil {
		return responseParseError(response, err)
	}
	rawContent := *content
	content.Blocks = ValidateBlocks(content.Blocks)
	// Attachments can change while the provider call is in flight. Filter against a
	// fresh snapshot so a concurrently deleted photo can never become a dangling IMAGE
	// reference in the persisted draft.
	current, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("reload revision attachments: %w", err)
	}
	// The voice is rechecked on the fresh snapshot too: a reassignment or deletion that
	// slipped in during the provider call must not persist output into the wrong profile.
	if _, err := frozenVoice(current, voiceID); err != nil {
		return err
	}
	if err := s.validateFrozenProfile(ctx, job.UserID, voiceID, decodeProfile(payload.Profile)); err != nil {
		return err
	}
	// The FRESH attachment snapshot, both kinds: a revision is filtered against what the post
	// carries now, not against what it carried when the job was queued.
	currentPhotos, currentVideos := AttachmentNames(current.Images)
	filtered := FilterAttachments(*content, currentPhotos, currentVideos, PhotoPortraits(current.Images, current.Observations))
	// nil keeps the post's nouns: a revision has no nouns answer (GEN-55), so what the last
	// generation said stands.
	var publishErr error
	if payload.OriginProtocolVersion == OriginProtocolVersion {
		final, mapping := NormalizeContentWithOriginMap(rawContent, currentPhotos, currentVideos, PhotoPortraits(current.Images, current.Observations))
		mapped, _ := RemapOriginCandidates(rawContent, final, mapping, candidates)
		var identity []postdomain.OriginResultIdentity
		if post.ContentOriginIdentity != nil {
			identity = append(identity, *post.ContentOriginIdentity)
		}
		prior := cloneOriginReview(post.ContentOrigins)
		if prior != nil {
			prior.Sources = OriginSourcesWithAttachments(prior.Sources, currentPhotos, currentVideos)
		}
		sources = OriginSourcesWithAttachments(sources, currentPhotos, currentVideos)
		origins := PreserveRevisionOrigins(*post.Content, prior, final, sources, mapped, identity...)
		var publishedIdentity postdomain.OriginResultIdentity
		publishedIdentity, publishErr = s.originPosts.PublishGeneratedResult(ctx, current.UserID, current.Slug, OriginPostCompletion{Content: final, Language: payload.ContentLanguage, Origins: origins, ExpectedContentRevision: post.ContentRevision})
		if publishErr == nil {
			bindRequestCaptureResult(ctx, publishedIdentity)
		}
	} else {
		publishErr = s.posts.SetGeneratedContent(ctx, current.UserID, current.Slug, filtered, payload.ContentLanguage, nil)
	}
	if err := publishErr; err != nil {
		return fmt.Errorf("persist revised content: %w", err)
	}
	s.recordGuidelineCandidate(ctx, current.UserID, current.Slug, payload.Instruction)
	progress("write", 1, 1)
	return nil
}

// contentChars is roughly how long the content a revision must re-emit is. It counts the
// prose a model actually rewrites — a title, a summary and block text — rather than the JSON
// envelope, which the budget's per-character ratio already allows for.
func contentChars(content *PostContent) int {
	if content == nil {
		return 0
	}
	chars := utf8.RuneCountInString(content.Title) + utf8.RuneCountInString(content.Summary)
	for _, block := range content.Blocks {
		chars += utf8.RuneCountInString(block.Content) + utf8.RuneCountInString(block.Caption)
		for _, item := range block.Items {
			chars += utf8.RuneCountInString(item)
		}
	}
	return chars
}

// prepareRevisionRequest performs no reads or writes; explicit execution and
// current configuration inspection assemble the same stage contract.
func (s *Service) prepareRevisionRequest(post PostInput, profile Profile, payload revisionPayloadJSON, model llm.ModelRef) (llm.Request, []postdomain.OriginSource) {
	filenames := make([]string, 0, len(post.Images))
	for _, image := range post.Images {
		filenames = append(filenames, image.Filename)
	}
	// The brief, the 지침 and the tag count come from the frozen payload, never from the
	// live rows, exactly as the generate handler does it.
	tagCount := resolveTagCount(payload.TagCount)
	var sources []postdomain.OriginSource
	photos, _ := AttachmentNames(post.Images)
	request := composeRevisionRequest(payload.ContentLanguage, profile, *post.Content, filenames, photos, PhotoPortraits(post.Images, post.Observations), payload.Instruction, post.TargetLength, tagCount, decodeTemplate(payload.Template), FrozenGuidelines{Defaults: payload.DefaultGuidelines, Stock: decodeStockGuidelines(payload.StockGuidelines), Owner: payload.Guidelines})
	if payload.OriginProtocolVersion == OriginProtocolVersion {
		sources = WritingOriginSources(WritePromptInput{Template: decodeTemplate(payload.Template)}, false)
		catalog := originCatalog{sources: sources}
		for _, source := range sources {
			catalog.chars += utf8.RuneCountInString(source.Text)
		}
		catalog.add("current.edit", postdomain.OriginSourceOwnerEdit, payload.Instruction, "", true)
		sources = catalog.sources
		_, videos := AttachmentNames(post.Images)
		sources = catalogWithPriorContent(sources, *post.Content, post.ContentOrigins, post.ContentOriginIdentity, photos, videos)
		prior := validatedContentOriginContext(*post.Content, post.ContentOrigins, post.ContentOriginIdentity, photos, videos)
		request = appendOriginRequest(request, sources, priorOriginProjection(prior))
	}
	request.Reasoning = s.reasoning.Write
	request.MaxTokens = s.budget.Revise(contentChars(post.Content), post.TargetLength, payload.WriteNativeEffort)
	if payload.OriginProtocolVersion == OriginProtocolVersion {
		request.MaxTokens = payload.CompletionTokens
	}
	schema := PostContentSchema()
	if payload.OriginProtocolVersion == 0 {
		schema = LegacyPostContentSchema()
	}
	setOriginOutput(&request, "PostContent", schema, payload.OriginProtocolVersion)
	if info, found := s.models.Resolve(model); found && info.StructuredOutput {
		request.JSONSchema = schema
	}
	return request, sources
}
