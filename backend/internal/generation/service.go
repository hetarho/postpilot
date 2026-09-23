package generation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/llm"
)

type Service struct {
	posts       Posts
	profiles    Profiles
	rules       RuleWriter
	models      LLM
	images      ImageReader
	jobs        Jobs
	experiments PendingExperiments
	templates   TemplateBriefs
	guidelines  GuidelinesForPrompt
	memories    MemoriesForPrompt
	candidates  GuidelineCandidates
	samples     VersionSampleWriter
	batchSize   int
	videos      VideoLinker
	videoURLTTL time.Duration
	reasoning   ReasoningPolicy
	budget      CompletionBudget
}

type ReasoningPolicy struct {
	Observe llm.ReasoningEffort
	Write   llm.ReasoningEffort
}

// DefaultReasoningPolicy is code-owned because changing a stage's reasoning
// strength changes generation behaviour rather than deployment topology
// (ARCH-21). A model-level registry override still wins.
//
// Analyze has no field on purpose: policy/providers.md requires it to send no
// effort, and a request that carries no stage value already sends none
// (registry.go forwards only a resolved effort). Adding the field back would be
// a second place for one rule to live, which is how it previously came to be
// set, asserted, and forwarded nowhere.
func DefaultReasoningPolicy() ReasoningPolicy {
	return ReasoningPolicy{Observe: llm.ReasoningLow, Write: llm.ReasoningLow}
}

// CompletionBudget is the per-stage completion cap policy, received from its owner
// (cmd/api, from the platform completion budget) rather than computed here: this context asks for the budget its
// work needs and holds no number of its own.
type CompletionBudget interface {
	// Write is the writing stage's cap for a post's requested target length.
	Write(targetLength *int, nativeEffort bool) int
	// Revise is a revision's cap. A revision re-emits the whole PostContent, so its budget
	// has to fit what already exists and not only what was asked for.
	Revise(contentChars int, targetLength *int, nativeEffort bool) int
	// Observation is one observation batch's cap, independent of the writer's.
	Observation() int
}

// Deps are the collaborators from other contexts a generation run reads or writes
// through (ARCH-40). Every one is required: a missing template brief, 지침, candidate
// recorder, version-sample writer or experiment finder would not fail a run, it would
// silently produce a poorer or unrecorded post — exactly the wire nobody notices.
type Deps struct {
	// Experiments answers whether a post has a pending A/B run (generation must not
	// overwrite what an experiment is about to judge).
	Experiments PendingExperiments
	// Templates is the template context's published brief lookup, read once at enqueue.
	Templates TemplateBriefs
	// Guidelines is the guideline context's published resolution, read once at enqueue.
	Guidelines GuidelinesForPrompt
	// Memories is the memory context's published retrieval, read once at enqueue and only
	// for a post that opted in.
	Memories MemoriesForPrompt
	// Candidates records what a completed revision asked for.
	Candidates GuidelineCandidates
	// Samples is the voice context's per-version snapshot recorder.
	Samples VersionSampleWriter
	// Videos mints the signed link a video reaches a model through (VIDEO-10); the
	// bytes never enter this process. VideoURLTTL is that link's lifetime.
	Videos      VideoLinker
	VideoURLTTL time.Duration
}

func NewService(posts Posts, profiles Profiles, rules RuleWriter, models LLM, images ImageReader, jobs Jobs, batchSize int, reasoning ReasoningPolicy, budget CompletionBudget, deps Deps) *Service {
	if batchSize <= 0 {
		panic("generation: batch size must be positive")
	}
	if !reasoning.Observe.Valid() || !reasoning.Write.Valid() {
		panic("generation: reasoning policy is invalid")
	}
	if budget == nil {
		panic("generation: a completion budget policy is required")
	}
	for name, dep := range map[string]any{"experiments": deps.Experiments, "template briefs": deps.Templates, "guidelines": deps.Guidelines, "memories": deps.Memories, "guideline candidates": deps.Candidates, "version samples": deps.Samples, "video linker": deps.Videos} {
		if dep == nil {
			panic("generation: " + name + " collaborator is required")
		}
	}
	if deps.VideoURLTTL <= 0 {
		panic("generation: video link TTL must be positive")
	}
	return &Service{posts: posts, profiles: profiles, rules: rules, models: models, images: images, jobs: jobs, batchSize: batchSize, reasoning: reasoning, budget: budget,
		experiments: deps.Experiments, templates: deps.Templates, guidelines: deps.Guidelines, memories: deps.Memories, candidates: deps.Candidates, samples: deps.Samples, videos: deps.Videos, videoURLTTL: deps.VideoURLTTL}
}

