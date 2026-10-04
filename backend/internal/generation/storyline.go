package generation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

// StartStorylineRequest is 스토리라인 먼저 or 다시 만들기 on the way in (GEN-68). TargetLanguage,
// VoiceID, ObserveCalls and WriteNativeEffort are resolved by StartStoryline; the enqueue adapter
// reads them for the row and the hold.
type StartStorylineRequest struct {
	UserID       string
	PostSlug     string
	ObserveModel string
	WriteModel   string
	// ObserveFiles is the re-observation picker's answer, with StartRequest's presence rules.
	ObserveFiles   *[]string
	TargetLanguage Language
	VoiceID        string
	ObserveCalls   int
	// WriteNativeEffort is frozen from the write model at enqueue, as Start freezes it, so the
	// hold prices exactly the cap the call sends (GEN-22).
	WriteNativeEffort bool
}

// StorylineJob is one queued storyline job as the worker hands it over.
type StorylineJob struct {
	UserID       string
	PostSlug     string
	ObserveModel string
	WriteModel   string
	Payload      []byte
}

// StartStorylineRevisionRequest is the storyline space's AI request on the way in (GEN-69).
// WriteNativeEffort is frozen at enqueue as StartStorylineRequest's is.
type StartStorylineRevisionRequest struct {
	UserID            string
	PostSlug          string
	Request           string
	WriteModel        string
	TargetLanguage    Language
	VoiceID           string
	WriteNativeEffort bool
}

// StorylineRevisionJob is one queued storyline request as the worker hands it over.
type StorylineRevisionJob struct {
	UserID     string
	PostSlug   string
	WriteModel string
	Payload    []byte
}

// StartStoryline freezes a storyline job (GEN-68) under Start's preconditions, in Start's
// order, and with its observe semantics: the same selection, the same reusable snapshot and
// the same frozen material, without the length, the tags, the rules and the voice, which a
// plan does not use.
func (s *Service) StartStoryline(ctx context.Context, request StartStorylineRequest) (string, error) {
	post, err := s.posts.AttachedImages(ctx, request.UserID, request.PostSlug)
	if err != nil {
		return "", err
	}
	if post.Published {
		return "", ErrPostPublished
	}
	if !post.TargetLanguage.Valid() {
		return "", ErrLanguageRequired
	}
	request.TargetLanguage = post.TargetLanguage
	voiceID, err := activeVoice(post)
	if err != nil {
		return "", err
	}
	request.VoiceID = voiceID
	if err := s.refusePendingExperiment(ctx, request.UserID, request.PostSlug); err != nil {
		return "", err
	}
	writeInfo, err := s.requireWriteModel(request.WriteModel)
	if err != nil {
		return "", err
	}
	request.WriteNativeEffort = writeInfo.ReasoningNativeEffort
	if len(post.Images) == 0 {
		request.ObserveModel = ""
	} else {
		observe, valid := parseModelRef(request.ObserveModel)
		if !valid || !modelEnabled(s.models, observe, llm.StageNameObserve) {
			return "", ErrObserveModelRequired
		}
		if err := s.refuseVideoBlindObserveModel(post.Images, observe); err != nil {
			return "", err
		}
	}
	material, err := s.freezeStorylineMaterial(ctx, post)
	if err != nil {
		return "", err
	}
	options := storylineOptions{TargetLanguage: post.TargetLanguage, storylineMaterial: material, WriteNativeEffort: request.WriteNativeEffort}
	if len(post.Images) > 0 {
		files, carried := freezeObserveSelection(post.Images, post.Observations, request.ObserveFiles)
		options.ObserveFiles = &files
		options.Observations = carried
	}
	request.ObserveCalls = s.observeCalls(observeTargets(post.Images, options.ObserveFiles))
	payload, err := encodeStorylinePayload(options)
	if err != nil {
		return "", fmt.Errorf("encode storyline payload: %w", err)
	}
	id, err := s.jobs.EnqueueStoryline(ctx, request, payload)
	if err != nil {
		return "", fmt.Errorf("enqueue storyline: %w", err)
	}
	return id, nil
}

