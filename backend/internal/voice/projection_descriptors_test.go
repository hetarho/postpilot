package voice

import (
	"strings"
	"testing"
)

func descriptorProfile() StructuredProfile {
	v := func(s string) VoiceValue { return VoiceValue{Value: s, Source: SourceAnalyzed} }
	return StructuredProfile{
		Version: 4,
		Lexical: LexicalProfile{
			Description:    v("담백한 어휘"),
			PreferredWords: []WeightedWord{{Word: "맛있다", Alternatives: []string{"고소하다"}, Weight: 3}, {Word: "천천히", Weight: 1}},
		},
		Endings: EndingsProfile{
			BaseRegister:     v("해요"),
			Distribution:     []EndingRatio{{Ending: "해요", Ratio: 0.8}},
			SignatureEndings: []string{"~더라고요"},
			Constraints:      []string{"같은 어미 세 번 연속 금지"},
		},
		Syntax: SyntaxProfile{
			AverageSentenceChars: 21.5,
			SentenceLength:       v("짧음"),
			ConnectiveStyle:      v("그래서"),
			Nominalization:       v("낮음"),
			PassiveTendency:      v("거의 없음"),
		},
		Structure: StructureProfile{
			IntroPattern: v("바로 시작"), ClosingPattern: v("짧게 마침"),
			ParagraphSentencesMin: 2, ParagraphSentencesMax: 4,
			HeadingHabit: v("소제목 없음"), ListHabit: v("목록 드묾"), EmojiUse: v("안 씀"),
		},
	}
}

// VOICE-46, LANG-15: a Korean voice writing Korean receives the complete typed descriptors,
// its own structure included — never less than the same voice writing English.
func TestTheKoreanProjectionCarriesEveryTypedDescriptor(t *testing.T) {
	rendered := renderStructuredProfileForLanguage(descriptorProfile(), LanguageKorean)
	for _, want := range []string{
		"paragraph sentences: 2-4", "headings: 소제목 없음", "lists: 목록 드묾", "emojis: 안 씀",
		"sentence length: 짧음", "nominalization: 낮음", "passive: 거의 없음",
		"preferred words: 맛있다 (→고소하다), 천천히",
		"signatures: ~더라고요", "constraints: 같은 어미 세 번 연속 금지",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("the Korean projection lacks %q:\n%s", want, rendered)
		}
	}
	english := renderStructuredProfileForLanguage(descriptorProfile(), LanguageEnglish)
	if !strings.Contains(english, "preferred words: 맛있다 (→고소하다), 천천히") {
		t.Fatalf("the English projection lacks the preferred words:\n%s", english)
	}
	// Still an allowlist across languages: none of these crosses.
	portable := renderPortableProfile(descriptorProfile())
	for _, excluded := range []string{"preferred words", "signatures", "constraints", "sentence length"} {
		if strings.Contains(portable, excluded) {
			t.Fatalf("the portable projection carries %q", excluded)
		}
	}
}
