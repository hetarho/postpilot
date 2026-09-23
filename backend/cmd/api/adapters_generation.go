package main

import (
	"context"
	"errors"
	"fmt"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/storage"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
	"google.golang.org/protobuf/encoding/protojson"
)

type generationTemplates struct{ service *template.Service }

// guidelineTemplates hands the guideline context the account's template directory: the ids it
// must prove are owned before saving a scope, and the names it projects when listing.
type generationGuidelines struct{ service *guideline.Service }

func (a generationGuidelines) ForPrompt(ctx context.Context, userID string, templateID *string) ([]string, error) {
	return a.service.ForPrompt(ctx, userID, templateID)
}

// generationMemories hands the generation context the memory context's retrieval. What
// crosses is the post's own words and, coming back, TEXTS — the generation context never
// learns that a memory has a kind, tags or an id, and the memory context never learns what a
// job is. It is consulted once per enqueue, and only for a post that opted in.
type generationMemories struct{ service *memory.Service }

func (a generationMemories) ForPost(ctx context.Context, userID string, keyParts []string) ([]string, error) {
	return a.service.TextsForPost(ctx, userID, keyParts)
}

// generationCandidates hands the generation context the candidate recorder. The instruction
// crosses as opaque text: the guideline context records the user's sentence without learning
// what a revision job is, and nothing on either side reads it with a model.
type generationCandidates struct{ service *guideline.Service }

func (a generationCandidates) Record(ctx context.Context, userID, postSlug, instruction string) error {
	return a.service.RecordCandidate(ctx, userID, postSlug, instruction)
}

// postCandidateLinks lets post deletion drop the link without the post context learning what
// a guideline candidate is: it hands over the account and the slug, and nothing comes back.
func (a generationTemplates) RenderedFor(ctx context.Context, userID, templateID string, filenames []string, answers []generation.TemplateAnswer) (generation.TemplateBrief, bool, error) {
	owned := make([]template.Answer, 0, len(answers))
	for _, answer := range answers {
		owned = append(owned, template.Answer{Label: answer.Label, Text: answer.Text, Enabled: answer.Enabled})
	}
	rendered, ok, err := a.service.RenderedFor(ctx, userID, templateID, filenames, owned)
	if err != nil || !ok {
		return generation.TemplateBrief{}, false, err
	}
	slots := make([]generation.TemplateSlot, 0, len(rendered.Slots))
	for _, slot := range rendered.Slots {
		slots = append(slots, generation.TemplateSlot{Kind: string(slot.Kind), Label: slot.Label})
	}
	rows := make([]generation.TemplatePhotoRow, 0, len(rendered.Rows))
	for _, row := range rendered.Rows {
		rows = append(rows, generation.TemplatePhotoRow{Count: row.Count, Filenames: row.Filenames})
	}
	facts := make([]generation.TemplateFact, 0, len(rendered.Facts))
	for _, fact := range rendered.Facts {
		facts = append(facts, generation.TemplateFact{Label: fact.Label, Value: fact.Value})
	}
	return generation.TemplateBrief{
		Name: rendered.Name, Body: rendered.Body, Slots: slots, Rows: rows, Facts: facts, TitleArea: rendered.TitleArea,
	}, true, nil
}

// experimentVoices adapts the directory for the experiment context: only an owned, active
// voice may start or retry a comparison in its name.
type generationModels struct{ registry meteredRegistry }

type generationImages struct{ bucket *storage.Bucket }

func (a generationImages) Read(ctx context.Context, key string) ([]byte, error) {
	return a.bucket.ReadObject(ctx, key)
}

func (a generationModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return a.registry.Lookup(ref)
}

func (a generationModels) Complete(ctx context.Context, ref llm.ModelRef, request llm.Request) (llm.Response, error) {
	return a.registry.Complete(ctx, ref, request)
}

type generationProfiles struct{ service *voice.Service }

func (a generationProfiles) ProfileForPrompt(ctx context.Context, userID, voiceID string, target generation.Language) (generation.Profile, error) {
	return a.ProfileForPromptForTopic(ctx, userID, voiceID, target, "", nil)
}

