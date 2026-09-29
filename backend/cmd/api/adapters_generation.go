package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/memory"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/quality"
	"github.com/postpilot/backend/internal/storage"
	"github.com/postpilot/backend/internal/template"
	"github.com/postpilot/backend/internal/voice"
)

type generationTemplates struct{ service *template.Service }

// guidelineTemplates hands the guideline context the account's template directory: the ids it
// must prove are owned before saving a scope, and the names it projects when listing.
// generationQuality hands generation the quality context's rendering of the ticked rules. What
// crosses in is the post, the ASCII metric ids and the language; what comes back is TEXTS —
// generation never learns a metric, a band or a measurement (ARCH-7).
type generationQuality struct{ service *quality.Service }

func (a generationQuality) RulesFor(ctx context.Context, userID, slug string, ticked []string, language generation.Language) ([]string, error) {
	var lang quality.Language
	switch language {
	case generation.LanguageKorean:
		lang = quality.LanguageKorean
	case generation.LanguageEnglish:
		lang = quality.LanguageEnglish
	default:
		return nil, fmt.Errorf("quality rules for unknown language %q", language)
	}
	return a.service.RulesFor(ctx, userID, slug, ticked, lang)
}

type generationGuidelines struct{ service *guideline.Service }

// ForPrompt asks for a post's 지침 in the run's target language. Generation receives the texts
// alone (ARCH-7): the kind is always a post's here, and the language is mapped at this seam.
// withMemories passes through as generation states it: whether the run carries [기억] (GEN-73).
func (a generationGuidelines) ForPrompt(ctx context.Context, userID string, templateID, field *string, target generation.Language, withMemories bool) (generation.FrozenGuidelines, error) {
	language := guideline.LanguageKorean
	if target == generation.LanguageEnglish {
		language = guideline.LanguageEnglish
	}
	resolved, err := a.service.ForPrompt(ctx, userID, guideline.KindPost, templateID, field, language, withMemories)
	if err != nil {
		return generation.FrozenGuidelines{}, err
	}
	return generation.FrozenGuidelines{Defaults: resolved.Defaults, Owner: resolved.Owner}, nil
}

// generationMemories hands the generation context the memory context's retrieval. What
// crosses is the post's own words and, coming back, TEXTS — the generation context never
// learns that a memory has a kind, tags or an id, and the memory context never learns what a
// job is. A preference comes back labelled `취향:` inside its text, which is prompt framing the
// memories-only 기본 지침 reads rather than a kind (GEN-73). It is consulted once per enqueue,
// and only for a post that opted in.
type generationMemories struct{ service *memory.Service }

func (a generationMemories) ForPost(ctx context.Context, userID string, keyParts []string) ([]string, error) {
	return a.service.TextsForPost(ctx, userID, keyParts)
}

// generationCandidates hands the generation context the candidate recorder. The instruction
// crosses as opaque text: the guideline context records the user's sentence without learning
// what a revision job is, and nothing on either side reads it with a model.
type generationCandidates struct{ service *guideline.Service }

func (a generationCandidates) Record(ctx context.Context, userID, postSlug, instruction string) error {
	return a.service.RecordCandidate(ctx, userID, guideline.KindPost, postSlug, instruction)
}

// clipGuidelineCandidates hands the clip context the same recorder for its revision requests, a
// 영상 지침 candidate each, and the detach a project deletion needs (GUIDE-7, GUIDE-13).
type clipGuidelineCandidates struct{ service *guideline.Service }

func (a clipGuidelineCandidates) Record(ctx context.Context, userID, projectID, request string) error {
	return a.service.RecordCandidate(ctx, userID, guideline.KindClip, projectID, request)
}

func (a clipGuidelineCandidates) DetachProject(ctx context.Context, userID, projectID string) error {
	return a.service.DetachCandidateClip(ctx, userID, projectID)
}

// ForClip resolves a clip's 영상 지침 for the clip context (GUIDE-15, GUIDE-17): the clip kind's
// enabled 기본 지침 in the project's language and the owner's scoped to its video template, or
// the global ones alone when the project has none.
func (a clipGuidelineCandidates) ForClip(ctx context.Context, userID, videoTemplateID, language string) (clip.VideoGuidelines, error) {
	var templateID *string
	if videoTemplateID != "" {
		templateID = &videoTemplateID
	}
	// A clip carries no memories (MEM-29), so no memories-only 기본 지침 reaches it.
	got, err := a.service.ForPrompt(ctx, userID, guideline.KindClip, templateID, nil, guideline.Language(language), false)
	if err != nil {
		return clip.VideoGuidelines{}, err
	}
	return clip.VideoGuidelines{Defaults: got.Defaults, Owner: got.Owner}, nil
}

