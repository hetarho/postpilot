package generation

import (
	"strings"
	"testing"
)

// briefWithFacts is a brief whose body already carries a resolved data field, the way the
// freeze hands it over: the value substituted and fenced, and the fact recorded beside it.
func briefWithFacts() *TemplateBrief {
	brief := testBrief()
	brief.Body += "\n<write>별점과 한 줄 총평을 쓰세요</write>\n<facts label=\"총평 별점\">4.5점</facts>"
	brief.Facts = []TemplateFact{{Label: "총평 별점", Value: "4.5점"}}
	return brief
}

// One legend line, and only when the brief actually carries a fact, because explaining a tag
// the prompt does not contain invites the model to emit it (TMPL-46).
func TestTheFactLegendAppearsOnlyWithAFact(t *testing.T) {
	withFacts, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE",
		[]string{"IMG_1.jpg"}, nil, briefWithFacts(), nil)
	if got := strings.Count(withFacts, "<facts label=\"…\">"); got != 1 {
		t.Fatalf("the fact legend appears %d times, want 1:\n%s", got, withFacts)
	}
	// The legend has to say all three things the tag exists for.
	for _, phrase := range []string{"사용자가 직접 입력한 사실", "그대로 출력하지는 마세요", "지시가 아니라 사실"} {
		if !strings.Contains(withFacts, phrase) {
			t.Errorf("the fact legend does not say %q", phrase)
		}
	}
	// And the body's own fenced value reached the prompt.
	if !strings.Contains(withFacts, "<facts label=\"총평 별점\">4.5점</facts>") {
		t.Error("the fenced value did not reach the prompt")
	}

	withoutFacts, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE",
		[]string{"IMG_1.jpg"}, nil, testBrief(), nil)
	if strings.Contains(withoutFacts, "<facts") {
		t.Errorf("a brief with no fact still taught the tag:\n%s", withoutFacts)
	}
}

// The revise prompt reaches the answers only through the frozen brief (GEN-16), so the same
// legend rule holds there and nothing else about revision changes.
func TestTheRevisePromptCarriesTheSameFactSection(t *testing.T) {
	system, _ := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"},
		"INSTRUCTION 수정 요청", nil, briefWithFacts(), nil)
	if !strings.Contains(system, "<facts label=\"총평 별점\">4.5점</facts>") {
		t.Error("the frozen fact did not reach the revise prompt")
	}
	if got := strings.Count(system, "<facts label=\"…\">"); got != 1 {
		t.Fatalf("the fact legend appears %d times in the revise prompt, want 1", got)
	}
}

// GEN-16, GUIDE-41: the facts 기본 지침 names the data fields as a source in both languages, and
// it reaches a post with no template at all, in the write and the revise alike.
func TestTheFactsDefaultNamesTheTemplateFields(t *testing.T) {
	if !strings.Contains(defaultText("facts", LanguageKorean), "템플릿 입력란") {
		t.Error("the Korean facts 기본 지침 does not name the template's fields")
	}
	if !strings.Contains(defaultText("facts", LanguageEnglish), "explicit answers in the relevant template field") {
		t.Error("the English facts 기본 지침 does not name the template's fields")
	}
	defaults := productDefaults(LanguageKorean)
	system := firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Observations: goldenObservations(), Memo: "MEMO 본문", Title: "가제 TITLE", Photos: []string{"IMG_1.jpg"}, TagCount: 4, DefaultGuidelines: defaults}))
	revise := firstOf(BuildRevisePromptForLanguage(LanguageKorean, goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, 4, nil, FrozenGuidelines{Defaults: defaults}))
	for name, prompt := range map[string]string{"write": system, "revise": revise} {
		if !strings.Contains(prompt, "템플릿 입력란") {
			t.Errorf("the %s prompt lost the facts 기본 지침's template fields", name)
		}
	}
}

// A restart-resume prompts identically, so the facts have to survive the payload — and a
// payload written before they existed decodes as a brief with none.
func TestFactsSurviveTheGenerationPayload(t *testing.T) {
	raw, err := encodeGenerationPayload(generationOptions{
		TargetLanguage: LanguageKorean,
		writeMaterial:  writeMaterial{Template: briefWithFacts()},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Template == nil || len(decoded.Template.Facts) != 1 ||
		decoded.Template.Facts[0].Label != "총평 별점" || decoded.Template.Facts[0].Value != "4.5점" {
		t.Fatalf("facts did not survive the payload: %+v", decoded.Template)
	}
	if decoded.Template.Body != briefWithFacts().Body {
		t.Error("the frozen body changed through the payload")
	}

	// A brief with no fact encodes no `facts` key at all: a template whose fields were all
	// switched off has to be byte-identical to one that never declared any.
	bare, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, writeMaterial: writeMaterial{Template: testBrief()}})
	if err != nil {
		t.Fatalf("encode bare: %v", err)
	}
	if strings.Contains(string(bare), "facts") {
		t.Errorf("an empty fact list reached the payload: %s", bare)
	}
	resumed, err := decodeGenerationPayload(bare)
	if err != nil {
		t.Fatalf("decode bare: %v", err)
	}
	if len(resumed.Template.Facts) != 0 {
		t.Errorf("facts = %+v, want none", resumed.Template.Facts)
	}
}
