package voice

import (
	"context"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

func (s *Service) Analyze(ctx context.Context, found AnalysisJob, progress Progress) error {
	ref, err := parseModelRef(found.WriteModel)
	if err != nil {
		return err
	}
	attempted := false
	for {
		samples, corpusVersion, err := s.store.CorpusSnapshot(ctx, found.UserID)
		if err != nil {
			return fmt.Errorf("문체 샘플을 불러오지 못했어요: %w", err)
		}
		if len(samples) == 0 {
			if attempted {
				progress("analyze", 1, 1)
				return nil
			}
			return fmt.Errorf("분석할 문체 샘플이 없어요")
		}
		attempted = true
		progress("analyze", 0, 1)
		response, err := s.models.Complete(ctx, ref, llm.Request{
			System:   analysisPrompt,
			Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(AssembleCorpus(samples))}}},
		})
		if err != nil {
			return err
		}
		styleguide := strings.TrimSpace(response.Text)
		if !hasRequiredAnalysisShape(styleguide) {
			return fmt.Errorf("문체 분석 결과에 종결어미 또는 never uses 섹션이 없어요. 다시 시도해 주세요")
		}
		stored, err := s.store.SetStyleguideIfCorpusVersion(ctx, found.UserID, styleguide, corpusVersion, s.now())
		if err != nil {
			return fmt.Errorf("문체 분석 결과를 저장하지 못했어요: %w", err)
		}
		if stored {
			progress("analyze", 1, 1)
			return nil
		}
		// A sample changed while the provider was running. Keep the same durable job and
		// analyze the newest full snapshot instead of publishing a stale styleguide.
		if err := ctx.Err(); err != nil {
			return err
		}
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
