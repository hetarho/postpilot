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
		head, err := s.store.GetProfile(ctx, found.UserID)
		if err != nil {
			return fmt.Errorf("현재 문체 프로필을 불러오지 못했어요: %w", err)
		}
		samples, corpusVersion, err := s.store.CorpusSnapshot(ctx, found.UserID)
		if err != nil {
			return fmt.Errorf("문체 샘플을 불러오지 못했어요: %w", err)
		}
		var sources []AuthoredSource
		if s.personalization != nil {
			sources, err = s.personalization.ListAuthoredSources(ctx, found.UserID)
			if err != nil {
				return fmt.Errorf("완성 글을 불러오지 못했어요: %w", err)
			}
		}
		if len(samples) == 0 && len(sources) == 0 {
			if attempted {
				progress("analyze", 1, 1)
				return nil
			}
			return fmt.Errorf("분석할 문체 자료가 없어요")
		}
		corpus := personalizationCorpus(samples, sources)
		attempted = true
		progress("analyze", 0, 1)
		response, err := s.models.Complete(ctx, ref, llm.Request{
			System:   analysisPrompt,
			Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.TextPart(corpus)}}},
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
			measured := MeasuredProfile(corpus, s.now)
			measured.Lexical.Description = VoiceValue{Value: styleguide, Source: SourceAnalyzed}
			measured.SourceCount = len(samples) + len(sources)
			measured.Sources = sources
			measured.Empty = false
			if s.personalization == nil {
				progress("analyze", 1, 1)
				return nil
			}
			overrides, overrideErr := s.personalization.ListManualOverrides(ctx, found.UserID)
			if overrideErr != nil {
				return fmt.Errorf("manual voice overrides: %w", overrideErr)
			}
			for _, override := range overrides {
				if overrideErr = applyOverride(&measured, override.Layer, override.Field, override.Value); overrideErr != nil {
					return overrideErr
				}
			}
			measured.Rules, overrideErr = s.personalization.ListRules(ctx, found.UserID)
			if overrideErr != nil {
				return fmt.Errorf("voice rules: %w", overrideErr)
			}
			if _, published, versionErr := s.personalization.PublishProfileVersionIfHead(ctx, found.UserID, measured, "analysis", head.Structured.Version, s.now()); versionErr != nil {
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
		// analyze the newest full snapshot instead of publishing a stale styleguide.
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

func personalizationCorpus(samples []Sample, sources []AuthoredSource) string {
	var corpus strings.Builder
	if len(samples) > 0 {
		corpus.WriteString(AssembleCorpus(samples))
	}
	for _, source := range sources {
		if corpus.Len() > 0 {
			corpus.WriteString("\n\n")
		}
		corpus.WriteString("--- finalized: ")
		corpus.WriteString(source.Title)
		corpus.WriteString(" ---\n")
		corpus.WriteString(source.Body)
	}
	return corpus.String()
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
