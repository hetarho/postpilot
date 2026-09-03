package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

const AnalyzeExperimentPromptVersion = "analyze-v1-ab"

type analyzeExperimentSnapshot struct {
	Corpus         string   `json:"corpus"`
	SourceLanguage Language `json:"source_language,omitempty"`
}

type CandidateUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	CostMicrousd     int64
	CostReported     bool
}

// SnapshotAnalysisInput freezes one voice's whole corpus — the same samples plus finalized
// sources the analyze job would read — so both candidates see exactly what analysis sees.
func (s *Service) SnapshotAnalysisInput(ctx context.Context, userID, voiceID string) ([]byte, error) {
	active, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return nil, err
	}
	samples, _, err := s.store.CorpusSnapshot(ctx, userID, voiceID)
	if err != nil {
		return nil, fmt.Errorf("문체 샘플을 불러오지 못했어요: %w", err)
	}
	var sources []AuthoredSource
	if s.personalization != nil {
		if sources, err = s.personalization.ListAuthoredSources(ctx, userID, voiceID); err != nil {
			return nil, fmt.Errorf("완성 글을 불러오지 못했어요: %w", err)
		}
		sources = authoredSourcesForLanguage(sources, active.SourceLanguage)
	}
	if len(samples) == 0 && len(sources) == 0 {
		return nil, fmt.Errorf("분석할 문체 자료가 없어요")
	}
	return json.Marshal(analyzeExperimentSnapshot{Corpus: personalizationCorpus(samples, sources), SourceLanguage: active.SourceLanguage})
}

func (s *Service) RunAnalyzeCandidate(ctx context.Context, raw []byte, ref llm.ModelRef) (string, CandidateUsage, error) {
	var snapshot analyzeExperimentSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil || strings.TrimSpace(snapshot.Corpus) == "" {
		return "", CandidateUsage{}, fmt.Errorf("saved analyze input is unavailable")
	}
	if !snapshot.SourceLanguage.Valid() {
		// Pre-language analyze snapshots are migration-compatible Korean input; no text
		// inspection or runtime language guessing is involved.
		snapshot.SourceLanguage = LanguageKorean
	}
	response, err := s.models.Complete(ctx, ref, llm.Request{
		System:   analysisPromptForLanguage(snapshot.SourceLanguage),
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(snapshot.Corpus)}}},
		// The candidate path resolves the purpose of the stage it is COMPARING, so an
		// analyze A/B measures both models under the effort each would really run at.
		Stage: llm.StageNameAnalyze,
	})
	usage := CandidateUsage{PromptTokens: int64(response.Usage.PromptTokens), CompletionTokens: int64(response.Usage.CompletionTokens), CostMicrousd: response.Usage.CostMicrousd, CostReported: response.Usage.CostReported}
	if err != nil {
		return "", usage, err
	}
	styleguide := strings.TrimSpace(response.Text)
	if !hasRequiredAnalysisShapeForLanguage(styleguide, snapshot.SourceLanguage) {
		return "", usage, fmt.Errorf("문체 분석 결과에 종결어미 또는 never uses 섹션이 없어요. 다시 시도해 주세요")
	}
	return styleguide, usage, nil
}

// ApplyStyleguideWinner applies the winner only to the voice the experiment froze, and only
// while that voice is still active.
//
// It keeps its NAME and SIGNATURE: the experiment context's ApplyWinner port, the
// confirmStyleguide confirmation and the ApplyWinnerOutput RPC all still call exactly this.
// What changed is where the winning analysis lands. It used to be written into a free-text
// `styleguide` column; that column is gone (change 16), so the winner is applied the way an
// analysis run applies its own result — as a published structured profile version whose
// lexical description IS the winning analysis, with the account's manual overrides and its
// earned rules carried onto it, mirroring analyze_handler.
//
// Publishing IF HEAD rather than unconditionally: an analysis or a rule change that published
// while the operator was confirming this winner is newer evidence, and a stale winner must not
// overwrite it. Losing that race is not an error — the confirmation simply stands down.
func (s *Service) ApplyStyleguideWinner(ctx context.Context, userID, voiceID, styleguide string) error {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return err
	}
	if strings.TrimSpace(styleguide) == "" {
		return nil
	}
	if s.personalization == nil {
		return nil
	}
	head, err := s.store.GetProfile(ctx, userID, voiceID)
	if err != nil {
		return err
	}
	active, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return err
	}
	samples, _, err := s.store.CorpusSnapshot(ctx, userID, voiceID)
	if err != nil {
		return fmt.Errorf("corpus for analyze winner: %w", err)
	}
	var sources []AuthoredSource
	if sources, err = s.personalization.ListAuthoredSources(ctx, userID, voiceID); err != nil {
		return fmt.Errorf("authored sources for analyze winner: %w", err)
	}
	// The measured half is MEASURED, not inherited: cloning the current head would carry its
	// ending distribution and sentence metrics onto a version the winner never described, and a
	// voice with no head at all would publish a v1 with every metric unset. This is exactly what
	// analyze_handler does with its own result — only the description differs.
	corpus := personalizationCorpus(samples, sources)
	profile := MeasuredProfileForLanguage(corpus, active.SourceLanguage, s.now)
	profile.Lexical.Description = VoiceValue{Value: styleguide, Source: SourceAnalyzed}
	profile.SourceCount = len(samples) + len(sources)
	profile.Sources = sources
	profile.Empty = false
	overrides, err := s.personalization.ListManualOverrides(ctx, userID, voiceID)
	if err != nil {
		return fmt.Errorf("manual voice overrides: %w", err)
	}
	for _, override := range overrides {
		if err := applyOverride(&profile, override.Layer, override.Field, override.Value); err != nil {
			return err
		}
	}
	if profile.Rules, err = s.personalization.ListRules(ctx, userID, voiceID); err != nil {
		return fmt.Errorf("voice rules: %w", err)
	}
	// origin "analysis": an analyze-stage winner IS an analysis result, and the version
	// history's origin vocabulary already names that. A separate origin would need a
	// migration to widen the CHECK for a distinction the history does not make.
	_, ok, err := s.personalization.PublishProfileVersionIfHead(ctx, userID, voiceID, profile, "analysis", head.Structured.Version, s.now())
	if err != nil {
		return fmt.Errorf("publish analyze winner profile: %w", err)
	}
	// Losing the head race is NOT success. The caller records the winner as applied on a nil
	// error, so swallowing this would leave the experiment claiming an effect the voice never
	// received. An analysis or a rule published while the operator was confirming is newer
	// evidence; the honest answer is to say the apply did not land so it can be retried.
	if !ok {
		return fmt.Errorf("문체 프로필이 그 사이에 갱신되었어요. 다시 적용해 주세요")
	}
	return nil
}
