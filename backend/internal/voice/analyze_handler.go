package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// Analyze is the `analyze_voice` job: it reads one snapshot of the voice's 학습 글, measures it
// and asks one call for the rest, and publishes what it read even when a 학습 글 changed
// meanwhile — nothing repeats the call without the owner's press (VOICE-22, VOICE-23).
func (s *Service) Analyze(ctx context.Context, found AnalysisJob, progress Progress) error {
	ref, err := parseModelRef(found.WriteModel)
	if err != nil {
		return err
	}
	// The job froze its voice at enqueue; recheck before the provider call so a voice
	// deleted while the job waited never receives a new styleguide.
	if _, err := s.activeVoice(ctx, found.UserID, found.VoiceID); err != nil {
		return voiceUnavailableError(err)
	}
	newestFirst, err := s.samples.ListSampleBodies(ctx, found.UserID, found.VoiceID)
	if err != nil {
		return fmt.Errorf("학습 글을 불러오지 못했어요: %w", err)
	}
	if len(newestFirst) == 0 {
		return fmt.Errorf("분석할 학습 글이 없어요")
	}
	samples := make([]Sample, len(newestFirst))
	for i, sample := range newestFirst {
		samples[len(newestFirst)-1-i] = sample
	}
	progress("analyze", 0, 1)
	// The typed analysis, with its schema: the voice gets its axes and structure habits
	// from this call (VOICE-27).
	qualitative, err := s.completeAnalysis(ctx, ref, AssembleCorpus(samples))
	if err != nil {
		return err
	}
	measured := MeasuredProfile(proseCorpus(samples), s.now)
	mergeQualitativeProfile(&measured, qualitative, analyzedValue)
	if err := validateAxes(measured.Axes); err != nil {
		return err
	}
	measured.SourceCount = len(samples)
	measured.Empty = false
	if s.personalization == nil {
		progress("analyze", 1, 1)
		return nil
	}
	overrides, err := s.overrides.ListManualOverrides(ctx, found.UserID, found.VoiceID)
	if err != nil {
		return fmt.Errorf("manual voice overrides: %w", err)
	}
	for _, override := range overrides {
		if err := applyOverride(&measured, override.Layer, override.Field, override.Value); err != nil {
			return err
		}
	}
	if _, err := s.versions.PublishProfileVersion(ctx, found.UserID, found.VoiceID, measured, "analysis", 0, s.now()); err != nil {
		return fmt.Errorf("publish typed voice profile: %w", err)
	}
	progress("analyze", 1, 1)
	return nil
}

// voiceUnavailableError turns a directory refusal into the user-facing reason a job row
// shows, keeping the sentinel so tests and callers can still identify it.
func voiceUnavailableError(err error) error {
	switch {
	case errors.Is(err, ErrVoiceDeleted):
		return fmt.Errorf("삭제된 말투에는 AI 작업을 진행할 수 없어요. 말투를 복원하거나 글의 말투를 바꿔 주세요: %w", err)
	case errors.Is(err, ErrVoiceNotFound), errors.Is(err, ErrVoiceRequired):
		return fmt.Errorf("작업이 가리키는 말투를 찾을 수 없어요: %w", err)
	default:
		return err
	}
}

func hasRequiredAnalysisShape(styleguide string) bool {
	lines := strings.Split(strings.TrimSpace(styleguide), "\n")
	if len(lines) == 0 || !strings.Contains(lines[0], "종결어미") {
		return false
	}
	lower := strings.ToLower(styleguide)
	return strings.Contains(lower, "never uses") || strings.Contains(styleguide, "사용하지 않는") || strings.Contains(styleguide, "쓰지 않는")
}

func parseModelRef(value string) (llm.ModelRef, error) {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return llm.ModelRef{}, fmt.Errorf("분석 모델 정보가 올바르지 않아요")
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}, nil
}