// recordVersionSample copies what a run produced into the voice's current head version. It is
// called AFTER the machine baseline is written, and its failure is swallowed on template: a
// snapshot is a record of a post, and losing the record must never lose the post ([I1] is about
// history outliving its subject, not the other way round). The voice id is the one the run was
// frozen against, so a reassignment mid-run cannot file the snapshot under the wrong profile.
func (s *Service) recordVersionSample(ctx context.Context, userID, voiceID string, content PostContent) {
	if s.samples == nil || voiceID == "" {
		return
	}
	if err := s.samples.RecordVersionSample(ctx, userID, voiceID, content); err != nil {
		slog.WarnContext(ctx, "record voice version sample failed", "error", err, "voice_id", voiceID)
	}
}

// recordGuidelineCandidate records what the user ASKED FOR, after the revised content is
// already persisted. Its failure is swallowed for the same reason recordVersionSample's is:
// the result the user is looking at is authoritative, and turning a bookkeeping error into a
// failed job would throw that work away. Nothing is recorded for a failed, cancelled or
// still-running revision, which follows from the call position alone.
func (s *Service) recordGuidelineCandidate(ctx context.Context, userID, postSlug, instruction string) {
	if s.candidates == nil {
		return
	}
	if err := s.candidates.Record(ctx, userID, postSlug, instruction); err != nil {
		slog.WarnContext(ctx, "record guideline candidate failed", "error", err, "post_slug", postSlug)
	}
}

func (s *Service) refusePendingExperiment(ctx context.Context, userID, postSlug string) error {
	if s.experiments == nil {
		return nil
	}
	id, err := s.experiments.PendingForPost(ctx, userID, postSlug)
	if err != nil {
		return err
	}
	if id != "" {
		return &JobAlreadyInProgressError{ActiveID: id}
	}
	return nil
}

func (s *Service) StartRevision(ctx context.Context, request StartRevisionRequest) (string, error) {
	request.Instruction = strings.TrimSpace(request.Instruction)
	if request.Instruction == "" {
		return "", ErrRevisionInstructionRequired
	}
	if utf8.RuneCountInString(request.Instruction) > RevisionInstructionMaxChars {
		return "", ErrRevisionInstructionTooLong
	}
	post, err := s.posts.AttachedImages(ctx, request.UserID, request.PostSlug)
	if err != nil {
		return "", err
	}
	// Ahead of everything else, the rule append included: that writes to the voice, and a
	// revision that cannot land must not teach it anything (GEN-56).
	if post.Published {
		return "", ErrPostPublished
	}
	if post.Content == nil {
		return "", ErrRevisionContentRequired
	}
	if post.ContentLanguage == nil || !post.ContentLanguage.Valid() {
		return "", ErrContentLanguageRequired
	}
	request.ContentLanguage = *post.ContentLanguage
	request.TargetLength = cloneOptionalInt(post.TargetLength)
	request.TagCount = resolveTagCount(post.TagCount)
	request.ContentChars = contentChars(post.Content)
	voiceID, err := activeVoice(post)
	if err != nil {
		return "", err
	}
	request.VoiceID = voiceID
	if request.SaveAsRule && (!post.Voice.SourceLanguage.Valid() || *post.ContentLanguage != post.Voice.SourceLanguage) {
		return "", ErrVoiceContentLanguageMismatch
	}
	if err := s.refusePendingExperiment(ctx, request.UserID, request.PostSlug); err != nil {
		return "", err
	}
	write, ok := parseModelRef(request.WriteModel)
	writeInfo, found := s.models.Resolve(write)
	if !ok || !found || writeInfo.Disabled || !writeInfo.ServesStage(llm.StageNameWrite) {
		return "", ErrWriteModelRequired
	}
	request.WriteNativeEffort = writeInfo.ReasoningNativeEffort
	if request.SaveAsRule {
		if err := s.rules.AppendRule(ctx, request.UserID, voiceID, request.Instruction); err != nil {
			return "", fmt.Errorf("save revision rule: %w", err)
		}
	}
	brief, err := s.freezeTemplate(ctx, post)
	if err != nil {
		return "", err
	}
	request.Template = brief
	texts, err := s.freezeGuidelines(ctx, post, true)
	if err != nil {
		return "", err
	}
	request.Guidelines = texts
	payload, err := encodeRevisionPayloadForLanguage(request.Instruction, request.SaveAsRule, request.ContentLanguage, brief, texts, request.TagCount, request.WriteNativeEffort)
	if err != nil {
		return "", fmt.Errorf("encode revision payload: %w", err)
	}
	id, err := s.jobs.EnqueueRevision(ctx, request, payload)
	if err != nil {
		return "", fmt.Errorf("enqueue revision: %w", err)
	}
	return id, nil
}

