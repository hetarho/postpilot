package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

// GEN-14, GEN-48: the frozen phrases are per-post material, once, between the memo and the
// memories; the stable prefix every golden pins is the same with and without them.
func TestFieldPhrasesRenderOnceInThePerPostHalf(t *testing.T) {
	phrases := []string{"분위기 좋은 카페", "디저트 맛집"}
	section := FieldPhrasesHeading + "\n- 분위기 좋은 카페\n- 디저트 맛집\n" + fieldPhrasesObservation + "\n" + replacementsInstruction + "\n"
	for _, language := range []Language{LanguageKorean, LanguageEnglish} {
		baseSystem, baseUser := BuildWritePromptForLanguage(language, goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE",
			[]string{"IMG_1.jpg"}, nil, nil, 4, nil, nil, testMemories(), nil, nil)
		system, user := BuildWritePromptForLanguage(language, goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE",
			[]string{"IMG_1.jpg"}, nil, nil, 4, nil, nil, testMemories(), nil, phrases)
		if system != baseSystem {
			t.Fatalf("%s: the phrases reached the system prompt", language)
		}
		if strings.Count(user, FieldPhrasesHeading) != 1 {
			t.Fatalf("%s: the section appears %d times", language, strings.Count(user, FieldPhrasesHeading))
		}
		// Between the memo line and the memories, and the rest of the per-post half untouched.
		memo := "메모: MEMO 본문\n"
		if want := strings.Replace(baseUser, memo, memo+section, 1); user != want {
			t.Fatalf("%s: per-post half =\n%s\nwant\n%s", language, user, want)
		}
		if memoAt, sectionAt, memoriesAt := strings.Index(user, memo), strings.Index(user, FieldPhrasesHeading), strings.Index(user, "[기억]"); !(memoAt < sectionAt && sectionAt < memoriesAt) {
			t.Fatalf("%s: memo %d, phrases %d, memories %d", language, memoAt, sectionAt, memoriesAt)
		}
	}

	// QUAL-21: what was observed, never what it improves; and GEN-16: not a source of facts.
	for _, marker := range []string{"네이버 블로그", "상위 결과의 제목과 요약", "사실의 출처가 아니므로"} {
		if !strings.Contains(fieldPhrasesObservation, marker) {
			t.Errorf("the observation line lost %q", marker)
		}
	}
	for _, claim := range []string{"노출", "순위", "효과", "개선", "도움"} {
		if strings.Contains(fieldPhrasesObservation+replacementsInstruction, claim) {
			t.Errorf("the section claims %q", claim)
		}
	}
	if !strings.Contains(replacementsInstruction, "최대 20개") {
		t.Error("the instruction does not state the span bound")
	}
}

