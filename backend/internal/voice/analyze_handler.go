package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

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
	attempted := false
	for {
		head, err := s.profiles.GetProfile(ctx, found.UserID, found.VoiceID)
		if err != nil {
			return fmt.Errorf("현재 문체 프로필을 불러오지 못했어요: %w", err)
		}
		samples, corpusVersion, err := s.samples.CorpusSnapshot(ctx, found.UserID, found.VoiceID)
		if err != nil {
			return fmt.Errorf("문체 샘플을 불러오지 못했어요: %w", err)
		}
		if len(samples) == 0 {
			if attempted {
				progress("analyze", 1, 1)
				return nil
			}
			return fmt.Errorf("분석할 문체 자료가 없어요")
		}
		corpus := AssembleCorpus(samples)
		attempted = true
		progress("analyze", 0, 1)
		// The typed analysis, with its schema: the voice gets its axes and structure habits
		// from this call (VOICE-27).
		qualitative, err := s.completeAnalysis(ctx, ref, corpus)
		if err != nil {
			return err
		}
		// The guard, and only the guard: it used to be a write of `styleguide` that happened
		// to be conditional. False means a sample changed while the provider was working, so
		// this analysis describes a corpus the voice has already moved past (VOICE-22).
		stored, err := s.profiles.ClaimCorpusVersion(ctx, found.UserID, found.VoiceID, corpusVersion, s.now())
		if err != nil {
			return fmt.Errorf("문체 분석 결과를 저장하지 못했어요: %w", err)
		}
		if stored {
			measured := MeasuredProfile(corpus, s.now)
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
			overrides, overrideErr := s.overrides.ListManualOverrides(ctx, found.UserID, found.VoiceID)
			if overrideErr != nil {
				return fmt.Errorf("manual voice overrides: %w", overrideErr)
			}
			for _, override := range overrides {
				if overrideErr = applyOverride(&measured, override.Layer, override.Field, override.Value); overrideErr != nil {
					return overrideErr
				}
			}
			if _, published, versionErr := s.versions.PublishProfileVersionIfHead(ctx, found.UserID, found.VoiceID, measured, "analysis", head.Structured.Version, s.now()); versionErr != nil {
				return fmt.Errorf("publish typed voice profile: %w", versionErr)
			} else if !published {
				if err := ctx.Err(); err != nil {
					return err
				}
				continue
			}
			progress("analyze", 1, 1)
			return nil
		}
		// A sample changed while the provider was running. Keep the same durable job and
		// analyze the newest full snapshot instead of publishing a stale analysis.
		if err := ctx.Err(); err != nil {
			return err
		}
	}
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