// StartStorylineRevision freezes the storyline space's AI request (GEN-69): the request, the
// stored paragraphs, the material and every stored observation. It observes nothing, so it
// takes no observe model and no selection.
func (s *Service) StartStorylineRevision(ctx context.Context, request StartStorylineRevisionRequest) (string, error) {
	request.Request = strings.TrimSpace(request.Request)
	if request.Request == "" {
		return "", ErrRevisionInstructionRequired
	}
	if utf8.RuneCountInString(request.Request) > RevisionInstructionMaxChars {
		return "", ErrRevisionInstructionTooLong
	}
	post, err := s.posts.AttachedImages(ctx, request.UserID, request.PostSlug)
	if err != nil {
		return "", err
	}
	if post.Published {
		return "", ErrPostPublished
	}
	if post.Storyline == nil || len(post.Storyline.Paragraphs) == 0 {
		return "", ErrStorylineMissing
	}
	if !post.TargetLanguage.Valid() {
		return "", ErrLanguageRequired
	}
	request.TargetLanguage = post.TargetLanguage
	voiceID, err := activeVoice(post)
	if err != nil {
		return "", err
	}
	request.VoiceID = voiceID
	if err := s.refusePendingExperiment(ctx, request.UserID, request.PostSlug); err != nil {
		return "", err
	}
	writeInfo, err := s.requireWriteModel(request.WriteModel)
	if err != nil {
		return "", err
	}
	request.WriteNativeEffort = writeInfo.ReasoningNativeEffort
	material, err := s.freezeStorylineMaterial(ctx, post)
	if err != nil {
		return "", err
	}
	payload, err := encodeStorylineRevisionPayload(storylineRevisionOptions{
		TargetLanguage: post.TargetLanguage, Request: request.Request,
		Storyline: cloneParagraphs(post.Storyline.Paragraphs), storylineMaterial: material,
		Observations: cloneObservations(post.Observations), WriteNativeEffort: request.WriteNativeEffort,
	})
	if err != nil {
		return "", fmt.Errorf("encode storyline revision payload: %w", err)
	}
	id, err := s.jobs.EnqueueStorylineRevision(ctx, request, payload)
	if err != nil {
		return "", fmt.Errorf("enqueue storyline revision: %w", err)
	}
	return id, nil
}

// requireWriteModel is Start's write-model precondition: an enabled model that serves the write
// stage, which is the stage a storyline call runs on. It answers the model's catalog entry, whose
// native-effort flag the start freezes.
func (s *Service) requireWriteModel(value string) (llm.ModelInfo, error) {
	write, ok := parseModelRef(value)
	info, found := s.models.Resolve(write)
	if !ok || !found || info.Disabled || !info.ServesStage(llm.StageNameWrite) {
		return llm.ModelInfo{}, ErrWriteModelRequired
	}
	return info, nil
}

// WriteStoryline handles one storyline job (GEN-68): the observe step a generation runs, then
// one call, then the storyline replaces the post's. The content, the machine baseline, the
// status and the revisions are never touched, and a failure leaves the stored storyline as it
// was, because the only write comes after a parsed answer.
func (s *Service) WriteStoryline(ctx context.Context, job StorylineJob, progress Progress) error {
	options, err := decodeStorylinePayload(job.Payload)
	if err != nil {
		return err
	}
	post, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("load storyline input: %w", err)
	}
	if post.Published {
		return ErrPostPublished
	}
	post = options.onto(post)
	post, observations, err := s.observeForRun(ctx, post, job.ObserveModel, options.ObserveFiles, options.Observations, progress)
	if err != nil {
		return err
	}
	photos, videos := AttachmentNames(post.Images)
	shown := append(append([]string(nil), photos...), videos...)
	system, user := BuildStorylinePromptForLanguage(StorylinePromptInput{
		Language: post.TargetLanguage, Title: post.Title, Memo: post.Memo,
		Photos: photos, Videos: videos, Observations: observations,
		Template: post.Template, DefaultGuidelines: post.DefaultGuidelines, Guidelines: post.Guidelines,
		Memories: post.Memories,
	})
	progress("storyline", 0, 1)
	paragraphs, err := s.storylineCall(ctx, job.WriteModel, options.WriteNativeEffort, system, user, shown)
	if err != nil {
		return err
	}
	// What the call was shown is what a later attachment reads as added against (POST-99).
	if err := s.posts.SetStoryline(ctx, post.UserID, post.Slug, Storyline{Paragraphs: paragraphs, MadeWith: shown}); err != nil {
		return fmt.Errorf("persist storyline: %w", err)
	}
	progress("storyline", 1, 1)
	return nil
}