// With no phrases the prompt, the schema and the parse are exactly what the run had before.
func TestAWriteWithoutPhrasesIsUnchanged(t *testing.T) {
	wantSystem, wantUser := loadGolden(t, "write_prompt_no_template.golden")
	memoriesSystem, memoriesUser := loadGolden(t, "write_prompt_memories.golden")
	for name, none := range map[string][]string{"nil": nil, "empty": {}} {
		system, user := BuildWritePromptForLanguage(LanguageKorean, goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE",
			[]string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, post.TagCountRange.Default, nil, nil, nil, nil, none)
		if system != wantSystem || user != wantUser {
			t.Errorf("%s phrases moved the no-template golden", name)
		}
		system, user = BuildWritePromptForLanguage(LanguageKorean, goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE",
			[]string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, post.TagCountRange.Default, nil, nil, testMemories(), nil, none)
		if system != memoriesSystem || user != memoriesUser {
			t.Errorf("%s phrases moved the memories golden", name)
		}
	}

	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"title":"카페","summary":"s","tags":[],"nouns":[],"blocks":[{"type":"TEXT","content":"카페"}],
			"replacements":[{"surface":"title","index":0,"source":"카페","phrases":["분위기 좋은 카페"]}]}`}, nil
	}
	svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	answer, _, err := svc.writeCandidate(context.Background(), PostInput{UserID: "alice", TargetLanguage: LanguageKorean}, Profile{}, nil, writeRef)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(models.calls[0].request.JSONSchema, WriteAnswerSchema()) {
		t.Fatal("a write without phrases asked for the replacements schema")
	}
	if answer.Replacements != nil {
		t.Fatalf("a write without phrases kept %+v", answer.Replacements)
	}

	// With phrases, the replacements schema is asked for.
	with := PostInput{UserID: "alice", TargetLanguage: LanguageKorean, FieldPhrases: []string{"분위기 좋은 카페"}}
	if _, _, err := svc.writeCandidate(context.Background(), with, Profile{}, nil, writeRef); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(models.calls[1].request.JSONSchema, WriteAnswerReplacementsSchema()) {
		t.Fatal("a write with phrases did not ask for the replacements schema")
	}
}

// GEN-57: a revision keeps unrelated sentences verbatim, and a phrase sweep would rewrite them.
func TestTheRevisePromptCarriesNoFieldPhrases(t *testing.T) {
	for name, prompt := range map[string][2]string{
		"Korean":  toPair(BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "고쳐줘", nil, testBrief(), testGuidelines())),
		"English": toPair(BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 4, nil, nil)),
	} {
		for _, half := range prompt {
			if strings.Contains(half, FieldPhrasesHeading) || strings.Contains(half, "replacements") {
				t.Errorf("the %s revise prompt carries the phrase section", name)
			}
		}
	}

	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("body"), FieldPhrases: []string{"분위기 좋은 카페"},
	}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"title":"제목","summary":"요약","tags":["a"],"blocks":[{"type":"TEXT","content":"고친 본문"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "고쳐줘", false),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	request := models.calls[0].request
	for _, needle := range []string{FieldPhrasesHeading, "분위기 좋은 카페"} {
		if strings.Contains(request.System, needle) || strings.Contains(request.Messages[0].Parts[0].Text, needle) {
			t.Errorf("the revise request carries %q", needle)
		}
	}
}

// The replacements schema is the nouns schema plus one member, in the same closed shape, with a
// string-only enum: a numeric enum makes Gemini answer an empty object for the whole schema.
func TestWriteAnswerReplacementsSchemaExtendsTheNounsSchema(t *testing.T) {
	var withReplacements, nouns map[string]any
	if err := json.Unmarshal(WriteAnswerReplacementsSchema(), &withReplacements); err != nil {
		t.Fatalf("the replacements schema is not JSON: %v", err)
	}
	if err := json.Unmarshal(WriteAnswerSchema(), &nouns); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, node any)
	walk = func(path string, node any) {
		object, ok := node.(map[string]any)
		if !ok {
			return
		}
		if object["type"] == "object" {
			if object["additionalProperties"] != false {
				t.Errorf("%s admits extra members", path)
			}
			required := map[string]bool{}
			for _, name := range object["required"].([]any) {
				required[name.(string)] = true
			}
			for name := range object["properties"].(map[string]any) {
				if !required[name] {
					t.Errorf("%s.%s is not required", path, name)
				}
			}
		}
		if enum, ok := object["enum"].([]any); ok {
			for _, value := range enum {
				if _, isString := value.(string); !isString {
					t.Errorf("%s has a non-string enum value %v", path, value)
				}
			}
		}
		for key, child := range object {
			walk(path+"."+key, child)
		}
	}
	walk("$", withReplacements)

	replacements := withReplacements["properties"].(map[string]any)["replacements"].(map[string]any)
	if replacements["maxItems"] != float64(ReplacementSpansMax) {
		t.Errorf("replacements maxItems = %v, want %d", replacements["maxItems"], ReplacementSpansMax)
	}
	phrases := replacements["items"].(map[string]any)["properties"].(map[string]any)["phrases"].(map[string]any)
	if phrases["maxItems"] != float64(ReplacementPhrasesMax) {
		t.Errorf("phrases maxItems = %v, want %d", phrases["maxItems"], ReplacementPhrasesMax)
	}

	delete(withReplacements["properties"].(map[string]any), "replacements")
	var required []any
	for _, name := range withReplacements["required"].([]any) {
		if name != "replacements" {
			required = append(required, name)
		}
	}
	withReplacements["required"] = required
	if !reflect.DeepEqual(withReplacements, nouns) {
		t.Fatal("without replacements the schema is not the nouns schema")
	}
}
