package voice

import (
	"strings"
	"testing"
)

func TestStarterQuestionsAreTenCompleteOrdinarySituations(t *testing.T) {
	count := 0
	parts := map[PromptPart]bool{}
	scenes := map[string]bool{}
	for _, prompt := range Prompts() {
		if !prompt.Starter {
			continue
		}
		count++
		if prompt.Photo || prompt.Scene == "" || prompt.Hint == "" || !strings.Contains(prompt.Text, "1~3문장") || scenes[prompt.Scene] {
			t.Fatalf("invalid starter: %+v", prompt)
		}
		scenes[prompt.Scene] = true
		parts[prompt.Part] = true
	}
	if count != InitialQuestionCount || len(parts) != 3 {
		t.Fatalf("starter count=%d parts=%v", count, parts)
	}
	for _, old := range legacyPrompts {
		found, ok := PromptByKey(old.Key)
		if !ok || found.Photo != old.Photo || found.Part != old.Part {
			t.Fatalf("legacy semantics changed: %s", old.Key)
		}
	}
}

func TestTenValidDistinctAnswersBecomeReadyWithShortProse(t *testing.T) {
	var samples []Sample
	for _, prompt := range Prompts() {
		if prompt.Starter {
			samples = append(samples, Sample{Kind: SampleKindAnswer, PromptKey: prompt.Key, Body: "잠깐 쉬고 싶어요."})
		}
	}
	ready := ReadinessOf(samples)
	if !ready.Ready() || ready.AnsweredQuestions != 10 || ready.RequiredQuestions != 10 || ready.Sentences != 10 {
		t.Fatalf("readiness=%+v", ready)
	}
	if got := ReadinessOf(samples[:9]); got.Ready() || got.AnsweredQuestions != 9 {
		t.Fatalf("nine answers=%+v", got)
	}
	duplicate := append([]Sample{}, samples[:9]...)
	duplicate = append(duplicate, samples[0])
	if got := ReadinessOf(duplicate); got.Ready() || got.AnsweredQuestions != 9 {
		t.Fatalf("duplicate answer advances=%+v", got)
	}
	missingClosing := append([]Sample{}, samples[:8]...)
	missingClosing = append(missingClosing, Sample{Kind: SampleKindAnswer, PromptKey: "opening_greeting", Body: "반가워요."}, Sample{Kind: SampleKindAnswer, PromptKey: "opening_topic", Body: "오늘은 쉬고 싶어요."})
	if got := ReadinessOf(missingClosing); got.Percent != 99 || len(got.MissingParts) != 1 || got.MissingParts[0] != PartClosing {
		t.Fatalf("missing part=%+v", got)
	}
	for _, body := range []string{"#좋아요 #행복", "가격: 1000원", "📍 서울", "ㅎㅎ ㅠㅠ", "just text", "😀"} {
		invalid := append([]Sample{}, samples[:9]...)
		invalid = append(invalid, Sample{Kind: SampleKindAnswer, PromptKey: samples[9].PromptKey, Body: body})
		if got := ReadinessOf(invalid); got.Ready() || got.AnsweredQuestions != 9 {
			t.Fatalf("nonprose %q advances=%+v", body, got)
		}
	}
	fingerprint := FingerprintOf(materialsOf(samples))
	if !fingerprint.Adverbs.Unknown {
		t.Fatal("ten answers invent adverb measurement")
	}
}
