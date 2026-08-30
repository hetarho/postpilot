package generation

import (
	"context"
	"fmt"
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
// prompt simply carries no brief — the same output this code produced before purposes
// existed — so a partially wired process degrades to the old behavior rather than failing.
func (s *Service) SetPurposeBriefs(briefs PurposeBriefs) { s.purposes = briefs }

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
	voiceID, err := activeVoice(post)
	if err != nil {
		return "", err
	}
	request.VoiceID = voiceID
	if err := s.refusePendingExperiment(ctx, request.UserID, request.PostSlug); err != nil {
		return "", err
	}
	write, ok := parseModelRef(request.WriteModel)
	if !ok || !modelEnabled(s.models, write) {
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
	payload, err := encodeRevisionPayload(request.Instruction, request.SaveAsRule, brief)
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
	voiceID, err := activeVoice(post)
	if err != nil {
		return "", err
	}
	request.VoiceID = voiceID
	if err := s.refusePendingExperiment(ctx, request.UserID, request.PostSlug); err != nil {
		return "", err
	}
	write, ok := parseModelRef(request.WriteModel)
	if !ok || !modelEnabled(s.models, write) {
		return "", ErrWriteModelRequired
	}
	if len(post.Images) == 0 {
		request.ObserveModel = ""
	} else {
		observe, valid := parseModelRef(request.ObserveModel)
		info, found := s.models.Resolve(observe)
		if !valid || !found || info.Disabled || !info.Vision {
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
	id, err := s.jobs.EnqueueGeneration(ctx, request)
	if err != nil {
		return "", fmt.Errorf("enqueue generation: %w", err)
	}
	return id, nil
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

func modelEnabled(models LLM, ref llm.ModelRef) bool {
	info, ok := models.Resolve(ref)
	return ok && !info.Disabled
}

func parseModelRef(value string) (llm.ModelRef, bool) {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return llm.ModelRef{}, false
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}, true
}