// ReviseStoryline handles the storyline space's AI request (GEN-69): one call on the request
// prompt, from the frozen material and observations. MadeWith keeps its stored value — the
// request rearranges what the write was shown, it shows nothing new — and no guideline
// candidate is recorded, because the request is about this storyline, not every post (GUIDE-7).
func (s *Service) ReviseStoryline(ctx context.Context, job StorylineRevisionJob, progress Progress) error {
	options, err := decodeStorylineRevisionPayload(job.Payload)
	if err != nil {
		return err
	}
	post, err := s.posts.AttachedImages(ctx, job.UserID, job.PostSlug)
	if err != nil {
		return fmt.Errorf("load storyline revision input: %w", err)
	}
	if post.Published {
		return ErrPostPublished
	}
	// Shown: the attachments still here that the frozen observations have eyesight for. One
	// deleted since the enqueue must not come back into the storyline.
	post.Images = observedImages(post.Images, options.Observations)
	photos, videos := AttachmentNames(post.Images)
	shown := append(append([]string(nil), photos...), videos...)
	system, user := BuildStorylinePromptForLanguage(StorylinePromptInput{
		Language: options.TargetLanguage, Title: post.Title, Memo: post.Memo,
		Photos: photos, Videos: videos, Observations: options.Observations,
		Template: options.Template, DefaultGuidelines: options.DefaultGuidelines, Guidelines: options.Guidelines,
		Memories: options.Memories, Current: options.Storyline, Request: options.Request,
	})
	progress("storyline", 0, 1)
	paragraphs, err := s.storylineCall(ctx, job.WriteModel, options.WriteNativeEffort, system, user, shown)
	if err != nil {
		return err
	}
	var madeWith []string
	if post.Storyline != nil {
		madeWith = append([]string(nil), post.Storyline.MadeWith...)
	}
	if err := s.posts.SetStoryline(ctx, post.UserID, post.Slug, Storyline{Paragraphs: paragraphs, MadeWith: madeWith}); err != nil {
		return fmt.Errorf("persist storyline: %w", err)
	}
	progress("storyline", 1, 1)
	return nil
}

// storylineCall is the one call both storyline jobs make: the write model at low reasoning with
// the storyline's budget for the native-effort flag the start froze, answered in the storyline
// schema and parsed against what was shown. A bad or cut-off answer fails as GEN-20 says
// (MODEL_OUTPUT_INVALID, MODEL_OUTPUT_TRUNCATED).
func (s *Service) storylineCall(ctx context.Context, writeModel string, nativeEffort bool, system, user string, shown []string) ([]StorylineParagraph, error) {
	model, ok := parseModelRef(writeModel)
	if !ok {
		return nil, ErrWriteModelRequired
	}
	request := llm.Request{
		System:    system,
		Messages:  []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(user)}}},
		Reasoning: llm.ReasoningLow,
		Stage:     llm.StageNameWrite,
		MaxTokens: s.budget.Storyline(nativeEffort),
	}
	if info, found := s.models.Resolve(model); found && info.StructuredOutput {
		request.JSONSchema = StorylineAnswerSchema()
	}
	response, err := s.models.Complete(ctx, model, request)
	if err != nil {
		return nil, providerCallError("스토리라인 작성", err)
	}
	paragraphs, err := ParseStorylineAnswer(response.Text, shown)
	if err != nil {
		return nil, responseParseError(response, err)
	}
	return paragraphs, nil
}