func (a generationProfiles) ProfileForPromptForTopic(ctx context.Context, userID, voiceID string, target generation.Language, topic string, tags []string) (generation.Profile, error) {
	profile, err := a.service.PromptProfileForTopicAndLanguage(ctx, userID, voiceID, voice.Language(target), topic, tags)
	return generation.Profile{Styleguide: profile.Styleguide, ActiveRules: profile.ActiveRules, Excerpts: profile.Excerpts, Rules: profile.ManualRules, EndingMaxConsecutive: a.service.EndingMaxConsecutive(), SourceLanguage: generation.Language(profile.SourceLanguage), TargetLanguage: generation.Language(profile.TargetLanguage), Portable: profile.Portable}, generationVoiceError(err)
}

// generationVersionSamples adapts at the boundary the way generationProfiles does. The wire
// format is the PostContent message's own protojson, so the voice context can keep the value as
// opaque text and the voice RPC edge can hand the client back exactly the message it already
// decodes -- one schema, defined in the proto, rather than a second JSON shape maintained here.
type generationVersionSamples struct{ service *voice.Service }

func (a generationVersionSamples) RecordVersionSample(ctx context.Context, userID, voiceID string, content generation.PostContent) error {
	encoded, err := protojson.Marshal(generationContentProto(content))
	if err != nil {
		return fmt.Errorf("encode voice version sample: %w", err)
	}
	return generationVoiceError(a.service.RecordVersionSample(ctx, userID, voiceID, string(encoded)))
}

