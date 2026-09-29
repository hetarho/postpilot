package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// CheckJobKind is 검증's durable job (VOICE-43).
const CheckJobKind = "check_voice"

// VOICE_CHECK_SENTENCES: the piece 검증 asks for is 10~15 sentences with an opening and a
// closing — a requested range, not a validation (two to five sentences cannot show a ratio).
const (
	CheckSentencesMin = 10
	CheckSentencesMax = 15
)

var (
	// ErrCheckPromptUnanswered is 검증 on a prompt the voice holds no answer to.
	ErrCheckPromptUnanswered = errors.New("the prompt has no answer to check against")
	// ErrCheckPhotoUnsupported is a photo prompt on a write model that does not read images.
	ErrCheckPhotoUnsupported = errors.New("the write model cannot read the prompt's photo")
	// ErrCheckNotFound is a check id this account does not own.
	ErrCheckNotFound = errors.New("voice check not found")
	// ErrWriteModelRequired is 검증 without an enabled write-stage model.
	ErrWriteModelRequired = errors.New("an enabled write model is required")
)

// CheckStatus is where a check is: queued and running while its job holds it, then done with a
// piece or failed with its failure.
type CheckStatus string

const (
	CheckQueued  CheckStatus = "queued"
	CheckRunning CheckStatus = "running"
	CheckDone    CheckStatus = "done"
	CheckFailed  CheckStatus = "failed"
)

