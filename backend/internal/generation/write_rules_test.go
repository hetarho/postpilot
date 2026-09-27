package generation

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/guideline"
)

// titleAreaBrief is testBrief with an authored title form, the case TMPL-52 is about.
func titleAreaBrief() *TemplateBrief {
	brief := testBrief()
	brief.TitleArea = "[맛집] <write>가게 이름과 대표 메뉴</write>"
	return brief
}

// productDefaults are the 기본 지침 texts a post run in target freezes with every switch on — the
// guideline context's own registry, read through its published texts (GUIDE-41).
func productDefaults(target Language) []string {
	var out []string
	for _, d := range guideline.Defaults(guideline.KindPost) {
		if text, ok := d.Text(guidelineLanguage(target)); ok {
			out = append(out, text)
		}
	}
	return out
}

// defaultText is one 기본 지침's text in a language, and "" when it does not reach that target.
func defaultText(key string, target Language) string {
	d, _ := guideline.DefaultFor(guideline.KindPost, key)
	text, _ := d.Text(guidelineLanguage(target))
	return text
}

func guidelineLanguage(target Language) guideline.Language {
	if target == LanguageEnglish {
		return guideline.LanguageEnglish
	}
	return guideline.LanguageKorean
}

// rendered is a text as [작문 지침] draws it: one bullet, its continuation lines indented.
func rendered(text string) string { return "- " + strings.ReplaceAll(text, "\n", "\n  ") }

func without(texts []string, drop string) []string {
	var out []string
	for _, text := range texts {
		if text != drop {
			out = append(out, text)
		}
	}
	return out
}

// GUIDE-1, GEN-14: the static rules of every pass hold the format alone. No writing rule of any
// 기본 지침, in either language, sits in the write, revise or observe prompts' fixed text.
func TestStaticRulesHoldTheFormatAlone(t *testing.T) {
	if !strings.HasPrefix(WritePrompt, "첨부 사진 관찰과 메모를 바탕으로 한국어 블로그 글을 작성하세요.\n") {
		t.Fatalf("the Korean task sentence changed: %q", WritePrompt[:80])
	}
	if !strings.HasPrefix(englishWritePrompt, "Write an English blog post from the photo observations and memo.\n") {
		t.Fatalf("the English task sentence changed: %q", englishWritePrompt[:80])
	}
	for name, static := range map[string]string{
		"Korean write": WritePrompt, "English write": englishWritePrompt,
		"Korean revise": RevisePrompt, "English revise": englishRevisePrompt,
		"photo observe": ObservePrompt, "video observe": ObserveVideoPrompt,
	} {
		for _, d := range guideline.Defaults(guideline.KindPost) {
			for _, text := range []string{d.Ko.Text, d.En.Text} {
				for _, line := range strings.Split(text, "\n") {
					if strings.Contains(static, line) {
						t.Errorf("%s carries the %s 기본 지침 line %q", name, d.Key, line)
					}
				}
			}
		}
		for _, gone := range []string{"자연스러운 한국어", "natural English", "지어내지", "invent interactions"} {
			if strings.Contains(static, gone) {
				t.Errorf("%s still carries %q", name, gone)
			}
		}
	}
	// The nouns line asks for a member only the write answer has.
	for name, prompt := range map[string]string{"Korean revise": RevisePrompt, "English revise": englishRevisePrompt, "photo observe": ObservePrompt} {
		if strings.Contains(prompt, koreanNounsRule) || strings.Contains(prompt, englishNounsRule) {
			t.Errorf("%s carries the nouns line", name)
		}
	}
}

// GUIDE-41, GUIDE-43: each moved rule reaches the write and the revise once as its 기본 지침 while
// it is on, inside [작문 지침], and not at all once it is switched off.
func TestEachMovedRuleIsItsDefaultOnceWhenOnAndAbsentWhenOff(t *testing.T) {
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		defaults := productDefaults(language)
		write := func(defaults []string) string {
			return firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 4, DefaultGuidelines: defaults}))
		}
		revise := func(defaults []string) string {
			return firstOf(BuildRevisePromptForLanguage(language, goldenProfile(), goldenContent(), nil, "고쳐줘", nil, 4, nil, FrozenGuidelines{Defaults: defaults}))
		}
		for pass, build := range map[string]func([]string) string{"write": write, "revise": revise} {
			on := build(defaults)
			section := on[strings.Index(on, "\n\n[작문 지침]"):]
			for _, text := range defaults {
				if strings.Count(on, rendered(text)) != 1 || !strings.Contains(section, rendered(text)) {
					t.Errorf("%s %s: a 기본 지침 is not once inside [작문 지침]: %q", language, pass, text)
				}
				off := build(without(defaults, text))
				if strings.Contains(off, text) {
					t.Errorf("%s %s: a switched-off 기본 지침 still reached the prompt: %q", language, pass, text)
				}
			}
		}
	}
}

// TMPL-52: the static prefix no longer changes with a title form; the titles 기본 지침 says itself
// that a template's title form comes first and that the rule then binds only what is written
// inside its <write>.
func TestATitleFormChangesNoStaticRule(t *testing.T) {
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		build := func(brief *TemplateBrief) string {
			return firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: language, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 4, Template: brief}))
		}
		static := writeStaticRules(language)
		for label, brief := range map[string]*TemplateBrief{"no template": nil, "no title area": testBrief(), "a title form": titleAreaBrief()} {
			if !strings.HasPrefix(build(brief), static) {
				t.Errorf("%s, %s: the static prefix moved", language, label)
			}
		}
		if titles := defaultText("titles", language); !strings.Contains(titles, "<write>") {
			t.Errorf("%s: the titles 기본 지침 does not bind the title form's <write>: %q", language, titles)
		}
	}
}

// GUIDE-15, GUIDE-35, GUIDE-36, GUIDE-37: 문체 and 종결어미 stay with the voice; a concrete
// substitution ranks guideline > template > profile; an abstract instruction about better words
// carries no authority; the owner's line outranks a 기본 지침; inside one group the earlier wins.
func TestPrecedenceSentencesRankConcreteSubstitutions(t *testing.T) {
	for name, sentence := range map[string]string{"template": templatePrecedence, "guideline": guidelinePrecedence} {
		if strings.Contains(sentence, "어휘") {
			t.Errorf("the %s precedence still hands 어휘 to the profile: %q", name, sentence)
		}
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
		"지침은 이 글을 어떻게 쓸지 정합니다",
		"지침이 템플릿과 충돌하면 지침을 우선하고",
		"지침, 템플릿, 말투 프로필 순으로 우선",
		"사용자 지침이 기본 지침과 충돌하면 사용자 지침을 따르고",
		"같은 묶음 안에서 충돌하면 먼저 적힌 지침을 따르세요",
	} {
		if !strings.Contains(guidelinePrecedence, clause) {
			t.Errorf("the guideline precedence lost %q", clause)
		}
	}
}
