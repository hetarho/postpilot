package voice_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/voice"
)

func TestAuthoringSeedKeepsLatestDraftDescriptionAndExampleAuthoritative(t *testing.T) {
	for _, origin := range []voice.Origin{voice.OriginPersonal, voice.OriginSynthetic} {
		t.Run(string(origin), func(t *testing.T) {
			h := newVoiceHarness(t)
			ctx := context.Background()
			id := h.voice("alice")
			prefix := strings.Repeat("담", voice.CandidateDescriptionMaxChars)
			suffix := " 추가로 길고 유용한 원본 문체 인상을 보존해요."
			analysis := voice.Analysis{Origin: origin, AI: voice.AIPart{Impression: prefix, Tics: []voice.Tic{{Phrase: "정말", When: "감탄할 때"}}}, CreatedAt: time.Now()}
			if origin == voice.OriginPersonal {
				analysis.AI.Impression += suffix
			} else {
				analysis.SyntheticSample = strings.Repeat("가상 상황에서 조용히 차를 마시고 산책했어요. ", 10)
			}
			if err := h.store.PublishAnalysis(ctx, "alice", id, analysis); err != nil {
				t.Fatal(err)
			}
			draft, revision, source, err := voice.NewAuthoring(h.svc, h.store).Seed(ctx, "alice", id)
			if err != nil || revision == "" || draft.Description != prefix || utf8.RuneCountInString(draft.Description) != voice.CandidateDescriptionMaxChars || strings.Contains(source, prefix) || !strings.Contains(source, "감탄할 때") {
				t.Fatalf("source repeated current description or lost other style context: %+v, %q, %v", draft, source, err)
			}
			if origin == voice.OriginPersonal {
				if draft.Sample != "" || !strings.Contains(source, suffix) || !strings.Contains(source, "최신 초안의 description을 우선") || !strings.Contains(source, "원본 분석 인상의 추가 설명") {
					t.Fatalf("personal source lost useful long description or fabricated an example: %+v, %s", draft, source)
				}
			} else if draft.Sample != analysis.SyntheticSample || strings.Contains(source, analysis.SyntheticSample) {
				t.Fatalf("synthetic example was repeated outside latest draft: %+v, %s", draft, source)
			}
			stored, err := h.store.CurrentAnalysis(ctx, "alice", id)
			if err != nil || stored.AI.Impression != analysis.AI.Impression || h.models.completeCalls != 0 {
				t.Fatal("source composition changed accepted analysis or made a provider call", err)
			}
		})
	}
}
