package generation

import (
	"context"
	"encoding/json"
	"fmt"
	postdomain "github.com/postpilot/backend/internal/post"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

// StartStorylineRequest is 스토리라인 먼저 or 다시 만들기 on the way in (GEN-68). TargetLanguage,
// VoiceID, ObserveCalls and WriteNativeEffort are resolved by StartStoryline; the enqueue adapter
// reads them for the row and the hold.
type StartStorylineRequest struct {
	ExpectedPlanFingerprint *string
	ExpectedInputRevision   *int64
	OriginProtocolVersion   int
	CompletionTokens        int
	UserID                  string
	PostSlug                string
	ObserveModel            string
	WriteModel              string
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
	ID           string
	UserID       string
	PostSlug     string
	ObserveModel string
	WriteModel   string
	Payload      []byte
}

// StartStorylineRevisionRequest is the storyline space's AI request on the way in (GEN-69).
// WriteNativeEffort is frozen at enqueue as StartStorylineRequest's is.
type StartStorylineRevisionRequest struct {
	ExpectedPlanFingerprint *string
	ExpectedInputRevision   *int64
	OriginProtocolVersion   int
	CompletionTokens        int
	UserID                  string
	PostSlug                string
	Request                 string
	WriteModel              string
	TargetLanguage          Language
	VoiceID                 string
	WriteNativeEffort       bool
}

// StorylineRevisionJob is one queued storyline request as the worker hands it over.
type StorylineRevisionJob struct {
	ID         string
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
	in, err := s.startPreconditions(ctx, request.UserID, request.PostSlug, startStage{
		writeModel: request.WriteModel,
		observe:    observePicked, observeModel: request.ObserveModel, observeFiles: request.ObserveFiles,
	})
	if err != nil {
		return "", err
	}
	request.OriginProtocolVersion = s.originProtocol
	if s.originProtocol == OriginProtocolVersion {
		inputRevision := in.post.InputRevision
		request.ExpectedInputRevision = &inputRevision
		request.ExpectedPlanFingerprint = originExpectedPlanFingerprint(in.post)
	}
	if s.originProtocol == OriginProtocolVersion {
		request.CompletionTokens = s.budget.Storyline(in.write.ReasoningNativeEffort)
	}
	request.TargetLanguage = in.language
	request.VoiceID = in.voiceID
	request.WriteNativeEffort = in.write.ReasoningNativeEffort
	request.ObserveModel = in.observe.model
	request.ObserveCalls = in.observe.calls
	material, err := s.freezeStorylineMaterial(ctx, in.post)
	if err != nil {
		return "", err
	}
	payload, err := encodeStorylinePayload(storylineOptions{
		ExpectedPlanFingerprint: request.ExpectedPlanFingerprint,
		ExpectedInputRevision:   request.ExpectedInputRevision,
		OriginProtocolVersion:   s.originProtocol, CompletionTokens: request.CompletionTokens,
		TargetLanguage: in.language, storylineMaterial: material, WriteNativeEffort: request.WriteNativeEffort,
		ObserveFiles: in.observe.files, Observations: in.observe.observations,
	})
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
	in, err := s.startPreconditions(ctx, request.UserID, request.PostSlug, startStage{
		storyline: true, writeModel: request.WriteModel, observe: observeReused,
	})
	if err != nil {
		return "", err
	}
	request.OriginProtocolVersion = s.originProtocol
	if s.originProtocol == OriginProtocolVersion {
		inputRevision := in.post.InputRevision
		request.ExpectedInputRevision = &inputRevision
		request.ExpectedPlanFingerprint = originExpectedPlanFingerprint(in.post)
	}
	if s.originProtocol == OriginProtocolVersion {
		request.CompletionTokens = s.budget.Storyline(in.write.ReasoningNativeEffort)
	}
	request.TargetLanguage = in.language
	request.VoiceID = in.voiceID
	request.WriteNativeEffort = in.write.ReasoningNativeEffort
	material, err := s.freezeStorylineMaterial(ctx, in.post)
	if err != nil {
		return "", err
	}
	payload, err := encodeStorylineRevisionPayload(storylineRevisionOptions{
		ExpectedPlanFingerprint: request.ExpectedPlanFingerprint,
		ExpectedInputRevision:   request.ExpectedInputRevision,
		OriginProtocolVersion:   s.originProtocol, CompletionTokens: request.CompletionTokens, PlanOrigins: clonePlanOrigins(in.post.Storyline.Origins),
		TargetLanguage: in.language, Request: request.Request,
		Storyline: in.storyline, storylineMaterial: material,
		Observations: in.observe.observations, WriteNativeEffort: request.WriteNativeEffort,
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
	ctx, finishCapture := s.beginRequestCapture(ctx, job.ID, post)
	defer finishCapture()
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
	input := StorylinePromptInput{
		Language: post.TargetLanguage, Title: post.Title, Memo: post.Memo,
		Photos: photos, Videos: videos, Observations: observations,
		Template: post.Template, DefaultGuidelines: post.DefaultGuidelines, StockGuidelines: post.StockGuidelines, Guidelines: post.Guidelines,
		Memories: post.Memories,
	}
	request, sources := preparePlanRequest(input, options.OriginProtocolVersion, options.CompletionTokens, originAttachmentIDs(post.Images), nil)
	progress("storyline", 0, 1)
	result, err := s.storylineCall(ctx, job.WriteModel, options.WriteNativeEffort, request, shown, sources, options.OriginProtocolVersion)
	if err != nil {
		return err
	}
	// What the call was shown is what a later attachment reads as added against (POST-99).
	result.MadeWith = shown
	var publishErr error
	if options.OriginProtocolVersion == OriginProtocolVersion {
		publishErr = s.originPlans.PublishStorylineResult(ctx, post.UserID, post.Slug, OriginStorylineCompletion{Storyline: result, ExpectedContentRevision: post.ContentRevision, ExpectedInputRevision: options.ExpectedInputRevision, ExpectedPlanFingerprint: options.ExpectedPlanFingerprint})
	} else {
		publishErr = s.posts.SetStoryline(ctx, post.UserID, post.Slug, result)
	}
	if err := publishErr; err != nil {
		return fmt.Errorf("persist storyline: %w", err)
	}
	s.bindPublishedRequestCapturePlan(ctx, post.UserID, post.Slug, result)
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
	ctx, finishCapture := s.beginRequestCapture(ctx, job.ID, post)
	defer finishCapture()
	if post.Published {
		return ErrPostPublished
	}
	// Shown: the attachments still here that the frozen observations have eyesight for. One
	// deleted since the enqueue must not come back into the storyline.
	post.Images = observedImages(post.Images, options.Observations)
	photos, videos := AttachmentNames(post.Images)
	shown := append(append([]string(nil), photos...), videos...)
	input := StorylinePromptInput{
		Language: options.TargetLanguage, Title: post.Title, Memo: post.Memo,
		Photos: photos, Videos: videos, Observations: options.Observations,
		Template: options.Template, DefaultGuidelines: options.DefaultGuidelines, StockGuidelines: options.StockGuidelines, Guidelines: options.Guidelines,
		Memories: options.Memories, Current: options.Storyline, Request: options.Request,
	}
	request, sources := preparePlanRequest(input, options.OriginProtocolVersion, options.CompletionTokens, originAttachmentIDs(post.Images), options.PlanOrigins)
	progress("storyline", 0, 1)
	result, err := s.storylineCall(ctx, job.WriteModel, options.WriteNativeEffort, request, shown, sources, options.OriginProtocolVersion)
	if err != nil {
		return err
	}
	var madeWith []string
	if post.Storyline != nil {
		madeWith = append([]string(nil), post.Storyline.MadeWith...)
	}
	result.MadeWith = madeWith
	var publishErr error
	if options.OriginProtocolVersion == OriginProtocolVersion {
		result.Origins = PreservePlanOrigins(options.Storyline, options.PlanOrigins, result.Paragraphs, sources, result.OriginCandidates)
		publishErr = s.originPlans.PublishStorylineResult(ctx, post.UserID, post.Slug, OriginStorylineCompletion{Storyline: result, ExpectedContentRevision: post.ContentRevision, ExpectedInputRevision: options.ExpectedInputRevision, ExpectedPlanFingerprint: options.ExpectedPlanFingerprint})
	} else {
		publishErr = s.posts.SetStoryline(ctx, post.UserID, post.Slug, result)
	}
	if err := publishErr; err != nil {
		return fmt.Errorf("persist storyline: %w", err)
	}
	s.bindPublishedRequestCapturePlan(ctx, post.UserID, post.Slug, result)
	progress("storyline", 1, 1)
	return nil
}

// storylineCall is the one call both storyline jobs make: the write model at low reasoning with
// the storyline's budget for the native-effort flag the start froze, answered in the storyline
// schema and parsed against what was shown. A bad or cut-off answer fails as GEN-20 says
// (MODEL_OUTPUT_INVALID, MODEL_OUTPUT_TRUNCATED).
func (s *Service) storylineCall(ctx context.Context, writeModel string, nativeEffort bool, request llm.Request, shown []string, sources []postdomain.OriginSource, protocol int) (Storyline, error) {
	model, ok := parseModelRef(writeModel)
	if !ok {
		return Storyline{}, ErrWriteModelRequired
	}
	request = s.prepareStorylineCall(request, model, nativeEffort, protocol)
	response, err := s.completePostRequest(ctx, model, request, captureAttachments(ctx, shown))
	if err != nil {
		return Storyline{}, providerCallError("스토리라인 작성", err)
	}
	paragraphs, candidates, err := ParseStorylineAnswerWithOrigins(response.Text, shown)
	if err != nil {
		return Storyline{}, responseParseError(response, err)
	}
	result := Storyline{Paragraphs: paragraphs, OriginCandidates: candidates}
	if protocol == OriginProtocolVersion {
		review := ValidatePlanOrigins(paragraphs, sources, candidates)
		result.Origins = &review
	}
	return result, nil
}

// storylineMaterial is what both storyline jobs freeze beside their own members: the brief,
// the 지침 and the 기억 (GEN-68). No quality rules — they are measured on a post, not a plan.
type storylineMaterial struct {
	Template          *TemplateBrief
	Guidelines        []string
	DefaultGuidelines []string
	StockGuidelines   []StockGuideline
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
	return storylineMaterial{Template: brief, Guidelines: guidelines.Owner, DefaultGuidelines: guidelines.Defaults, StockGuidelines: cloneStockGuidelines(guidelines.Stock), Memories: memories}, nil
}

// storylineOptions is what a storyline job froze at enqueue.
type storylineOptions struct {
	ExpectedPlanFingerprint *string
	ExpectedInputRevision   *int64
	OriginProtocolVersion   int
	CompletionTokens        int
	TargetLanguage          Language
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
	post.OriginProtocolVersion, post.OriginCompletionTokens = o.OriginProtocolVersion, o.CompletionTokens
	post.TargetLanguage = o.TargetLanguage
	post.Template = o.Template
	post.Guidelines = o.Guidelines
	post.DefaultGuidelines = o.DefaultGuidelines
	post.StockGuidelines = cloneStockGuidelines(o.StockGuidelines)
	post.Memories = o.Memories
	return post
}

// storylineRevisionOptions is what a storyline request froze at enqueue.
type storylineRevisionOptions struct {
	ExpectedPlanFingerprint *string
	ExpectedInputRevision   *int64
	OriginProtocolVersion   int
	CompletionTokens        int
	PlanOrigins             *PlanOriginReview
	TargetLanguage          Language
	Request                 string
	Storyline               []StorylineParagraph
	storylineMaterial
	Observations      []Observation
	WriteNativeEffort bool
}

type storylinePayload struct {
	ExpectedPlanFingerprint *string               `json:"expected_plan_fingerprint,omitempty"`
	ExpectedInputRevision   *int64                `json:"expected_input_revision,omitempty"`
	OriginProtocolVersion   int                   `json:"origin_protocol_version,omitempty"`
	CompletionTokens        int                   `json:"completion_tokens,omitempty"`
	TargetLanguage          string                `json:"target_language"`
	Template                *templatePayload      `json:"template,omitempty"`
	Guidelines              []string              `json:"guidelines,omitempty"`
	StockGuidelines         *[]stockGuidelineJSON `json:"stock_guidelines,omitempty"`
	DefaultGuidelines       []string              `json:"default_guidelines,omitempty"`
	Memories                []string              `json:"memories,omitempty"`
	ObserveFiles            *[]string             `json:"observe_files"`
	Observations            []observationPayload  `json:"observations,omitempty"`
	WriteNativeEffort       bool                  `json:"write_native_effort,omitempty"`
}

type storylineRevisionPayload struct {
	ExpectedPlanFingerprint *string                  `json:"expected_plan_fingerprint,omitempty"`
	ExpectedInputRevision   *int64                   `json:"expected_input_revision,omitempty"`
	OriginProtocolVersion   int                      `json:"origin_protocol_version,omitempty"`
	CompletionTokens        int                      `json:"completion_tokens,omitempty"`
	PlanOrigins             *planOriginReviewJSON    `json:"plan_origins,omitempty"`
	TargetLanguage          string                   `json:"target_language"`
	Request                 string                   `json:"request"`
	Storyline               []storylineParagraphJSON `json:"storyline"`
	Template                *templatePayload         `json:"template,omitempty"`
	Guidelines              []string                 `json:"guidelines,omitempty"`
	StockGuidelines         *[]stockGuidelineJSON    `json:"stock_guidelines,omitempty"`
	DefaultGuidelines       []string                 `json:"default_guidelines,omitempty"`
	Memories                []string                 `json:"memories,omitempty"`
	Observations            []observationPayload     `json:"observations,omitempty"`
	WriteNativeEffort       bool                     `json:"write_native_effort,omitempty"`
}

func encodeStorylinePayload(options storylineOptions) ([]byte, error) {
	if !options.TargetLanguage.Valid() {
		return nil, ErrLanguageRequired
	}
	return json.Marshal(storylinePayload{
		ExpectedPlanFingerprint: options.ExpectedPlanFingerprint,
		ExpectedInputRevision:   options.ExpectedInputRevision,
		OriginProtocolVersion:   options.OriginProtocolVersion, CompletionTokens: options.CompletionTokens,
		TargetLanguage:    options.TargetLanguage.String(),
		Template:          encodeTemplate(options.Template),
		Guidelines:        cloneTexts(options.Guidelines),
		DefaultGuidelines: cloneTexts(options.DefaultGuidelines), StockGuidelines: encodeStockGuidelines(options.StockGuidelines),
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
	if payload.OriginProtocolVersion != 0 && (payload.OriginProtocolVersion != OriginProtocolVersion || payload.CompletionTokens <= 0) {
		return storylineOptions{}, fmt.Errorf("invalid storyline origin protocol or admitted cap")
	}
	language, err := payloadLanguage(payload.TargetLanguage)
	if err != nil {
		return storylineOptions{}, fmt.Errorf("decode storyline payload: %w", err)
	}
	return storylineOptions{
		ExpectedPlanFingerprint: payload.ExpectedPlanFingerprint,
		ExpectedInputRevision:   payload.ExpectedInputRevision,
		OriginProtocolVersion:   payload.OriginProtocolVersion, CompletionTokens: payload.CompletionTokens,
		TargetLanguage: language,
		storylineMaterial: storylineMaterial{
			Template:          decodeTemplate(payload.Template),
			Guidelines:        cloneTexts(payload.Guidelines),
			DefaultGuidelines: cloneTexts(payload.DefaultGuidelines), StockGuidelines: decodeStockGuidelines(payload.StockGuidelines),
			Memories: cloneTexts(payload.Memories),
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
		ExpectedPlanFingerprint: options.ExpectedPlanFingerprint,
		ExpectedInputRevision:   options.ExpectedInputRevision,
		OriginProtocolVersion:   options.OriginProtocolVersion, CompletionTokens: options.CompletionTokens, PlanOrigins: encodePlanOrigins(options.PlanOrigins),
		TargetLanguage:    options.TargetLanguage.String(),
		Request:           options.Request,
		Storyline:         storylineForPrompt(options.Storyline)["storyline"],
		Template:          encodeTemplate(options.Template),
		Guidelines:        cloneTexts(options.Guidelines),
		DefaultGuidelines: cloneTexts(options.DefaultGuidelines), StockGuidelines: encodeStockGuidelines(options.StockGuidelines),
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
	if payload.OriginProtocolVersion != 0 && (payload.OriginProtocolVersion != OriginProtocolVersion || payload.CompletionTokens <= 0) {
		return storylineRevisionOptions{}, fmt.Errorf("invalid storyline revision origin protocol or admitted cap")
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
		ExpectedPlanFingerprint: payload.ExpectedPlanFingerprint,
		ExpectedInputRevision:   payload.ExpectedInputRevision,
		OriginProtocolVersion:   payload.OriginProtocolVersion, CompletionTokens: payload.CompletionTokens, PlanOrigins: decodePlanOrigins(payload.PlanOrigins),
		TargetLanguage: language,
		Request:        payload.Request,
		Storyline:      paragraphs,
		storylineMaterial: storylineMaterial{
			Template:          decodeTemplate(payload.Template),
			Guidelines:        cloneTexts(payload.Guidelines),
			DefaultGuidelines: cloneTexts(payload.DefaultGuidelines), StockGuidelines: decodeStockGuidelines(payload.StockGuidelines),
			Memories: cloneTexts(payload.Memories),
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

func (s *Service) prepareStorylineCall(request llm.Request, model llm.ModelRef, nativeEffort bool, protocol int) llm.Request {
	request.Reasoning = llm.ReasoningLow
	if request.MaxTokens <= 0 {
		request.MaxTokens = s.budget.Storyline(nativeEffort)
	}
	schema := StorylineAnswerSchema()
	if protocol == 0 {
		schema = LegacyStorylineAnswerSchema()
	}
	setOriginOutput(&request, "StorylineAnswer", schema, protocol)
	if info, found := s.models.Resolve(model); found && info.StructuredOutput {
		request.JSONSchema = schema
	}
	return request
}

// preparePlanRequest owns both create/rewrite material and origin composition.
// Preview and admitted plan execution call this same pure assembly.
func preparePlanRequest(input StorylinePromptInput, protocol, completionTokens int, attachmentIDs map[string]string, origins *PlanOriginReview) (llm.Request, []postdomain.OriginSource) {
	request := ComposeStorylineRequest(input)
	var sources []postdomain.OriginSource
	if protocol == OriginProtocolVersion {
		sources = WritingOriginSources(WritePromptInput{Title: input.Title, Memo: input.Memo, Template: input.Template, Memories: input.Memories, Photos: input.Photos, Videos: input.Videos, Observations: input.Observations}, false, attachmentIDs)
		var projection any
		if input.Request != "" {
			prior := ValidateStoredPlanOrigins(input.Current, origins)
			sources = catalogWithPriorPlan(sources, input.Current, prior, input.Photos, input.Videos)
			catalog := originCatalog{sources: sources}
			for _, source := range sources {
				catalog.chars += utf8.RuneCountInString(source.Text)
			}
			catalog.add("current.edit", postdomain.OriginSourceOwnerEdit, input.Request, "", true)
			sources = catalog.sources
			projection = priorPlanProjection(prior)
		}
		request = appendOriginRequest(request, sources, projection)
		request.MaxTokens = completionTokens
	}
	return request, sources
}
