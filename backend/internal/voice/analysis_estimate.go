package voice

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/llm"
)

// Missing pricing is a legal unavailable estimate. Production wires its actual
// completion floor; the quote never enqueues work or calls the provider.
func (s *Service) WithAnalysisEstimates(estimates AnalysisEstimates, completionFloor int64) *Service {
	if estimates == nil || completionFloor <= 0 {
		panic("voice: invalid analysis estimate configuration")
	}
	s.analysisEstimates, s.analysisCompletionFloor = estimates, completionFloor
	return s
}

func (s *Service) prepareAnalysis(ctx context.Context, userID, voiceID string, ref llm.ModelRef) (llm.ModelInfo, []Sample, error) {
	if _, err := s.activeVoice(ctx, userID, voiceID); err != nil {
		return llm.ModelInfo{}, nil, err
	}
	info, found := s.models.Resolve(ref)
	if ref.ProviderID == "" || ref.ModelID == "" || !found || info.Disabled || !info.ServesStage(llm.StageNameAnalyze) {
		return llm.ModelInfo{}, nil, ErrAnalyzeModelRequired
	}
	samples, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return llm.ModelInfo{}, nil, fmt.Errorf("list analysis samples: %w", err)
	}
	if !ReadinessOf(samples).Ready() {
		return llm.ModelInfo{}, nil, ErrVoiceNotReady
	}
	return info, samples, nil
}

func (s *Service) EstimateAnalysis(ctx context.Context, userID, voiceID string, model llm.ModelRef) (AnalysisEstimate, error) {
	info, samples, err := s.prepareAnalysis(ctx, userID, voiceID, model)
	if err != nil {
		return AnalysisEstimate{}, err
	}
	if s.analysisEstimates == nil || s.analysisCompletionFloor <= 0 {
		return AnalysisEstimate{}, nil
	}
	counted, oldestFirst, _ := analysisSnapshot(samples)
	credits, available := s.analysisEstimates.CallCredits(ctx, info, int64(promptTokens(analysisRequest(counted, oldestFirst))), s.analysisCompletionFloor)
	if !available || credits < 0 {
		return AnalysisEstimate{}, nil
	}
	return AnalysisEstimate{Credits: credits, Free: credits == 0, Available: true}, nil
}
