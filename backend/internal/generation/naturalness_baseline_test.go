package generation

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// GEN-17, GUIDE-41: the Korean naturalness baseline is the natural_korean 기본 지침 — its rule
// markers, none of the rejected folk rules, at most 700 runes, closed by the line that lets the
// voice outrank it.
func TestNaturalKoreanDefaultContract(t *testing.T) {
	baseline := defaultText("natural_korean", LanguageKorean)
	for family, marker := range map[string]string{
		"TEXT and revision scope":   "수정에서는 요청 밖의 기존 문장을 그대로 두세요",
		"non-TEXT exclusion":        "제목·요약·HEADING·LIST에는 적용하지 말고",
		"antithesis cap":            "A가 아니라 B",
		"cleft closer":              "핵심은 …이다",
		"formulaic closer":          "결국 …로 이어진다",
		"reason closer":             "~하는 이유다",
		"vague future":              "향후·앞으로",
		"empty concession":          "과제도 남아 있다",
		"obligation ending cap":     "~해야 한다",
		"connective-ending comma":   "-고/-며/-지만/-면서/-아서",
		"sentence-length variation": "짧은 문장과 긴 복문",
		"sentence-form variation":   "단문과 복문",
		"generic verbs":             "확대·강화·개선·확보·구축",
		"invented metaphors":        "잠식·청사진·신호탄",
		"abstract noun chains":      "~적 명사",
		"rhetorical decoration":     "수사·경구",
	} {
		if !strings.Contains(baseline, marker) {
			t.Errorf("%s marker %q is missing", family, marker)
		}
	}
	for _, rejected := range []string{
		"에 대해", "통해", "것이다",
		"문장 첫머리", "문장 처음", "문장 시작", "첫 단어", "문두", "접속사",
		"그리고", "그러나", "하지만", "그런데", "또한", "반면", "따라서", "그러므로",
	} {
		if strings.Contains(baseline, rejected) {
			t.Errorf("rejected folk-rule marker %q is present", rejected)
		}
	}
	if !strings.HasSuffix(baseline, "말투 프로필, 활성 대조 규칙, 사용자 규칙과 충돌하면 그쪽을 따르세요.") {
		t.Fatal("the voice's precedence must close the baseline")
	}
	if got := utf8.RuneCountInString(baseline); got > 700 {
		t.Fatalf("baseline is %d runes, want at most 700", got)
	}
	if english := defaultText("natural_korean", LanguageEnglish); english != "" {
		t.Fatalf("the Korean-only baseline has an English prompt text: %q", english)
	}
}

// It reaches a Korean write and revise exactly once, inside [작문 지침] and never before the
// styleguide; an English target and a switched-off one carry none of it.
func TestNaturalKoreanReachesKoreanRunsOnceInsideTheSection(t *testing.T) {
	baseline := defaultText("natural_korean", LanguageKorean)
	korean := productDefaults(LanguageKorean)
	for name, prompt := range map[string]string{
		"write":  firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 4, DefaultGuidelines: korean})),
		"revise": firstOf(BuildRevisePromptForLanguage(LanguageKorean, goldenProfile(), goldenContent(), nil, "고쳐줘", nil, 4, nil, FrozenGuidelines{Defaults: korean})),
	} {
		at := strings.Index(prompt, rendered(baseline))
		if strings.Count(prompt, rendered(baseline)) != 1 || at < strings.Index(prompt, "\n\n[작문 지침]") {
			t.Errorf("%s: the baseline is not once inside [작문 지침]", name)
		}
		if at < strings.Index(prompt, "[스타일가이드]") {
			t.Errorf("%s: the baseline sits before the voice profile", name)
		}
	}
	for name, prompt := range map[string]string{
		"English write": firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: LanguageEnglish, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 4, DefaultGuidelines: productDefaults(LanguageEnglish)})),
		"switched off":  firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 4, DefaultGuidelines: without(korean, baseline)})),
	} {
		if strings.Contains(prompt, "A가 아니라 B") || strings.Contains(prompt, "잠식·청사진·신호탄") {
			t.Errorf("%s carries the Korean baseline", name)
		}
	}
}

func TestWriteSystemPrefixIsStableAcrossPostMaterial(t *testing.T) {
	target := 900
	profile := goldenProfile()
	template := testBrief()
	first, firstUser := BuildWritePrompt(profile, goldenObservations(), "first memo", "first title", []string{"one.jpg"}, &target, template, nil)
	second, secondUser := BuildWritePrompt(profile, []Observation{{File: "two.jpg"}}, "second memo", "second title", []string{"two.jpg"}, &target, template, nil)

	if first != second {
		t.Fatal("per-post material changed the byte-stable system prefix")
	}
	if firstUser == secondUser {
		t.Fatal("fixture error: per-post user material did not change")
	}
}