// postCandidateLinks lets post deletion drop the link without the post context learning what
// a guideline candidate is: it hands over the account and the slug, and nothing comes back.
func (a generationTemplates) RenderedFor(ctx context.Context, userID, templateID string, hasPhotos bool, answers []generation.TemplateAnswer) (generation.TemplateBrief, bool, error) {
	owned := make([]template.Answer, 0, len(answers))
	for _, answer := range answers {
		owned = append(owned, template.Answer{Label: answer.Label, Text: answer.Text, Enabled: answer.Enabled})
	}
	rendered, ok, err := a.service.RenderedFor(ctx, userID, templateID, hasPhotos, owned)
	if err != nil || !ok {
		return generation.TemplateBrief{}, false, err
	}
	facts := make([]generation.TemplateFact, 0, len(rendered.Facts))
	for _, fact := range rendered.Facts {
		facts = append(facts, generation.TemplateFact{Label: fact.Label, Value: fact.Value})
	}
	return generation.TemplateBrief{
		Name: rendered.Name, Body: rendered.Body, Facts: facts, TitleArea: rendered.TitleArea,
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
	retrieval := strings.TrimSpace(topic + " " + strings.Join(tags, " "))
	profile, err := a.service.PromptProfileForTopic(ctx, userID, voiceID, retrieval, voice.Language(target), "")
	return generation.Profile{Text: profile.Text, Excerpts: profile.Excerpts, Portable: profile.Portable}, generationVoiceError(err)
}

func generationVoiceError(err error) error {
	switch {
	case errors.Is(err, voice.ErrVoiceDeleted):
		return generation.ErrVoiceDeleted
	case errors.Is(err, voice.ErrVoiceNotMade):
		return generation.ErrVoiceNotMade
	case errors.Is(err, voice.ErrVoiceNotFound), errors.Is(err, voice.ErrVoiceRequired):
		// The voice the run froze no longer resolves: the result may not land (GEN-27).
		return generation.ErrVoiceMismatch
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
		Voice:          generation.VoiceRef{ID: found.Voice.ID, Name: found.Voice.Name, Deleted: found.Voice.Deleted, Made: found.Voice.Made},
		TargetLanguage: generation.Language(found.TargetLanguage),
		// The id, never the brief: only the enqueue resolves it, and only through the template
		// context's own port. Dropping it here is what would make the whole feature a silent
		// no-op — every prompt would be built as if no post ever had a 템플릿.
		TemplateID:   found.TemplateID,
		TargetLength: found.TargetLength,
		TagCount:     found.TagCount,
		// The lock, read here so every generation path refuses a published post before it
		// freezes, holds or calls anything (POST-74, GEN-56).
		Published: found.Status == post.StatusPublished,
		// Inputs the enqueue resolves, like TemplateID: the 분야 and the ticked metrics.
		Field:          found.Field,
		QualityRuleIDs: append([]string(nil), found.QualityRules...),
		// The opt-in, never the memories: like TemplateID, only the enqueue resolves it, and
		// only through the memory context's own port (MEM-18, MEM-19).
		UseMemory: found.UseMemory,
		Images:    make([]generation.Image, 0, len(found.Images)+len(found.Videos)),
		// The stored contact sheet, read here so the ENQUEUE can decide what to reuse. It
		// was write-only from this context's point of view before observations could be reused
		// (GEN-8), which is why every retry re-paid for eyesight the post already had.
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
	if found.Storyline != nil {
		input.Storyline = generationStoryline(*found.Storyline)
	}
	return input, nil
}

// generationStoryline hands generation the stored storyline's paragraphs and what it was made
// with; whether the owner edited it is the post's own business.
func generationStoryline(storyline post.Storyline) *generation.Storyline {
	out := &generation.Storyline{MadeWith: append([]string(nil), storyline.MadeWith...)}
	for _, paragraph := range storyline.Paragraphs {
		out.Paragraphs = append(out.Paragraphs, generation.StorylineParagraph{
			Text: paragraph.Text, Files: append([]string(nil), paragraph.Files...),
		})
	}
	return out
}

// postStoryline is the other direction, for a write's or a storyline job's answer.
func postStoryline(storyline generation.Storyline) post.Storyline {
	out := post.Storyline{MadeWith: append([]string(nil), storyline.MadeWith...)}
	for _, paragraph := range storyline.Paragraphs {
		out.Paragraphs = append(out.Paragraphs, post.StorylineParagraph{
			Text: paragraph.Text, Files: append([]string(nil), paragraph.Files...),
		})
	}
	return out
}

func (a generationPosts) SetStoryline(ctx context.Context, userID, slug string, storyline generation.Storyline) error {
	return generationPostError(a.service.SetStoryline(ctx, userID, slug, postStoryline(storyline)))
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

func (a generationPosts) SetGeneratedContent(ctx context.Context, userID, slug string, content generation.PostContent, language generation.Language, annotations *generation.WriteAnnotations) error {
	value := post.PostContent{Title: content.Title, Summary: content.Summary, Tags: content.Tags}
	for _, block := range content.Blocks {
		value.Blocks = append(value.Blocks, post.Block{
			Type: post.BlockType(block.Type), Content: block.Content, Level: block.Level,
			File: block.File, Alt: block.Alt, Caption: block.Caption, Items: block.Items,
		})
	}
	return generationPostError(a.service.SetGeneratedContent(ctx, userID, slug, value, post.Language(language), postAnnotations(annotations)))
}

// postAnnotations hands the post a write's nouns. nil stays nil, which keeps what the post
// holds.
func postAnnotations(annotations *generation.WriteAnnotations) *post.WriteAnnotations {
	if annotations == nil {
		return nil
	}
	out := &post.WriteAnnotations{Nouns: append([]string(nil), annotations.Nouns...)}
	if annotations.Storyline != nil {
		storyline := postStoryline(*annotations.Storyline)
		out.Storyline = &storyline
	}
	return out
}

func generationPostError(err error) error {
	switch {
	case errors.Is(err, post.ErrNotFound):
		return generation.ErrNotFound
	case errors.Is(err, post.ErrForbidden):
		return generation.ErrForbidden
	case errors.Is(err, post.ErrPostPublished):
		return generation.ErrPostPublished
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

// EnqueueGeneration stores the payload generation encoded, byte for byte: the frozen options are
// generation's own, and this adapter only routes the row, guards it and prices its hold.
func (a generationJobs) EnqueueGeneration(ctx context.Context, request generation.StartRequest, payload []byte) (string, error) {
	slug := request.PostSlug
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

// EnqueueStoryline routes a storyline job (GEN-68). It is post-targeted — the one active job per
// post applies — and carries no voice: the prompt has none, so a voice change cannot make its
// answer land in the wrong profile.
func (a generationJobs) EnqueueStoryline(ctx context.Context, request generation.StartStorylineRequest, payload []byte) (string, error) {
	calls := map[string]int{}
	if request.ObserveModel != "" {
		// Stated even when ZERO, as for a generation: the count is per model across the job.
		total := request.ObserveCalls
		if request.ObserveModel == request.WriteModel {
			total++
		}
		calls[request.ObserveModel] = total
	}
	subjects, guards := postVoiceWork(job.KindStoryline, request.UserID, request.PostSlug, "")
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindStoryline, UserID: request.UserID, Subjects: subjects, Guards: guards,
		ObserveModel: request.ObserveModel, WriteModel: request.WriteModel,
		TargetLanguage: request.TargetLanguage.String(), Payload: payload,
		CallCounts: calls, PricingCalls: storylinePricingCalls(request, a.budget),
	})
	return id, generationEnqueueError(err)
}

// EnqueueStorylineRevision routes the storyline space's AI request (GEN-69): one call, no
// observation.
func (a generationJobs) EnqueueStorylineRevision(ctx context.Context, request generation.StartStorylineRevisionRequest, payload []byte) (string, error) {
	subjects, guards := postVoiceWork(job.KindReviseStoryline, request.UserID, request.PostSlug, "")
	id, err := a.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindReviseStoryline, UserID: request.UserID, Subjects: subjects, Guards: guards,
		WriteModel: request.WriteModel, TargetLanguage: request.TargetLanguage.String(), Payload: payload,
		PricingCalls: storylineRevisionPricingCalls(request),
	})
	return id, generationEnqueueError(err)
}

func generationEnqueueError(err error) error {
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return &generation.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	if errors.Is(err, job.ErrVoiceUnavailable) {
		return generation.ErrVoiceDeleted
	}
	return err
}

// storylinePricingCalls prices a storyline job over its frozen set, as a generation is priced
// (QUOTA-13): the observe calls it will make, then one storyline call at its own budget.
func storylinePricingCalls(request generation.StartStorylineRequest, budget config.LLMCompletionBudget) []job.PlannedCall {
	calls := make([]job.PlannedCall, 0, 2)
	if request.ObserveModel != "" && request.ObserveCalls > 0 {
		calls = append(calls, job.PlannedCall{
			Ref: request.ObserveModel, Count: request.ObserveCalls, CompletionTokens: budget.Observation(),
		})
	}
	if request.WriteModel != "" {
		calls = append(calls, job.PlannedCall{
			Ref: request.WriteModel, Count: 1, CompletionTokens: generation.StorylineCompletionBudget,
		})
	}
	return calls
}

// storylineRevisionPricingCalls prices the storyline request: one storyline call.
func storylineRevisionPricingCalls(request generation.StartStorylineRevisionRequest) []job.PlannedCall {
	if request.WriteModel == "" {
		return nil
	}
	return []job.PlannedCall{{Ref: request.WriteModel, Count: 1, CompletionTokens: generation.StorylineCompletionBudget}}
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
