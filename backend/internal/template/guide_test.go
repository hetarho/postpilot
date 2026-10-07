package template

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// productLimits are the shipped TEMPLATE_* defaults (platform/config), which are the numbers
// the golden guides were written with. testLimits narrows two of them for the shared fixture, so
// the guide cannot reuse it.
func productLimits() Limits {
	return NewLimits(Ceilings{
		NameMaxChars: 40, DescriptionMaxChars: 200, BodyMaxChars: 4000, TitleAreaMaxChars: 200,
		MaxPerAccount: 50, PhotoRowMax: 4, AskLabelMaxChars: 40, AskMaxPerBody: 10,
	}, NumberBounds{TargetLengthMin: 100, TargetLengthMax: 10_000, TagCountMin: 1, TagCountMax: 10})
}

var guideLanguages = []Language{LanguageKorean, LanguageEnglish}

func guideText(t *testing.T, language Language) string {
	t.Helper()
	text, err := FormatGuide(language, productLimits())
	if err != nil {
		t.Fatalf("FormatGuide(%s): %v", language, err)
	}
	return text
}

// The golden files are the guide the browser copied before the backend owned it, written from
// the client's own output. Matching them byte for byte is what makes moving the guide a move and
// not a rewrite (TMPL-41).
func TestFormatGuideMatchesTheGoldenText(t *testing.T) {
	for _, language := range guideLanguages {
		want, err := os.ReadFile(filepath.Join("testdata", "guide", string(language)+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		if got := guideText(t, language); got != string(want) {
			t.Errorf("%s guide drifted from testdata/guide/%s.txt", language, language)
		}
	}
}

// The example is a BODY, and this is what stops it drifting from the grammar it teaches.
func TestFormatGuideCarriesAnExampleTheParserAccepts(t *testing.T) {
	limits := productLimits()
	if _, _, err := ParseTemplate("", GuideExampleBody, ParseOptions{PhotoRowMax: limits.PhotoRowMax, AskMaxPerBody: limits.AskMaxPerBody}); err != nil {
		t.Fatalf("the example does not parse: %v", err)
	}
	for _, language := range guideLanguages {
		if !strings.Contains(guideText(t, language), GuideExampleBody) {
			t.Errorf("%s guide does not carry the example", language)
		}
	}
}

// Derived from the grammar rather than from the guide's prose: when TMPL-18 changes, this list
// and the shared fixture both fail until the guide follows.
func TestFormatGuideTeachesEveryAuthorableConstruct(t *testing.T) {
	constructs := []string{
		"<write>", "</write>", `<slot kind="photo"`, `count="`,
		`<repeat each="photo">`, "</repeat>", `<ask label="`, "</ask>",
	}
	for _, language := range guideLanguages {
		text := guideText(t, language)
		for _, construct := range constructs {
			if !strings.Contains(text, construct) {
				t.Errorf("%s guide does not teach %s", language, construct)
			}
		}
	}
}

func TestFormatGuideStatesEveryCeilingAsItsNumber(t *testing.T) {
	limits := productLimits()
	limits.BodyMaxChars, limits.PhotoRowMax, limits.AskMaxPerBody, limits.AskLabelMaxChars = 3210, 7, 9, 33
	for _, language := range guideLanguages {
		text, err := FormatGuide(language, limits)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range []int{3210, 7, 9, 33} {
			if !strings.Contains(text, strconv.Itoa(n)) {
				t.Errorf("%s guide does not state %d", language, n)
			}
		}
	}
}

// TMPL-19, TMPL-20: the refusals an outside AI would otherwise walk into — a tag inside text,
// and an attribute a tag does not name or gives twice.
func TestFormatGuideStatesTheTextOnlyAndAttributeRules(t *testing.T) {
	rules := map[Language][]string{
		LanguageKorean:  {"write, ask 안에는 글만", "위에 나온 속성만"},
		LanguageEnglish: {"hold text only", "only the attributes shown"},
	}
	for language, phrases := range rules {
		text := guideText(t, language)
		for _, phrase := range phrases {
			if !strings.Contains(text, phrase) {
				t.Errorf("%s guide does not say %q", language, phrase)
			}
		}
	}
}

func TestFormatGuideAllowsExperienceQuestionsWithoutWritingCommands(t *testing.T) {
	for language, phrases := range map[Language][]string{
		LanguageKorean:  {"직접 매긴 별점과 그 이유는 무엇인가요?", "실제 경험·확인한 사실·불확실한 점을 묻는 구체적인 질문", "말투·길이·서식·생략·반복을 지시하지"},
		LanguageEnglish: {"What rating did you give, and why?", "firsthand experience, verified facts, or uncertainty", "tone, length, formatting, omissions, or repetition"},
	} {
		guide := guideText(t, language)
		for _, phrase := range phrases {
			if !strings.Contains(guide, phrase) {
				t.Errorf("%s guide does not say %q", language, phrase)
			}
		}
	}
}

// Retired and never taught again (TMPL-37): a slot label belongs to the retired place and link
// positions, while `ask` carries a live `label` of its own, so the check is scoped to slots.
func TestFormatGuideTeachesNothingRetiredAndLeaksNoPlaceholder(t *testing.T) {
	slotLabel := regexp.MustCompile(`<slot[^>]*label=`)
	retired := []string{"place", "link", `<slot kind="place`, `<slot kind="link`, `label="이름`, "<note", "</note"}
	for _, language := range guideLanguages {
		text := guideText(t, language)
		if slotLabel.MatchString(text) {
			t.Errorf("%s guide teaches a slot label", language)
		}
		for _, word := range retired {
			if strings.Contains(text, word) {
				t.Errorf("%s guide mentions %q", language, word)
			}
		}
		for _, leftover := range []string{"{photoRowMax}", "{askLabelMax}", "{askMax}", "{bodyMax}", "{example}", "{{", "&lt;write"} {
			if strings.Contains(text, leftover) {
				t.Errorf("%s guide leaks %q", language, leftover)
			}
		}
	}
}

func TestFormatGuideRefusesAnUnsupportedLanguage(t *testing.T) {
	if _, err := FormatGuide("ja", productLimits()); !errors.Is(err, ErrUnsupportedLanguage) {
		t.Fatalf("err = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestAuthoringGrammarPreservesPublicMeaningWithoutBodyOnlyOutput(t *testing.T) {
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		public, err := FormatGuide(language, productLimits())
		if err != nil {
			t.Fatal(err)
		}
		grammar, err := GrammarGuide(language, productLimits())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(public, grammar) || !strings.Contains(grammar, GuideExampleBody) {
			t.Fatal("authoring grammar changed public meaning")
		}
		for _, text := range []string{"[답변 방식]", "[How to answer]", "Send the body only", "설명이나 코드 블록 없이 본문만"} {
			if strings.Contains(grammar, text) {
				t.Fatal("external response instruction reached authoring")
			}
		}
	}
	if _, err := GrammarGuide("ja", productLimits()); !errors.Is(err, ErrUnsupportedLanguage) {
		t.Fatal("unsupported grammar language accepted")
	}
}