// storylineMaterial is what both storyline jobs freeze beside their own members: the brief,
// the 지침 and the 기억 (GEN-68). No quality rules — they are measured on a post, not a plan.
type storylineMaterial struct {
	Template          *TemplateBrief
	Guidelines        []string
	DefaultGuidelines []string
	Memories          []string
}

// freezeStorylineMaterial resolves the material once, at enqueue, through the same freezes a
// generation uses, so the two can never disagree about what a post's brief or 지침 are.
func (s *Service) freezeStorylineMaterial(ctx context.Context, post PostInput) (storylineMaterial, error) {
	brief, err := s.freezeTemplate(ctx, post, true)
	if err != nil {
		return storylineMaterial{}, err
	}
	memories, err := s.freezeMemories(ctx, post)
	if err != nil {
		return storylineMaterial{}, err
	}
	// The storyline prompt carries the same [기억] section, so it gets the same memories-only
	// 기본 지침 exactly when that section exists (GEN-73).
	guidelines, err := s.freezeGuidelines(ctx, post, post.TargetLanguage, len(memories) > 0)
	if err != nil {
		return storylineMaterial{}, err
	}
	return storylineMaterial{Template: brief, Guidelines: guidelines.Owner, DefaultGuidelines: guidelines.Defaults, Memories: memories}, nil
}

// storylineOptions is what a storyline job froze at enqueue.
type storylineOptions struct {
	TargetLanguage Language
	storylineMaterial
	// ObserveFiles and Observations are the frozen selection and snapshot, with the generate
	// payload's presence rules.
	ObserveFiles *[]string
	Observations []Observation
	// WriteNativeEffort sizes the call's budget as the hold priced it; a payload without it is
	// an ordinary model's.
	WriteNativeEffort bool
}

func (o storylineOptions) onto(post PostInput) PostInput {
	post.TargetLanguage = o.TargetLanguage
	post.Template = o.Template
	post.Guidelines = o.Guidelines
	post.DefaultGuidelines = o.DefaultGuidelines
	post.Memories = o.Memories
	return post
}

// storylineRevisionOptions is what a storyline request froze at enqueue.
type storylineRevisionOptions struct {
	TargetLanguage Language
	Request        string
	Storyline      []StorylineParagraph
	storylineMaterial
	Observations      []Observation
	WriteNativeEffort bool
}

type storylinePayload struct {
	TargetLanguage    string               `json:"target_language"`
	Template          *templatePayload     `json:"template,omitempty"`
	Guidelines        []string             `json:"guidelines,omitempty"`
	DefaultGuidelines []string             `json:"default_guidelines,omitempty"`
	Memories          []string             `json:"memories,omitempty"`
	ObserveFiles      *[]string            `json:"observe_files"`
	Observations      []observationPayload `json:"observations,omitempty"`
	WriteNativeEffort bool                 `json:"write_native_effort,omitempty"`
}

type storylineRevisionPayload struct {
	TargetLanguage    string                   `json:"target_language"`
	Request           string                   `json:"request"`
	Storyline         []storylineParagraphJSON `json:"storyline"`
	Template          *templatePayload         `json:"template,omitempty"`
	Guidelines        []string                 `json:"guidelines,omitempty"`
	DefaultGuidelines []string                 `json:"default_guidelines,omitempty"`
	Memories          []string                 `json:"memories,omitempty"`
	Observations      []observationPayload     `json:"observations,omitempty"`
	WriteNativeEffort bool                     `json:"write_native_effort,omitempty"`
}

