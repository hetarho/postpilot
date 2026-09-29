package voice

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMeasureKoreanEndingsAndUnicodeLength(t *testing.T) {
	got := Measure("오늘은 좋아요. 정말 좋습니다! 다음에도 온다.\n짧아요.")
	if len(got.Sentences) != 4 || got.AverageSentenceChars <= 0 {
		t.Fatalf("measure = %+v", got)
	}
	ratios := map[string]float64{}
	for _, item := range got.EndingDistribution {
		ratios[item.Ending] = item.Ratio
	}
	if ratios["해요"] != .5 || ratios["습니다"] != .25 || ratios["다"] != .25 {
		t.Fatalf("ending ratios = %+v", ratios)
	}
}

func TestKoreanAnalysisContractsRemainTheDefaultByteForByte(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) }
	text := "오늘은 좋아요. 정말 좋습니다!"
	if got, want := MeasuredProfileForLanguage(text, LanguageKorean, now), MeasuredProfile(text, now); !reflect.DeepEqual(got, want) {
		t.Fatalf("Korean measured profile changed\ngot=%#v\nwant=%#v", got, want)
	}
	if analysisPromptForLanguage(LanguageKorean) != analysisPrompt {
		t.Fatal("Korean analysis prompt changed through language selection")
	}
	if !bytes.Equal(VoiceAnalysisSchemaForLanguage(LanguageKorean), VoiceAnalysisSchema()) {
		t.Fatal("Korean analysis schema changed through language selection")
	}
}

func TestEnglishMeasurementUsesWordsRegisterCadenceAndSixAxes(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) }
	text := "However, we can't ignore this decision. John's proposal is formal and cannot be dismissed. Are you ready? Great! A fragment"
	got := MeasuredProfileForLanguage(text, LanguageEnglish, now)
	if got.Syntax.AverageSentenceWords == nil || *got.Syntax.AverageSentenceWords <= 0 {
		t.Fatalf("average words = %#v", got.Syntax.AverageSentenceWords)
	}
	if !strings.Contains(got.Endings.BaseRegister.Value, "contractions present") {
		t.Fatalf("register = %#v", got.Endings.BaseRegister)
	}
	if !reflect.DeepEqual(got.Syntax.PreferredConnectives, []string{"however"}) {
		t.Fatalf("connectives = %#v", got.Syntax.PreferredConnectives)
	}
	if !strings.Contains(got.Syntax.PassiveTendency.Value, "1 of") {
		t.Fatalf("passive tendency = %#v", got.Syntax.PassiveTendency)
	}
	cadence := map[string]float64{}
	for _, item := range got.Endings.Distribution {
		cadence[item.Ending] = item.Ratio
		if strings.ContainsAny(item.Ending, "가-힣") {
			t.Fatalf("English cadence used a Korean category: %#v", item)
		}
	}
	for _, category := range []string{"statement", "question", "exclamation", "fragment"} {
		if _, ok := cadence[category]; !ok {
			t.Fatalf("cadence category %q missing: %#v", category, cadence)
		}
	}
	for name, value := range map[string]*int{"involvement": got.Axes.Involvement, "narrativity": got.Axes.Narrativity, "persuasion": got.Axes.PersuasionOvertness, "abstractness": got.Axes.Abstractness, "addressee": got.Axes.AddresseeFocus, "humor": got.Axes.Humor} {
		if value == nil || *value < -3 || *value > 3 {
			t.Fatalf("axis %s = %#v", name, value)
		}
	}
}

func TestEnglishContractionsExcludePossessivesAndUncontractedCannot(t *testing.T) {
	for _, word := range []string{"can't", "we're", "I've", "you'll", "I'd", "I'm", "it's"} {
		if !englishContraction(strings.ToLower(word)) {
			t.Fatalf("actual contraction %q was not detected", word)
		}
	}
	for _, word := range []string{"john's", "company's", "cannot", "gonna"} {
		if englishContraction(word) {
			t.Fatalf("non-contraction %q was detected", word)
		}
	}
	formal := MeasuredProfileForLanguage("John's proposal cannot be dismissed.", LanguageEnglish, time.Now)
	if !strings.Contains(formal.Endings.BaseRegister.Value, "formal") {
		t.Fatalf("possessive/cannot made formal prose conversational: %#v", formal.Endings.BaseRegister)
	}
}

func TestEnglishAnalysisPromptAndSchemaSelection(t *testing.T) {
	if !strings.Contains(analysisPromptForLanguage(LanguageEnglish), "English writing-style analyst") || strings.Contains(analysisPromptForLanguage(LanguageEnglish), "한국어 문체") {
		t.Fatalf("English analysis prompt = %q", analysisPromptForLanguage(LanguageEnglish))
	}
	var schema map[string]any
	if err := json.Unmarshal(VoiceAnalysisSchemaForLanguage(LanguageEnglish), &schema); err != nil {
		t.Fatalf("English schema is invalid JSON: %v", err)
	}
	if !strings.Contains(structuredAnalysisPromptForLanguage(LanguageEnglish), "English authored corpus") || strings.Contains(structuredAnalysisPromptForLanguage(LanguageEnglish), "Korean authored corpus") {
		t.Fatalf("English structured analysis prompt = %q", structuredAnalysisPromptForLanguage(LanguageEnglish))
	}
}

func TestExcerptUsesFirstBoundaryBetweenTargetAndCap(t *testing.T) {
	body := strings.Repeat("가", 510) + "." + strings.Repeat("나", 400)
	got := excerptAroundTarget(body, 500, 800)
	if len([]rune(got)) != 511 || !strings.HasSuffix(got, ".") {
		t.Fatalf("excerpt length=%d suffix=%q", len([]rune(got)), got[len(got)-1:])
	}
	if got := excerptAroundTarget(strings.Repeat("가", 900), 500, 800); len([]rune(got)) != 800 {
		t.Fatalf("hard-capped excerpt length=%d", len([]rune(got)))
	}
}