// Check is one 검증 as stored: what it froze at start and what it produced.
type Check struct {
	ID        string
	UserID    string
	VoiceID   string
	PromptKey string
	// MaterialID is the answer that was withheld from the projection and is shown beside the
	// piece.
	MaterialID string
	// AnalysisCreatedAt is the analysis the projection was taken from.
	AnalysisCreatedAt time.Time
	Projection        string
	WriteModel        string
	Status            CheckStatus
	Piece             string
	Failure           *Failure
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CheckView is a check as the 검증 tab reads it (VOICE-43): its prompt, the answer's current
// text (AnswerDeleted once that 학습 글 is gone), the piece measured against the current
// analysis, and whether it was written from an analysis the voice has since replaced.
type CheckView struct {
	Check
	Prompt        Prompt
	Answer        string
	AnswerDeleted bool
	Comparison    []ItemComparison
	Stale         bool
}

// CheckJob is the frozen input the handler reads from the job row.
type CheckJob struct {
	UserID, VoiceID, CheckID, WriteModel string
}

// CheckJobRequest is a 검증 enqueue.
type CheckJobRequest struct {
	UserID, VoiceID, CheckID, WriteModel string
}

// CheckStore holds the voice's checks.
type CheckStore interface {
	InsertCheck(ctx context.Context, check Check) error
	// DeleteCheck removes a check whose enqueue failed (VOICE-44).
	DeleteCheck(ctx context.Context, userID, checkID string) error
	// GetCheck is ErrCheckNotFound for an id this account does not own.
	GetCheck(ctx context.Context, userID, checkID string) (Check, error)
	ListChecks(ctx context.Context, userID, voiceID string) ([]Check, error)
	// MarkCheckRunning, FinishCheck and FailCheck report false for a check no longer waiting
	// for them.
	MarkCheckRunning(ctx context.Context, userID, checkID string, now time.Time) (bool, error)
	FinishCheck(ctx context.Context, userID, checkID, piece string, now time.Time) (bool, error)
	FailCheck(ctx context.Context, userID, checkID string, failure Failure, now time.Time) (bool, error)
}

// StartVoiceCheck is 검증하기 (VOICE-43): on a made, active voice and an answered prompt, it
// freezes the projection with that answer withheld, the prompt and the analysis it read, and
// enqueues one check_voice job holding one write call. A photo prompt needs a write model that
// reads images.
func (s *Service) StartVoiceCheck(ctx context.Context, userID, voiceID, promptKey string, model llm.ModelRef) (CheckView, string, error) {
	input, err := s.prepareCheck(ctx, userID, voiceID, promptKey)
	if err != nil {
		return CheckView{}, "", err
	}
	analysis, prompt, answer := input.analysis, input.prompt, input.answer
	info, found := s.models.Resolve(model)
	if model.ProviderID == "" || model.ModelID == "" || !found || info.Disabled || !info.ServesStage(llm.StageNameWrite) {
		return CheckView{}, "", ErrWriteModelRequired
	}
	// The registry refuses an image to a model without vision; the start refuses first, so a
	// check that could only fail is never paid for.
	if prompt.Photo && !info.Vision {
		return CheckView{}, "", ErrCheckPhotoUnsupported
	}
	now := s.now()
	check := Check{
		ID: s.newID(), UserID: userID, VoiceID: voiceID, PromptKey: promptKey, MaterialID: answer.ID,
		AnalysisCreatedAt: analysis.CreatedAt, Projection: input.projection, WriteModel: model.String(),
		Status: CheckQueued, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.checks.InsertCheck(ctx, check); err != nil {
		return CheckView{}, "", fmt.Errorf("insert check: %w", err)
	}
	id, err := s.jobs.EnqueueCheck(ctx, CheckJobRequest{UserID: userID, VoiceID: voiceID, CheckID: check.ID, WriteModel: check.WriteModel})
	if err != nil {
		// A failed enqueue leaves no result and blocks nothing (VOICE-44).
		if cleanup := s.checks.DeleteCheck(context.WithoutCancel(ctx), userID, check.ID); cleanup != nil {
			err = errors.Join(err, fmt.Errorf("delete unqueued check: %w", cleanup))
		}
		var active *JobAlreadyInProgressError
		if errors.As(err, &active) {
			return CheckView{}, "", ErrVoiceBusy
		}
		return CheckView{}, "", err
	}
	return CheckView{Check: check, Prompt: prompt, Answer: answer.Body}, id, nil
}

// RetryVoiceCheck starts a new check on the same prompt against the voice's current analysis
// (VOICE-44); the old result stays listed.
func (s *Service) RetryVoiceCheck(ctx context.Context, userID, checkID string, model llm.ModelRef) (CheckView, string, error) {
	found, err := s.checks.GetCheck(ctx, userID, checkID)
	if err != nil {
		return CheckView{}, "", err
	}
	return s.StartVoiceCheck(ctx, userID, found.VoiceID, found.PromptKey, model)
}

// ListVoiceChecks is the 검증 tab (VOICE-43): the voice's checks newest first, readable on a
// deleted voice too (VOICE-15), each piece measured against the current analysis, and the
// voice's queued or running check job so a reload resumes polling (VOICE-31).
func (s *Service) ListVoiceChecks(ctx context.Context, userID, voiceID string) ([]CheckView, string, error) {
	if _, err := s.ownedVoice(ctx, userID, voiceID); err != nil {
		return nil, "", err
	}
	checks, err := s.checks.ListChecks(ctx, userID, voiceID)
	if err != nil {
		return nil, "", fmt.Errorf("list checks: %w", err)
	}
	analysis, err := s.analyses.CurrentAnalysis(ctx, userID, voiceID)
	if err != nil {
		return nil, "", fmt.Errorf("current analysis: %w", err)
	}
	active, err := s.jobs.ActiveForVoiceKind(ctx, voiceID, CheckJobKind)
	if err != nil {
		return nil, "", fmt.Errorf("active check: %w", err)
	}
	activeJobID := ""
	if active != nil {
		activeJobID = active.ID
	}
	answers := map[string]*Sample{}
	out := make([]CheckView, 0, len(checks))
	for i, check := range checks {
		// At most one check job runs per voice, and a check is inserted just before its job, so
		// only the newest check can be the live one; an older one still waiting was interrupted.
		live := activeJobID != "" && i == 0
		if (check.Status == CheckQueued || check.Status == CheckRunning) && !live {
			check.Status = CheckFailed
			check.Failure = &Failure{Reason: FailureReasonJobInterrupted, Params: map[string]string{}}
		}
		view := CheckView{Check: check}
		view.Prompt, _ = PromptByKey(check.PromptKey)
		answer, seen := answers[check.MaterialID]
		if !seen {
			if answer, err = s.samples.GetSampleBody(ctx, userID, voiceID, check.MaterialID); err != nil {
				return nil, "", fmt.Errorf("get answer: %w", err)
			}
			answers[check.MaterialID] = answer
		}
		if answer == nil {
			view.AnswerDeleted = true
		} else {
			view.Answer = answer.Body
		}
		if analysis != nil {
			view.Stale = !check.AnalysisCreatedAt.Equal(analysis.CreatedAt)
			if check.Status == CheckDone {
				view.Comparison = Compare(analysis.Counted, MeasureText(check.Piece))
			}
		}
		out = append(out, view)
	}
	return out, activeJobID, nil
}

// checkInput is what 검증 and 말투 반영 비교 freeze from a voice before any call: its current
// analysis, the prompt, the answer, and the projection with that answer withheld (VOICE-43).
type checkInput struct {
	analysis   *Analysis
	prompt     Prompt
	answer     *Sample
	projection string
}

// prepareCheck refuses a tombstone, a voice not made, an unknown prompt and an unanswered one,
// then freezes the projection with the answer withheld.
func (s *Service) prepareCheck(ctx context.Context, userID, voiceID, promptKey string) (checkInput, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return checkInput{}, err
	}
	analysis, err := s.analyses.CurrentAnalysis(ctx, userID, voiceID)
	if err != nil {
		return checkInput{}, fmt.Errorf("current analysis: %w", err)
	}
	if analysis == nil {
		return checkInput{}, ErrVoiceNotMade
	}
	prompt, ok := PromptByKey(promptKey)
	if !ok {
		return checkInput{}, ErrPromptNotFound
	}
	answer, err := s.answerTo(ctx, userID, voiceID, promptKey)
	if err != nil {
		return checkInput{}, err
	}
	if answer == nil {
		return checkInput{}, ErrCheckPromptUnanswered
	}
	projection, err := s.PromptProfileForTopic(ctx, userID, voiceID, prompt.Text, LanguageKorean, answer.ID)
	if err != nil {
		return checkInput{}, err
	}
	return checkInput{analysis: analysis, prompt: prompt, answer: answer, projection: frozenProjection(projection)}, nil
}

// answerTo is the voice's answer to one prompt, or nil.
func (s *Service) answerTo(ctx context.Context, userID, voiceID, promptKey string) (*Sample, error) {
	samples, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return nil, fmt.Errorf("list samples: %w", err)
	}
	for i := range samples {
		if samples[i].Kind == SampleKindAnswer && samples[i].PromptKey == promptKey {
			return &samples[i], nil
		}
	}
	return nil, nil
}

// frozenProjection is the projection as the check's call reads it: the [말투] section, then the
// excerpts the way the write prompt carries them (VOICE-46).
func frozenProjection(profile PromptProfile) string {
	var out strings.Builder
	out.WriteString(profile.Text)
	if !profile.Portable && len(profile.Excerpts) > 0 {
		out.WriteString("\n\n[글 예시 발췌]")
		for i, excerpt := range profile.Excerpts {
			fmt.Fprintf(&out, "\n%d. %s", i+1, excerpt)
		}
		out.WriteString("\n예시의 고유 사실, 주제, 문구를 복사하지 말고 문체 특징만 참고하세요.")
	}
	return out.String()
}