func encodeStorylinePayload(options storylineOptions) ([]byte, error) {
	if !options.TargetLanguage.Valid() {
		return nil, ErrLanguageRequired
	}
	return json.Marshal(storylinePayload{
		TargetLanguage:    options.TargetLanguage.String(),
		Template:          encodeTemplate(options.Template),
		Guidelines:        cloneTexts(options.Guidelines),
		DefaultGuidelines: cloneTexts(options.DefaultGuidelines),
		Memories:          cloneTexts(options.Memories),
		ObserveFiles:      cloneOptionalTexts(options.ObserveFiles),
		Observations:      encodeObservations(options.Observations),
		WriteNativeEffort: options.WriteNativeEffort,
	})
}

func decodeStorylinePayload(raw []byte) (storylineOptions, error) {
	var payload storylinePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return storylineOptions{}, fmt.Errorf("decode storyline payload: %w", err)
	}
	language, err := payloadLanguage(payload.TargetLanguage)
	if err != nil {
		return storylineOptions{}, fmt.Errorf("decode storyline payload: %w", err)
	}
	return storylineOptions{
		TargetLanguage: language,
		storylineMaterial: storylineMaterial{
			Template:          decodeTemplate(payload.Template),
			Guidelines:        cloneTexts(payload.Guidelines),
			DefaultGuidelines: cloneTexts(payload.DefaultGuidelines),
			Memories:          cloneTexts(payload.Memories),
		},
		ObserveFiles:      cloneOptionalTexts(payload.ObserveFiles),
		Observations:      decodeObservations(payload.Observations),
		WriteNativeEffort: payload.WriteNativeEffort,
	}, nil
}

func encodeStorylineRevisionPayload(options storylineRevisionOptions) ([]byte, error) {
	if !options.TargetLanguage.Valid() {
		return nil, ErrLanguageRequired
	}
	return json.Marshal(storylineRevisionPayload{
		TargetLanguage:    options.TargetLanguage.String(),
		Request:           options.Request,
		Storyline:         storylineForPrompt(options.Storyline)["storyline"],
		Template:          encodeTemplate(options.Template),
		Guidelines:        cloneTexts(options.Guidelines),
		DefaultGuidelines: cloneTexts(options.DefaultGuidelines),
		Memories:          cloneTexts(options.Memories),
		Observations:      encodeObservations(options.Observations),
		WriteNativeEffort: options.WriteNativeEffort,
	})
}

func decodeStorylineRevisionPayload(raw []byte) (storylineRevisionOptions, error) {
	var payload storylineRevisionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return storylineRevisionOptions{}, fmt.Errorf("decode storyline revision payload: %w", err)
	}
	language, err := payloadLanguage(payload.TargetLanguage)
	if err != nil {
		return storylineRevisionOptions{}, fmt.Errorf("decode storyline revision payload: %w", err)
	}
	paragraphs := make([]StorylineParagraph, 0, len(payload.Storyline))
	for _, paragraph := range payload.Storyline {
		paragraphs = append(paragraphs, StorylineParagraph{Text: paragraph.Text, Files: cloneTexts(paragraph.Files)})
	}
	return storylineRevisionOptions{
		TargetLanguage: language,
		Request:        payload.Request,
		Storyline:      paragraphs,
		storylineMaterial: storylineMaterial{
			Template:          decodeTemplate(payload.Template),
			Guidelines:        cloneTexts(payload.Guidelines),
			DefaultGuidelines: cloneTexts(payload.DefaultGuidelines),
			Memories:          cloneTexts(payload.Memories),
		},
		Observations:      decodeObservations(payload.Observations),
		WriteNativeEffort: payload.WriteNativeEffort,
	}, nil
}

// payloadLanguage reads a frozen target language; a payload without one is Korean, as every
// other payload reads it (GEN-30).
func payloadLanguage(value string) (Language, error) {
	if value == "" {
		return LanguageKorean, nil
	}
	return ParseLanguage(value)
}

func cloneParagraphs(paragraphs []StorylineParagraph) []StorylineParagraph {
	out := make([]StorylineParagraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		out = append(out, StorylineParagraph{Text: paragraph.Text, Files: cloneTexts(paragraph.Files)})
	}
	return out
}

func cloneObservations(observations []Observation) []Observation {
	return decodeObservations(encodeObservations(observations))
}
