package voice

import (
	"context"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// SeedJob is the frozen input of one described-voice seeding run. Description is the user's
// request, carried on the job row rather than in a table: it is an instruction consumed once,
// not evidence the profile keeps.
type SeedJob struct {
	UserID      string
	VoiceID     string
	Description string
	WriteModel  string
}

// Seed writes a brand-new voice's FIRST profile from a description of the wanted register.
//
// It is deliberately not Analyze with a different corpus. Analysis measures real prose and
// republishes on every corpus change; a seed has no prose to measure, runs exactly once, and
// must lose every race against real evidence. Hence the three differences below: no
// MeasuredProfileForLanguage, an expectedHead of 0, and no retry loop.
func (s *Service) Seed(ctx context.Context, found SeedJob, progress Progress) error {
	ref, err := parseModelRef(found.WriteModel)
	if err != nil {
		return err
	}
	description := strings.TrimSpace(found.Description)
	if description == "" {
		return fmt.Errorf("말투 설명이 비어 있어요")
	}
	// The job froze its voice at enqueue; recheck before the provider call so a voice deleted
	// while the job waited never receives a styleguide.
	active, err := s.activeVoice(ctx, found.UserID, found.VoiceID)
	if err != nil {
		return voiceUnavailableError(err)
	}
	head, err := s.store.GetProfile(ctx, found.UserID, found.VoiceID)
	if err != nil {
		return fmt.Errorf("현재 문체 프로필을 불러오지 못했어요: %w", err)
	}
	// A seed is a starting point, never a correction. If real evidence has already published
	// while this job waited, the run is finished — succeeding, so the user sees a completed
	// job rather than a failure they cannot act on.
	if head.Structured.Version > 0 {
		progress("seed", 1, 1)
		return nil
	}
	_, corpusVersion, err := s.store.CorpusSnapshot(ctx, found.UserID, found.VoiceID)
	if err != nil {
		return fmt.Errorf("문체 샘플을 불러오지 못했어요: %w", err)
	}
	progress("seed", 0, 1)
	response, err := s.models.Complete(ctx, ref, llm.Request{
		System:   seedPromptForLanguage(active.SourceLanguage),
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(description)}}},
		Stage:    llm.StageNameAnalyze,
	})
	if err != nil {
		return err
	}
	styleguide := strings.TrimSpace(response.Text)
	if !hasRequiredAnalysisShapeForLanguage(styleguide, active.SourceLanguage) {
		return fmt.Errorf("문체 분석 결과에 종결어미 또는 never uses 섹션이 없어요. 다시 시도해 주세요")
	}
	// The same optimistic guard the analysis uses: a sample added while the provider ran bumps
	// the corpus version, and its own analysis is the authority from then on.
	stored, err := s.store.ClaimCorpusVersion(ctx, found.UserID, found.VoiceID, corpusVersion, s.now())
	if err != nil {
		return fmt.Errorf("문체 분석 결과를 저장하지 못했어요: %w", err)
	}
	if !stored {
		progress("seed", 1, 1)
		return nil
	}
	if s.personalization == nil {
		progress("seed", 1, 1)
		return nil
	}
	// Everything a corpus would have measured stays unset. A description states a wanted
	// register; measuring the request's own sentences would report the user's prompt style as
	// though it were their writing.
	seeded := StructuredProfile{
		Empty:   false,
		Lexical: LexicalProfile{Description: VoiceValue{Value: styleguide, Source: SourceAnalyzed}},
	}
	if _, published, err := s.personalization.PublishProfileVersionIfHead(ctx, found.UserID, found.VoiceID, seeded, "seed", 0, s.now()); err != nil {
		return fmt.Errorf("publish seeded voice profile: %w", err)
	} else if !published {
		// Real evidence won the race between the corpus claim and this publish. Unlike
		// Analyze there is nothing to recompute, so the seed simply stands down.
		progress("seed", 1, 1)
		return nil
	}
	progress("seed", 1, 1)
	return nil
}