func generationContentProto(content generation.PostContent) *postpilotv1.PostContent {
	out := &postpilotv1.PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		out.Blocks = append(out.Blocks, &postpilotv1.Block{
			// The domain's block type strings ARE the proto enum's value names, so the
			// generated name table is the mapping. An unknown name yields UNSPECIFIED, which
			// is the same thing every other mapper in the tree does with one.
			Type:    postpilotv1.BlockType(postpilotv1.BlockType_value[string(block.Type)]),
			Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return out
}

type generationRules struct{ service *voice.Service }

func (a generationRules) AppendRule(ctx context.Context, userID, voiceID, line string) error {
	return generationVoiceError(a.service.AppendRule(ctx, userID, voiceID, line))
}

func generationVoiceError(err error) error {
	switch {
	case errors.Is(err, voice.ErrVoiceDeleted):
		return generation.ErrVoiceDeleted
	case errors.Is(err, voice.ErrVoiceNotFound), errors.Is(err, voice.ErrVoiceRequired):
		return generation.ErrVoiceRequired
	default:
		return err
	}
}

type generationPosts struct{ service *post.Service }

func (a generationPosts) AttachedImages(ctx context.Context, userID, slug string) (generation.PostInput, error) {
	found, err := a.service.AttachedImages(ctx, userID, slug)
	if err != nil {
		return generation.PostInput{}, generationPostError(err)
	}
	input := generation.PostInput{
		Slug: found.Slug, UserID: found.UserID, Title: found.Title, Memo: found.Memo,
		Voice:          generation.VoiceRef{ID: found.Voice.ID, Name: found.Voice.Name, Deleted: found.Voice.Deleted, SourceLanguage: generation.Language(found.Voice.SourceLanguage)},
		TargetLanguage: generation.Language(found.TargetLanguage),
		// The id, never the brief: only the enqueue resolves it, and only through the template
		// context's own port. Dropping it here is what would make the whole feature a silent
		// no-op — every prompt would be built as if no post ever had a 템플릿.
		TemplateID:   found.TemplateID,
		TargetLength: found.TargetLength,
		TagCount:     found.TagCount,
		// The opt-in, never the memories: like TemplateID, only the enqueue resolves it, and
		// only through the memory context's own port (MEM-18, MEM-19).
		UseMemory: found.UseMemory,
		Images:    make([]generation.Image, 0, len(found.Images)+len(found.Videos)),
		// The stored contact sheet, read here so the ENQUEUE can decide what to reuse. It
		// was write-only from this context's point of view before change 21, which is why
		// every retry re-paid for eyesight the post already had.
		Observations: make([]generation.Observation, 0, len(found.Observations)),
		// The post's own answers to the template's data fields. Like TemplateID they are
		// read here and resolved only by the enqueue, through the template context's port.
		TemplateAnswers: make([]generation.TemplateAnswer, 0, len(found.TemplateAnswers)),
	}
	for _, answer := range found.TemplateAnswers {
		input.TemplateAnswers = append(input.TemplateAnswers, generation.TemplateAnswer{
			Label: answer.Label, Text: answer.Text, Enabled: answer.Enabled,
		})
	}
	if found.ContentLanguage != nil {
		contentLanguage := generation.Language(*found.ContentLanguage)
		input.ContentLanguage = &contentLanguage
	}
	if found.Content != nil {
		content := generation.PostContent{
			Title: found.Content.Title, Summary: found.Content.Summary, Tags: found.Content.Tags,
		}
		for _, block := range found.Content.Blocks {
			content.Blocks = append(content.Blocks, generation.Block{
				Type: generation.BlockType(block.Type), Content: block.Content, Level: block.Level,
				File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
			})
		}
		input.Content = &content
	}
	// Photos first, then videos, each already ordered by created_at: one list, because
	// every selection, freeze and merge function iterates attachments by filename and a
	// second slice would mean maintaining that reasoning twice.
	for _, image := range found.Images {
		input.Images = append(input.Images, generation.Image{
			Filename: image.Filename, Key: image.Key, Kind: generation.AttachmentPhoto,
			ContentType: "image/jpeg",
		})
	}
	for _, video := range found.Videos {
		input.Images = append(input.Images, generation.Image{
			Filename: video.Filename, Key: video.Key, Kind: generation.AttachmentVideo,
			ContentType: video.ContentType, DurationMs: video.DurationMs,
		})
	}
	for _, observation := range found.Observations {
		input.Observations = append(input.Observations, generation.Observation{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent, Model: observation.Model,
			Events: observation.Events, Speech: observation.Speech,
		})
	}
	return input, nil
}

func (a generationPosts) SetObservations(ctx context.Context, userID, slug string, observations []generation.Observation) error {
	values := make([]post.Observation, 0, len(observations))
	for _, observation := range observations {
		values = append(values, post.Observation{
			File: observation.File, Scene: observation.Scene, Mood: observation.Mood,
			VisibleText: observation.VisibleText, Objects: observation.Objects,
			PeoplePresent: observation.PeoplePresent, Model: observation.Model,
			Events: observation.Events, Speech: observation.Speech,
		})
	}
	return generationPostError(a.service.SetObservations(ctx, userID, slug, values))
}

func (a generationPosts) SetGeneratedContent(ctx context.Context, userID, slug string, content generation.PostContent, language generation.Language) error {
	value := post.PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		value.Blocks = append(value.Blocks, post.Block{
			Type: post.BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return generationPostError(a.service.SetGeneratedContent(ctx, userID, slug, value, post.Language(language)))
}

func generationPostError(err error) error {
	switch {
	case errors.Is(err, post.ErrNotFound):
		return generation.ErrNotFound
	case errors.Is(err, post.ErrForbidden):
		return generation.ErrForbidden
	default:
		return err
	}
}

// catalogReasoningSpend adapts the ledger's aggregate to the catalog's own port shape, so
// neither context names the other's type.
//
// It also translates the KEY, which is the whole reason this adapter is not a one-liner: the
// ledger records a model as the registry ref (`openrouter/z-ai/glm-5.3-flash`), while a
// catalog row is the provider-local id (`z-ai/glm-5.3-flash`). Without stripping the
// registry's provider segment here, every spend signal would silently fail to join and the
// curation surface would show nothing however many calls had been recorded.
type generationJobs struct {
	queue  *job.Queue
	budget config.LLMCompletionBudget
}

func (a generationJobs) EnqueueGeneration(ctx context.Context, request generation.StartRequest) (string, error) {
	slug := request.PostSlug
	payload, err := generation.EncodeGenerationPayload(generation.GenerationOptions{
		TargetLanguage: request.TargetLanguage, TargetLength: request.TargetLength, TagCount: request.TagCount, Template: request.Template,
		Guidelines: request.Guidelines, Memories: request.Memories,
		ObserveFiles: request.ObserveFiles, Observations: request.Observations,
	})
	if err != nil {
		return "", err
	}
	calls := map[string]int{}
	if request.ObserveModel != "" {
		// Stated even when it is ZERO: a run that reuses every stored observation makes no
		// observation call, and a hold for one can refuse a user who can afford the write-only
		// retry the picker exists to make cheap. The count is per MODEL across the whole job
		// (internal/job), so a model serving both stages states the write call too.
		total := request.ObserveCalls
		if request.ObserveModel == request.WriteModel {
			total++
		}
		calls[request.ObserveModel] = total
	}
	subjects, guards := postVoiceWork(job.KindGenerate, request.UserID, slug, request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindGenerate, UserID: request.UserID, Subjects: subjects, Guards: guards,
		ObserveModel: request.ObserveModel, WriteModel: request.WriteModel,
		TargetLanguage: request.TargetLanguage.String(), Payload: payload,
		CallCounts: calls, PricingCalls: generationPricingCalls(request, a.budget),
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &generation.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", generation.ErrVoiceDeleted
	}
	return id, err
}

func (a generationJobs) EnqueueRevision(ctx context.Context, request generation.StartRevisionRequest, payload []byte) (string, error) {
	slug := request.PostSlug
	subjects, guards := postVoiceWork(job.KindRevise, request.UserID, slug, request.VoiceID)
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindRevise, UserID: request.UserID, Subjects: subjects, Guards: guards,
		WriteModel: request.WriteModel, TargetLanguage: request.ContentLanguage.String(), Payload: payload,
		PricingCalls: revisionPricingCalls(request, a.budget),
	})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &generation.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return "", generation.ErrVoiceDeleted
	}
	return id, err
}

func generationPricingCalls(request generation.StartRequest, budget config.LLMCompletionBudget) []job.PlannedCall {
	calls := make([]job.PlannedCall, 0, 2)
	if request.ObserveModel != "" && request.ObserveCalls > 0 {
		calls = append(calls, job.PlannedCall{
			Ref: request.ObserveModel, Count: request.ObserveCalls, CompletionTokens: budget.Observation(),
		})
	}
	if request.WriteModel != "" {
		calls = append(calls, job.PlannedCall{
			Ref: request.WriteModel, Count: 1, CompletionTokens: budget.Write(request.TargetLength, request.WriteNativeEffort),
		})
	}
	return calls
}

func revisionPricingCalls(request generation.StartRevisionRequest, budget config.LLMCompletionBudget) []job.PlannedCall {
	if request.WriteModel == "" {
		return nil
	}
	return []job.PlannedCall{{
		Ref: request.WriteModel, Count: 1,
		CompletionTokens: budget.Revise(request.ContentChars, request.TargetLength, request.WriteNativeEffort),
	}}
}

func (a generationJobs) GetGeneration(ctx context.Context, id, userID string) (*generation.JobSummary, error) {
	found, err := a.queue.Get(ctx, id, userID)
	if err != nil {
		switch {
		case errors.Is(err, job.ErrNotFound):
			return nil, generation.ErrNotFound
		case errors.Is(err, job.ErrForbidden):
			return nil, generation.ErrForbidden
		default:
			return nil, err
		}
	}
	postSlug := found.Subject(post.JobSubject)
	return &generation.JobSummary{
		ID: found.ID, Kind: found.Kind, Status: found.Status, Stage: found.Stage,
		ProgressDone: found.ProgressDone, ProgressTotal: found.ProgressTotal,
		Failure:  generationFailure(found.Failure),
		PostSlug: postSlug, ObserveModel: found.ObserveModel, WriteModel: found.WriteModel,
		TargetLanguage: generation.Language(found.TargetLanguage),
		CreatedAt:      found.CreatedAt, UpdatedAt: found.UpdatedAt,
	}, nil
}