func (s *Service) Start(ctx context.Context, request StartRequest) (string, error) {
	post, err := s.posts.AttachedImages(ctx, request.UserID, request.PostSlug)
	if err != nil {
		return "", err
	}
	// Before anything is frozen, held or queued (GEN-56).
	if post.Published {
		return "", ErrPostPublished
	}
	if !post.TargetLanguage.Valid() {
		return "", ErrLanguageRequired
	}
	request.TargetLanguage = post.TargetLanguage
	// From the post, never from the request: there is no per-run override to carry (GEN-46).
	request.TagCount = resolveTagCount(post.TagCount)
	voiceID, err := activeVoice(post)
	if err != nil {
		return "", err
	}
	request.VoiceID = voiceID
	if err := s.refusePendingExperiment(ctx, request.UserID, request.PostSlug); err != nil {
		return "", err
	}
	write, ok := parseModelRef(request.WriteModel)
	writeInfo, found := s.models.Resolve(write)
	if !ok || !found || writeInfo.Disabled || !writeInfo.ServesStage(llm.StageNameWrite) {
		return "", ErrWriteModelRequired
	}
	request.WriteNativeEffort = writeInfo.ReasoningNativeEffort
	if len(post.Images) == 0 {
		request.ObserveModel = ""
		// A zero-photo post has no reuse decision to make, so nothing about the picker is
		// frozen for it: the run observes nothing and clears the snapshot, as it always has.
		request.ObserveFiles = nil
		request.Observations = nil
	} else {
		observe, valid := parseModelRef(request.ObserveModel)
		if !valid || !modelEnabled(s.models, observe, llm.StageNameObserve) {
			return "", ErrObserveModelRequired
		}
		if err := s.refuseVideoBlindObserveModel(post.Images, observe); err != nil {
			return "", err
		}
	}
	if request.TargetLength != nil && *request.TargetLength <= 0 {
		return "", ErrInvalidTargetLength
	}
	brief, err := s.freezeTemplate(ctx, post)
	if err != nil {
		return "", err
	}
	request.Template = brief
	texts, err := s.freezeGuidelines(ctx, post, false)
	if err != nil {
		return "", err
	}
	request.Guidelines = texts
	memories, err := s.freezeMemories(ctx, post)
	if err != nil {
		return "", err
	}
	request.Memories = memories
	if len(post.Images) > 0 {
		// Both halves of the reuse decision are resolved HERE, from one read of the post,
		// and frozen into the payload by the enqueue. Attaching a photo, deleting one or
		// switching the observation model afterwards cannot reach the queued run.
		files, carried := freezeObserveSelection(post.Images, post.Observations, request.ObserveFiles)
		request.ObserveFiles = &files
		request.Observations = carried
	}
	// Priced over the FROZEN set, never over the attached count: a run that reuses every
	// observation makes no observation call and must not be held for fifteen of them.
	request.ObserveCalls = s.observeCalls(observeTargets(post.Images, request.ObserveFiles))
	id, err := s.jobs.EnqueueGeneration(ctx, request)
	if err != nil {
		return "", fmt.Errorf("enqueue generation: %w", err)
	}
	return id, nil
}

