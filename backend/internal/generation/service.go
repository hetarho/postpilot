package generation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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
	samples     VersionSampleWriter
	batchSize   int
	reasoning   ReasoningPolicy
	budget      CompletionBudget
}

type ReasoningPolicy struct {
	Observe llm.ReasoningEffort
	Write   llm.ReasoningEffort
}

// CompletionBudget is the per-stage completion cap policy, received from its owner
// (internal/platform/config) rather than computed here: this context asks for the budget its
// work needs and holds no number of its own.
type CompletionBudget interface {
	// Write is the writing stage's cap for a post's requested target length.
	Write(targetLength *int) int
	// Revise is a revision's cap. A revision re-emits the whole PostContent, so its budget
	// has to fit what already exists and not only what was asked for.
	Revise(contentChars int, targetLength *int) int
	// Observation is one observation batch's cap, independent of the writer's.
	Observation() int
}

func NewService(posts Posts, profiles Profiles, rules RuleWriter, models LLM, images ImageReader, jobs Jobs, batchSize int, reasoning ReasoningPolicy, budget CompletionBudget) *Service {
	if batchSize <= 0 {
		panic("generation: batch size must be positive")
	}
	if !reasoning.Observe.Valid() || !reasoning.Write.Valid() {
		panic("generation: reasoning policy is invalid")
	}
	if budget == nil {
		panic("generation: a completion budget policy is required")
	}
	return &Service{posts: posts, profiles: profiles, rules: rules, models: models, images: images, jobs: jobs, batchSize: batchSize, reasoning: reasoning, budget: budget}
}

func (s *Service) SetPendingExperimentFinder(finder PendingExperiments) {
	s.experiments = finder
}

// SetTemplateBriefs wires the template context's published brief lookup. Without it the
// prompt simply carries no brief, so a partially wired process keeps the no-template
// behavior rather than failing.
func (s *Service) SetTemplateBriefs(briefs TemplateBriefs) { s.templates = briefs }

// SetVersionSamples wires the voice context's per-version snapshot recorder. Without it a
// generation simply records nothing, which is the same outcome a failed recording has: the
// post is the product and the snapshot is a record of it.
func (s *Service) SetVersionSamples(writer VersionSampleWriter) { s.samples = writer }

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

// SetGuidelines wires the guideline context's published resolution. Without it the prompt
// simply carries no 지침, so a partially wired process keeps the no-guideline behavior
// rather than failing.
func (s *Service) SetGuidelines(resolver GuidelinesForPrompt) { s.guidelines = resolver }

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
	if post.Content == nil {
		return "", ErrRevisionContentRequired
	}
	if post.ContentLanguage == nil || !post.ContentLanguage.Valid() {
		return "", ErrContentLanguageRequired
	}
	request.ContentLanguage = *post.ContentLanguage
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
	if !ok || !modelEnabled(s.models, write, llm.StageNameWrite) {
		return "", ErrWriteModelRequired
	}
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
	texts, err := s.freezeGuidelines(ctx, post)
	if err != nil {
		return "", err
	}
	request.Guidelines = texts
	payload, err := encodeRevisionPayloadForLanguage(request.Instruction, request.SaveAsRule, request.ContentLanguage, brief, texts)
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
	write, ok := parseModelRef(request.WriteModel)
	if !ok || !modelEnabled(s.models, write, llm.StageNameWrite) {
		return "", ErrWriteModelRequired
	}
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
	}
	if request.TargetLength != nil && *request.TargetLength <= 0 {
		return "", ErrInvalidTargetLength
	}
	brief, err := s.freezeTemplate(ctx, post)
	if err != nil {
		return "", err
	}
	request.Template = brief
	texts, err := s.freezeGuidelines(ctx, post)
	if err != nil {
		return "", err
	}
	request.Guidelines = texts
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
	request.ObserveCalls = s.observeCalls(observeTargetCount(post.Images, request.ObserveFiles))
	id, err := s.jobs.EnqueueGeneration(ctx, request)
	if err != nil {
		return "", fmt.Errorf("enqueue generation: %w", err)
	}
	return id, nil
}

// observeCalls is how many observation calls a photo count takes at the configured batch
// size. It mirrors the loop in observe.go: the hold and the work must agree on how many
// calls there will be, or the hold prices the wrong job.
func (s *Service) observeCalls(photos int) int {
	if photos <= 0 || s.batchSize <= 0 {
		return 0
	}
	return (photos + s.batchSize - 1) / s.batchSize
}

// observeTargetCount is how many photos the frozen decision actually observes. Nil is the
// no-picker case, which observes every attached photo.
func observeTargetCount(images []Image, observeFiles *[]string) int {
	if observeFiles == nil {
		return len(images)
	}
	return len(*observeFiles)
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
	brief, ok, err := s.templates.RenderedFor(ctx, post.UserID, post.TemplateID, postFilenames(post))
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
// the brief was resolved from — one read, one consistent view. Editing, rescoping or
// deleting a guideline afterwards cannot reach the queued work, including across a
// restart-resume or an explicit retry, because the handlers read only the payload.
func (s *Service) freezeGuidelines(ctx context.Context, post PostInput) ([]string, error) {
	if s.guidelines == nil {
		return nil, nil
	}
	var templateID *string
	if post.TemplateID != "" {
		id := post.TemplateID
		templateID = &id
	}
	texts, err := s.guidelines.ForPrompt(ctx, post.UserID, templateID)
	if err != nil {
		return nil, fmt.Errorf("load applicable guidelines: %w", err)
	}
	return texts, nil
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
