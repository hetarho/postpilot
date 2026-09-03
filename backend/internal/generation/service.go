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
	purposes    PurposeBriefs
	guidelines  GuidelinesForPrompt
	samples     VersionSampleWriter
	batchSize   int
	reasoning   ReasoningPolicy
}

type ReasoningPolicy struct {
	Observe llm.ReasoningEffort
	Write   llm.ReasoningEffort
}

func NewService(posts Posts, profiles Profiles, rules RuleWriter, models LLM, images ImageReader, jobs Jobs, batchSize int, reasoning ReasoningPolicy) *Service {
	if batchSize <= 0 {
		panic("generation: batch size must be positive")
	}
	if !reasoning.Observe.Valid() || !reasoning.Write.Valid() {
		panic("generation: reasoning policy is invalid")
	}
	return &Service{posts: posts, profiles: profiles, rules: rules, models: models, images: images, jobs: jobs, batchSize: batchSize, reasoning: reasoning}
}

func (s *Service) SetPendingExperimentFinder(finder PendingExperiments) {
	s.experiments = finder
}

// SetPurposeBriefs wires the purpose context's published brief lookup. Without it the
// prompt simply carries no brief, so a partially wired process keeps the no-purpose
// behavior rather than failing.
func (s *Service) SetPurposeBriefs(briefs PurposeBriefs) { s.purposes = briefs }

// SetVersionSamples wires the voice context's per-version snapshot recorder. Without it a
// generation simply records nothing, which is the same outcome a failed recording has: the
// post is the product and the snapshot is a record of it.
func (s *Service) SetVersionSamples(writer VersionSampleWriter) { s.samples = writer }

// recordVersionSample copies what a run produced into the voice's current head version. It is
// called AFTER the machine baseline is written, and its failure is swallowed on purpose: a
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
	brief, err := s.freezePurpose(ctx, post)
	if err != nil {
		return "", err
	}
	request.Purpose = brief
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
	} else {
		observe, valid := parseModelRef(request.ObserveModel)
		if !valid || !modelEnabled(s.models, observe, llm.StageNameObserve) {
			return "", ErrObserveModelRequired
		}
	}
	if request.TargetLength != nil && *request.TargetLength <= 0 {
		return "", ErrInvalidTargetLength
	}
	brief, err := s.freezePurpose(ctx, post)
	if err != nil {
		return "", err
	}
	request.Purpose = brief
	texts, err := s.freezeGuidelines(ctx, post)
	if err != nil {
		return "", err
	}
	request.Guidelines = texts
	request.ObserveCalls = s.observeCalls(len(post.Images))
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

// freezePurpose resolves the post's CURRENT purpose once, at enqueue, so the text the
// worker prompts with is decided here and never re-read. A purpose deleted between the save
// and the start is simply absent — that is a post with no purpose, not a failure.
func (s *Service) freezePurpose(ctx context.Context, post PostInput) (*PurposeBrief, error) {
	if s.purposes == nil || post.PurposeID == "" {
		return nil, nil
	}
	brief, ok, err := s.purposes.BriefFor(ctx, post.UserID, post.PurposeID)
	if err != nil {
		return nil, fmt.Errorf("load purpose brief: %w", err)
	}
	if !ok {
		return nil, nil
	}
	frozen := brief
	return &frozen, nil
}

// freezeGuidelines resolves the applicable 지침 once, at enqueue, from the SAME purpose id
// the brief was resolved from — one read, one consistent view. Editing, rescoping or
// deleting a guideline afterwards cannot reach the queued work, including across a
// restart-resume or an explicit retry, because the handlers read only the payload.
func (s *Service) freezeGuidelines(ctx context.Context, post PostInput) ([]string, error) {
	if s.guidelines == nil {
		return nil, nil
	}
	var purposeID *string
	if post.PurposeID != "" {
		id := post.PurposeID
		purposeID = &id
	}
	texts, err := s.guidelines.ForPrompt(ctx, post.UserID, purposeID)
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
// straight from the client, so the per-purpose registration (change 20) is enforced at
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
