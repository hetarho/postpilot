package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

// Analyze is the `analyze_voice` job (VOICE-22, VOICE-23): it reads the snapshot of the voice's
// 학습 글 its start froze — those that still exist, nothing added since — counts the fingerprint,
// makes one call for the AI part and publishes the result as the current analysis, the one it
// replaces becoming the previous. A failed call publishes nothing, and nothing repeats the call
// without the owner's press.
func (s *Service) Analyze(ctx context.Context, found AnalysisJob, progress Progress) error {
	ref, err := parseModelRef(found.WriteModel)
	if err != nil {
		return err
	}
	// The job froze its voice at enqueue; recheck before the provider call so a voice
	// deleted while the job waited is never analysed.
	if _, err := s.activeVoice(ctx, found.UserID, found.VoiceID); err != nil {
		return voiceUnavailableError(err)
	}
	listed, err := s.samples.ListSampleBodies(ctx, found.UserID, found.VoiceID)
	if err != nil {
		return fmt.Errorf("학습 글을 불러오지 못했어요: %w", err)
	}
	newestFirst := frozenSamples(listed, found.MaterialIDs)
	if len(newestFirst) == 0 {
		return fmt.Errorf("분석할 학습 글이 없어요")
	}
	counted, oldestFirst, ids := analysisSnapshot(newestFirst)
	progress("analyze", 0, 1)
	ai, err := s.completeAnalysis(ctx, ref, counted, oldestFirst)
	if err != nil {
		return err
	}
	if err := s.analyses.PublishAnalysis(ctx, found.UserID, found.VoiceID, Analysis{
		Counted: counted, AI: ai, MaterialIDs: ids, AnalyzeModel: ref.String(), CreatedAt: s.now(),
	}); err != nil {
		return fmt.Errorf("말투 분석 결과를 저장하지 못했어요: %w", err)
	}
	progress("analyze", 1, 1)
	return nil
}

// frozenSamples keeps, in listed order, the 학습 글 of the snapshot that still exist.
func frozenSamples(listed []Sample, materialIDs []string) []Sample {
	frozen := make(map[string]bool, len(materialIDs))
	for _, id := range materialIDs {
		frozen[id] = true
	}
	kept := make([]Sample, 0, len(materialIDs))
	for _, sample := range listed {
		if frozen[sample.ID] {
			kept = append(kept, sample)
		}
	}
	return kept
}

// materialsOf is the 학습 글 as the fingerprint reads them.
func materialsOf(samples []Sample) []Material {
	out := make([]Material, 0, len(samples))
	for _, sample := range samples {
		material := Material{ID: sample.ID, Kind: sample.Kind, CreatedAt: sample.CreatedAt, Text: sample.Body}
		if prompt, ok := PromptByKey(sample.PromptKey); ok {
			material.Part = prompt.Part
		}
		out = append(out, material)
	}
	return out
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

func parseModelRef(value string) (llm.ModelRef, error) {
	providerID, modelID, ok := strings.Cut(value, "/")
	if !ok || providerID == "" || modelID == "" {
		return llm.ModelRef{}, fmt.Errorf("분석 모델 정보가 올바르지 않아요")
	}
	return llm.ModelRef{ProviderID: providerID, ModelID: modelID}, nil
}