// observeCalls is how many observation calls a frozen selection takes: the photo batches at
// the configured batch size, plus ONE per video (VIDEO-8, VIDEO-13). It mirrors the loop in
// observe.go — the hold and the work must agree on how many calls there will be, or the hold
// prices the wrong job.
func (s *Service) observeCalls(targets []Image) int {
	photos := len(photosOf(targets))
	calls := len(videosOf(targets))
	if photos > 0 && s.batchSize > 0 {
		calls += (photos + s.batchSize - 1) / s.batchSize
	}
	return calls
}

// refuseVideoBlindObserveModel is the per-RUN video capability check (VIDEO-11): watching a
// clip is not a purpose of its own, so a video-blind model still serves every post without
// one — and is refused, by name, for a post with one.
//
// The missing-observe-model refusal runs first, so a post with videos and no model chosen
// still hears the simpler thing it has to fix.
func (s *Service) refuseVideoBlindObserveModel(images []Image, observe llm.ModelRef) error {
	if len(videosOf(images)) == 0 {
		return nil
	}
	info, found := s.models.Resolve(observe)
	if !found || !info.VideoInput || !info.VideoDelivery.SignedVideoURL {
		return &VideoUnsupportedError{Model: observe.String()}
	}
	return nil
}

// observeTargets is what the frozen decision actually observes, as attachments rather than a
// count: a photo and a video cost different numbers of calls, so the pricing has to know which
// is which. Nil is the no-picker case, which observes every attachment.
func observeTargets(images []Image, observeFiles *[]string) []Image {
	if observeFiles == nil {
		return images
	}
	selected := make(map[string]struct{}, len(*observeFiles))
	for _, filename := range *observeFiles {
		selected[filename] = struct{}{}
	}
	out := make([]Image, 0, len(images))
	for _, image := range images {
		if _, ok := selected[image.Filename]; ok {
			out = append(out, image)
		}
	}
	return out
}

// freezeTemplate resolves the post's CURRENT template once, at enqueue, expanded for the
// post's CURRENT attachments, so the text the worker prompts with is decided here and never
// re-read. A template deleted between the save and the start is simply absent — that is a
// post with no template, not a failure.
//
// Expansion happens inside the freeze rather than at prompt time on purpose: it is what
// makes "attaching a photo after the start cannot change the run" true, and it is the only
// place the expansion bound can refuse before a provider is called.
func (s *Service) freezeTemplate(ctx context.Context, post PostInput) (*TemplateBrief, error) {
	if s.templates == nil || post.TemplateID == "" {
		return nil, nil
	}
	brief, ok, err := s.templates.RenderedFor(ctx, post.UserID, post.TemplateID, postFilenames(post), post.TemplateAnswers)
	if err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}
	if !ok {
		return nil, nil
	}
	frozen := brief
	return &frozen, nil
}

// postFilenames is the attachment order every stage refers to a photo by.
func postFilenames(post PostInput) []string {
	names := make([]string, 0, len(post.Images))
	for _, image := range post.Images {
		names = append(names, image.Filename)
	}
	return names
}

