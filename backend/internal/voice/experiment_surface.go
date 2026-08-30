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
	Corpus string `json:"corpus"`
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
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
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
	}
	if len(samples) == 0 && len(sources) == 0 {
		return nil, fmt.Errorf("분석할 문체 자료가 없어요")
	}
	return json.Marshal(analyzeExperimentSnapshot{Corpus: personalizationCorpus(samples, sources)})
}

func (s *Service) RunAnalyzeCandidate(ctx context.Context, raw []byte, ref llm.ModelRef) (string, CandidateUsage, error) {
	var snapshot analyzeExperimentSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil || strings.TrimSpace(snapshot.Corpus) == "" {
		return "", CandidateUsage{}, fmt.Errorf("saved analyze input is unavailable")
	}
	response, err := s.models.Complete(ctx, ref, llm.Request{
		System:   analysisPrompt,
		Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(snapshot.Corpus)}}},
	})
	usage := CandidateUsage{PromptTokens: int64(response.Usage.PromptTokens), CompletionTokens: int64(response.Usage.CompletionTokens), CostMicrousd: response.Usage.CostMicrousd, CostReported: response.Usage.CostReported}
	if err != nil {
		return "", usage, err
	}
	styleguide := strings.TrimSpace(response.Text)
	if !hasRequiredAnalysisShape(styleguide) {
		return "", usage, fmt.Errorf("문체 분석 결과에 종결어미 또는 never uses 섹션이 없어요. 다시 시도해 주세요")
	}
	return styleguide, usage, nil
}

// ApplyStyleguideWinner writes the winner only into the voice the experiment froze, and
// only while that voice is still active.
func (s *Service) ApplyStyleguideWinner(ctx context.Context, userID, voiceID, styleguide string) error {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return err
	}
	profile, err := s.store.GetProfile(ctx, userID, voiceID)
	if err != nil {
		return err
	}
	if profile.Styleguide == styleguide {
		return nil
	}
	_, err = s.UpdateStyleguide(ctx, userID, voiceID, styleguide)
	return err
}
