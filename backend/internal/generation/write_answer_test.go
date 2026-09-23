package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// GEN-55: the nouns ride the write answer and the parser is their bound, whatever the model
// sends. The content half is exactly ParseContent's, and nothing about the nouns can fail a
// paid write.
func TestParseWriteAnswerBoundsTheNouns(t *testing.T) {
	const content = `"title":"제목","summary":"요약","tags":["a","b","c","d","e"],"blocks":[{"type":"TEXT","content":"본문"}]`
	forty := make([]string, 0, 45)
	quoted := make([]string, 0, 45)
	for i := 0; i < 45; i++ {
		noun := fmt.Sprintf("명사%d", i)
		quoted = append(quoted, `"`+noun+`"`)
		if i < WriteNounsMax {
			forty = append(forty, noun)
		}
	}
	for name, test := range map[string]struct {
		member string
		want   []string
	}{
		"trimmed, blanks dropped, one per noun keeping the first spelling": {
			member: `,"nouns":[" 카페 ",""," ","Cafe","cafe","커피","커피"]`,
			want:   []string{"카페", "Cafe", "커피"},
		},
		"capped in model order": {member: `,"nouns":[` + strings.Join(quoted, ",") + `]`, want: forty},
		"missing":               {member: ``},
		"null":                  {member: `,"nouns":null`},
		"empty":                 {member: `,"nouns":[]`},
		"not an array":          {member: `,"nouns":"카페"`},
		"not strings":           {member: `,"nouns":[1,2]`},
		"an object":             {member: `,"nouns":{"카페":1}`},
	} {
		raw := "{" + content + test.member + "}"
		answer, err := ParseWriteAnswer(raw, 4)
		if err != nil {
			t.Fatalf("%s: a paid write failed over its nouns: %v", name, err)
		}
		if !reflect.DeepEqual(answer.Nouns, test.want) {
			t.Errorf("%s: nouns = %q, want %q", name, answer.Nouns, test.want)
		}
		want, err := ParseContent(raw, 4)
		if err != nil || !reflect.DeepEqual(answer.Content, *want) {
			t.Errorf("%s: content = %+v, want ParseContent's %+v (%v)", name, answer.Content, want, err)
		}
		if len(answer.Content.Tags) != 4 {
			t.Errorf("%s: the tag bound did not apply, tags = %q", name, answer.Content.Tags)
		}
	}

	// A fenced answer parses the same way, and a missing required member fails both parsers alike.
	fenced, err := ParseWriteAnswer("```json\n{"+content+`,"nouns":["바다"]}`+"\n```", 4)
	if err != nil || !reflect.DeepEqual(fenced.Nouns, []string{"바다"}) {
		t.Fatalf("fenced answer = %+v, %v", fenced, err)
	}
	for _, raw := range []string{`{"title":"t","summary":"s","tags":[],"nouns":["바다"]}`, "not json at all"} {
		_, writeErr := ParseWriteAnswer(raw, 4)
		_, contentErr := ParseContent(raw, 4)
		if !errors.Is(writeErr, llm.ErrBadOutput) || !errors.Is(contentErr, llm.ErrBadOutput) {
			t.Errorf("%q: write error %v, content error %v; want both bad output", raw, writeErr, contentErr)
		}
	}
}

// The write schema is the content schema plus `nouns` and nothing else, in the same closed
// shape: every property required and none extra. The new member carries no enum, because a
// numeric enum makes Gemini return an empty object for the whole answer.
func TestWriteAnswerSchemaIsPostContentPlusNouns(t *testing.T) {
	var write, content map[string]any
	if err := json.Unmarshal(WriteAnswerSchema(), &write); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(PostContentSchema(), &content); err != nil {
		t.Fatal(err)
	}
	properties := write["properties"].(map[string]any)
	want := map[string]any{"type": "array", "maxItems": float64(WriteNounsMax), "items": map[string]any{"type": "string"}}
	if !reflect.DeepEqual(properties["nouns"], want) {
		t.Fatalf("nouns = %v, want %v", properties["nouns"], want)
	}
	if write["additionalProperties"] != false {
		t.Fatal("the write answer admits extra members")
	}
	required := map[string]bool{}
	for _, name := range write["required"].([]any) {
		required[name.(string)] = true
	}
	for name := range properties {
		if !required[name] {
			t.Errorf("%s is not required", name)
		}
	}

	delete(properties, "nouns")
	var withoutNouns []any
	for _, name := range write["required"].([]any) {
		if name != "nouns" {
			withoutNouns = append(withoutNouns, name)
		}
	}
	write["required"] = withoutNouns
	if !reflect.DeepEqual(write, content) {
		t.Fatalf("without nouns the write schema is not the content schema:\n%v\n%v", write, content)
	}

	// The prompt's number is the parser's, in both languages.
	for name, line := range map[string]string{"Korean": koreanNounsRule, "English": englishNounsRule} {
		if !strings.Contains(line, fmt.Sprint(WriteNounsMax)) {
			t.Errorf("the %s nouns line does not say %d", name, WriteNounsMax)
		}
	}
}

// Only the write pass asks for nouns: a structured write request carries the nouns schema and
// the shape line naming the member, while a revision keeps the stored nouns (GEN-55), so its
// request keeps the content schema and its own shape line.
func TestWriteSelectsTheNounsSchemaAndReviseKeepsPostContent(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("body")}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"title":"t","summary":"s","tags":["a"],"blocks":[{"type":"TEXT","content":"ok"}],"nouns":["카페"]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())

	answer, err := svc.write(context.Background(), PostInput{UserID: "alice", Voice: liveVoice, TargetLanguage: LanguageKorean}, nil, writeRef)
	if err != nil || !reflect.DeepEqual(answer.Nouns, []string{"카페"}) || len(answer.Content.Blocks) != 1 {
		t.Fatalf("answer = %+v, %v", answer, err)
	}
	write := models.calls[0].request
	if !bytes.Equal(write.JSONSchema, WriteAnswerSchema()) {
		t.Fatalf("the write request carries schema %s", write.JSONSchema)
	}
	if !strings.Contains(write.System, `{"title":"...","summary":"...","tags":[],"blocks":[],"nouns":[]}`) || !strings.Contains(write.System, koreanNounsRule) {
		t.Fatal("the write prompt does not ask for nouns")
	}

	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "고쳐줘", false),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	revise := models.calls[1].request
	if !bytes.Equal(revise.JSONSchema, PostContentSchema()) {
		t.Fatalf("the revise request carries schema %s", revise.JSONSchema)
	}
	if strings.Contains(revise.System, "nouns") || !strings.Contains(revise.System, `{"title":"...","summary":"...","tags":[],"blocks":[]}`) {
		t.Fatal("the revise prompt lost its own shape line or asks for nouns")
	}
}