// freezeGuidelines resolves the applicable 지침 once, at enqueue, from the SAME template id
// the brief was resolved from and the post's 분야 — one read, one consistent view. Editing,
// rescoping or deleting a guideline, or switching the preset, afterwards cannot reach the
// queued work, including across a restart-resume or an explicit retry, because the handlers
// read only the payload. forRevision leaves the preset line out (GEN-57, GUIDE-17).
func (s *Service) freezeGuidelines(ctx context.Context, post PostInput, forRevision bool) ([]string, error) {
	if s.guidelines == nil {
		return nil, nil
	}
	var templateID, field *string
	if post.TemplateID != "" {
		id := post.TemplateID
		templateID = &id
	}
	if post.Field != "" {
		id := post.Field
		field = &id
	}
	texts, err := s.guidelines.ForPrompt(ctx, post.UserID, templateID, field, forRevision)
	if err != nil {
		return nil, fmt.Errorf("load applicable guidelines: %w", err)
	}
	return texts, nil
}

// freezeMemories retrieves the post's memories once, at enqueue, and only when the post
// opted in (MEM-18). Editing or deleting a memory afterwards cannot reach the queued work —
// including across a restart-resume or an explicit retry — because the handlers read only
// the payload, exactly as they do for the guideline texts above.
//
// The revise pass has no equivalent and never will: it holds neither the memo nor the
// observations, so material it cannot check against would license rewriting sentences the
// request never touched (MEM-22).
func (s *Service) freezeMemories(ctx context.Context, post PostInput) ([]string, error) {
	if s.memories == nil || !post.UseMemory {
		return nil, nil
	}
	texts, err := s.memories.ForPost(ctx, post.UserID, memoryKeyParts(post))
	if err != nil {
		return nil, fmt.Errorf("retrieve memories: %w", err)
	}
	return texts, nil
}

// memoryKeyParts is the retrieval key (MEM-7): what this post is about, in the post's own
// words. The memo and the 가제 are what the author wrote; the template answers are the facts
// they filled in; and an observation contributes only `objects` and `visible_text` — the
// nouns and the letters actually in the frame — because `scene` and `mood` are the observer's
// prose and would match a tag on a word nobody in this post wrote.
func memoryKeyParts(post PostInput) []string {
	parts := make([]string, 0, 2+len(post.TemplateAnswers)+2*len(post.Observations))
	parts = append(parts, post.Memo, post.Title)
	for _, answer := range post.TemplateAnswers {
		if answer.Enabled {
			parts = append(parts, answer.Text)
		}
	}
	for _, observation := range post.Observations {
		parts = append(parts, observation.Objects...)
		parts = append(parts, observation.VisibleText)
	}
	return parts
}

func (s *Service) GetJob(ctx context.Context, id, userID string) (*JobSummary, error) {
	return s.jobs.GetGeneration(ctx, id, userID)
}

// activeVoice is the pre-enqueue gate: a post in a deleted voice gets no generation,
// revision or rule, and a post with no voice at all is a data error, not a fallback case.
func activeVoice(post PostInput) (string, error) {
	if post.Voice.ID == "" {
		return "", ErrVoiceRequired
	}
	if post.Voice.Deleted {
		return "", ErrVoiceDeleted
	}
	return post.Voice.ID, nil
}

// frozenVoice is the handler-side recheck: the job carries the voice it was queued for, and
// the result may only land if the post still belongs to that voice and the voice is alive.
// Jobs queued before voices existed carry no id and skip the mismatch half.
func frozenVoice(post PostInput, jobVoiceID string) (string, error) {
	voiceID, err := activeVoice(post)
	if err != nil {
		return "", err
	}
	if jobVoiceID != "" && jobVoiceID != voiceID {
		return "", ErrVoiceMismatch
	}
	return voiceID, nil
}

// modelEnabled requires stage membership, not mere registry presence: a ref arrives here
// straight from the client, so the per-template registration (change 20) is enforced at
// this boundary too, not only in the picker.
func modelEnabled(models LLM, ref llm.ModelRef, stage string) bool {
	info, ok := models.Resolve(ref)
	return ok && !info.Disabled && info.ServesStage(stage)
}

func parseModelRef(value string) (llm.ModelRef, bool) {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return llm.ModelRef{}, false
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}, true
}
