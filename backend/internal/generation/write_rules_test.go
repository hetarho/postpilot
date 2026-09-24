package generation

import (
	"strings"
	"testing"
)

// titleAreaBrief is testBrief with an authored title form, the case TMPL-52 is about.
func titleAreaBrief() *TemplateBrief {
	brief := testBrief()
	brief.TitleArea = "[맛집] <write>가게 이름과 대표 메뉴</write>"
	return brief
}

// GEN-49, GEN-50: the two title prohibitions and the tag rule are fixed write-prompt lines,
// once each, directly after the naming line and before the paragraph rule. A revision changes a
// title or a tag only on request, and neither observe pass writes one, so none of them — nor
// the nouns line, which asks for a member only the write answer has — reaches those prompts.
func TestTitleAndTagRulesAreInWritePromptsOnly(t *testing.T) {
	korean := func(brief *TemplateBrief, guidelines []string) string {
		return firstOf(BuildWritePrompt(goldenProfile(), goldenObservations(), "memo", "title", []string{"IMG_1.jpg"}, nil, brief, guidelines))
	}
	english := func(brief *TemplateBrief) string {
		return firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: LanguageEnglish, Profile: goldenProfile(), Observations: goldenObservations(), Memo: "memo", Title: "title", Photos: []string{"IMG_1.jpg"}, TagCount: 4, Template: brief}))
	}
	for name, test := range map[string]struct {
		prompt, naming, prohibitions, tags, paragraphRule string
	}{
		"Korean bare":           {korean(nil, nil), koreanNaming, koreanTitleProhibitions, koreanTagRule, "반드시 하나의 문단마다 TEXT 블록 하나만 사용하세요."},
		"Korean full":           {korean(testBrief(), testGuidelines()), koreanNaming, koreanTitleProhibitions, koreanTagRule, "반드시 하나의 문단마다 TEXT 블록 하나만 사용하세요."},
		"English bare":          {english(nil), englishNaming, englishTitleProhibitions, englishTagRule, "Use exactly one TEXT block for each paragraph."},
		"English with template": {english(testBrief()), englishNaming, englishTitleProhibitions, englishTagRule, "Use exactly one TEXT block for each paragraph."},
	} {
		for label, line := range map[string]string{"title prohibitions": test.prohibitions, "tag rule": test.tags} {
			if got := strings.Count(test.prompt, line); got != 1 {
				t.Errorf("%s carries the %s %d times", name, label, got)
			}
		}
		if want := test.naming + "\n" + test.prohibitions + "\n" + test.tags + "\n" + test.paragraphRule; !strings.Contains(test.prompt, want) {
			t.Errorf("%s: the title and tag lines are not directly after the naming line and before the paragraph rule", name)
		}
	}

	writeOnly := []string{
		koreanTitleProhibitions, englishTitleProhibitions, koreanTitleFormProhibitions, englishTitleFormProhibitions,
		koreanTagRule, englishTagRule, koreanNounsRule, englishNounsRule,
	}
	for name, prompt := range map[string]string{
		"Korean revise":  firstOf(BuildRevisePrompt(goldenProfile(), goldenContent(), nil, "고쳐줘", nil, titleAreaBrief(), testGuidelines())),
		"English revise": firstOf(BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 4, titleAreaBrief(), nil)),
		"photo observe":  ObservePrompt,
		"video observe":  ObserveVideoPrompt,
	} {
		for _, line := range writeOnly {
			if strings.Contains(prompt, line) {
				t.Errorf("%s carries the write-only line %q", name, line)
			}
		}
	}
}

// TMPL-52: an authored title form outranks the two prohibitions, which then bind only what the
// model writes inside the form's <write>. A post with no template, and one whose template
// authored no title, keep the plain line byte for byte — the prefix every golden pins.
func TestTitleProhibitionsYieldToATemplateTitleForm(t *testing.T) {
	for name, test := range map[string]struct {
		language            Language
		plain, form, static string
	}{
		"Korean":  {LanguageKorean, koreanTitleProhibitions, koreanTitleFormProhibitions, WritePrompt},
		"English": {LanguageEnglish, englishTitleProhibitions, englishTitleFormProhibitions, englishWritePrompt},
	} {
		// The exactly-once counts below can only tell the variants apart if neither holds the other.
		if strings.Contains(test.plain, test.form) || strings.Contains(test.form, test.plain) {
			t.Fatalf("%s: one title variant contains the other", name)
		}
		if !strings.Contains(test.form, "<write>") {
			t.Fatalf("%s: the form variant does not say the prohibitions bind only what is written inside <write>", name)
		}
		build := func(brief *TemplateBrief) string {
			return firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: test.language, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 4, Template: brief}))
		}
		for label, prompt := range map[string]string{"no template": build(nil), "a template with no title area": build(testBrief())} {
			if !strings.HasPrefix(prompt, test.static) || strings.Contains(prompt, test.form) {
				t.Errorf("%s, %s: the plain title line did not stay byte for byte", name, label)
			}
		}
		withForm := build(titleAreaBrief())
		if strings.Count(withForm, test.form) != 1 || strings.Contains(withForm, test.plain) {
			t.Errorf("%s: a title form did not put the form line in place of the plain one", name)
		}
		// Two stable prefixes, never a per-post one: the swap is the only difference.
		if want := strings.Replace(test.static, test.plain, test.form, 1); !strings.HasPrefix(withForm, want) {
			t.Errorf("%s: the title-form prefix differs from the plain one by more than that line", name)
		}
	}
}

// GUIDE-35, GUIDE-36, GUIDE-37: the profile no longer receives vocabulary wholesale. 문체 and
// 종결어미 stay with the voice; a concrete substitution ranks guideline > template > profile; an
// abstract instruction about better words carries no authority; and inside the guideline
// section the earlier line wins, which is what lets an owner's guideline beat the preset.
func TestPrecedenceSentencesRankConcreteSubstitutions(t *testing.T) {
	for name, sentence := range map[string]string{"template": templatePrecedence, "guideline": guidelinePrecedence} {
		if strings.Contains(sentence, "어휘") {
			t.Errorf("the %s precedence still hands 어휘 to the profile: %q", name, sentence)
		}
		// "따릅니다" in the template's sentence, "따르세요" in the guideline's.
		if !strings.Contains(sentence, "문체·종결어미는 위의 말투 프로필을 따") {
			t.Errorf("the %s precedence no longer keeps 문체 and 종결어미 with the voice: %q", name, sentence)
		}
		if !strings.Contains(sentence, `"A 대신 B라고 쓰세요"`) {
			t.Errorf("the %s precedence does not name a concrete substitution: %q", name, sentence)
		}
		if !strings.Contains(sentence, "더 나은 단어를 쓰라는 막연한 요구에는 그런 우선권이 없습니다") {
			t.Errorf("the %s precedence gives an abstract vocabulary instruction authority: %q", name, sentence)
		}
	}
	if !strings.Contains(templatePrecedence, "그 치환은 말투 프로필보다 우선") {
		t.Error("a template's concrete substitution does not outrank the profile")
	}
	if strings.Contains(templatePrecedence, "지침") {
		t.Errorf("the template precedence says 지침: %q", templatePrecedence)
	}
	for _, clause := range []string{
		"지침이 템플릿의 요구와 충돌하면 지침을 우선하고",
		"지침, 템플릿, 말투 프로필 순으로 우선",
		"지침끼리 충돌하면 먼저 적힌 지침을 따르세요",
	} {
		if !strings.Contains(guidelinePrecedence, clause) {
			t.Errorf("the guideline precedence lost %q", clause)
		}
	}
}
