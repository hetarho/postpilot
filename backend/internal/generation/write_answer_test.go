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
		answer, err := ParseWriteAnswer(raw, 4, nil)
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
	fenced, err := ParseWriteAnswer("```json\n{"+content+`,"nouns":["바다"]}`+"\n```", 4, nil)
	if err != nil || !reflect.DeepEqual(fenced.Nouns, []string{"바다"}) {
		t.Fatalf("fenced answer = %+v, %v", fenced, err)
	}
	for _, raw := range []string{`{"title":"t","summary":"s","tags":[],"nouns":["바다"]}`, "not json at all"} {
		_, writeErr := ParseWriteAnswer(raw, 4, nil)
		_, contentErr := ParseContent(raw, 4)
		if !errors.Is(writeErr, llm.ErrBadOutput) || !errors.Is(contentErr, llm.ErrBadOutput) {
			t.Errorf("%q: write error %v, content error %v; want both bad output", raw, writeErr, contentErr)
		}
	}
}

// The write schema is the content schema plus `nouns` and `storyline` and nothing else, in the
// same closed shape: every property required and none extra. Neither new member carries an
// enum, because a numeric enum makes Gemini return an empty object for the whole answer.
func TestWriteAnswerSchemaIsPostContentPlusNounsAndStoryline(t *testing.T) {
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
	storyline := map[string]any{"type": "array", "items": map[string]any{
		"type": "object", "additionalProperties": false, "required": []any{"text", "files"},
		"properties": map[string]any{
			"text":  map[string]any{"type": "string"},
			"files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}}
	if !reflect.DeepEqual(properties["storyline"], storyline) {
		t.Fatalf("storyline = %v, want %v", properties["storyline"], storyline)
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
	delete(properties, "storyline")
	var contentOnly []any
	for _, name := range write["required"].([]any) {
		if name != "nouns" && name != "storyline" {
			contentOnly = append(contentOnly, name)
		}
	}
	write["required"] = contentOnly
	if !reflect.DeepEqual(write, content) {
		t.Fatalf("without nouns and storyline the write schema is not the content schema:\n%v\n%v", write, content)
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
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())

	answer, err := svc.write(context.Background(), PostInput{UserID: "alice", Voice: liveVoice, TargetLanguage: LanguageKorean}, nil, writeRef)
	if err != nil || !reflect.DeepEqual(answer.Nouns, []string{"카페"}) || len(answer.Content.Blocks) != 1 {
		t.Fatalf("answer = %+v, %v", answer, err)
	}
	write := models.calls[0].request
	if !bytes.Equal(write.JSONSchema, WriteAnswerSchema()) {
		t.Fatalf("the write request carries schema %s", write.JSONSchema)
	}
	if !strings.Contains(write.System, `{"storyline":[{"text":"...","files":[]}],"title":"...","summary":"...","tags":[],"blocks":[],"nouns":[]}`) ||
		!strings.Contains(write.System, koreanNounsRule) || !strings.Contains(write.System, koreanStorylineRule) {
		t.Fatal("the write prompt does not ask for the storyline and nouns")
	}

	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "고쳐줘"),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	revise := models.calls[1].request
	if !bytes.Equal(revise.JSONSchema, PostContentSchema()) {
		t.Fatalf("the revise request carries schema %s", revise.JSONSchema)
	}
	if strings.Contains(revise.System, "nouns") || strings.Contains(revise.System, "storyline") ||
		!strings.Contains(revise.System, `{"title":"...","summary":"...","tags":[],"blocks":[]}`) {
		t.Fatal("the revise prompt lost its own shape line or asks for nouns or a storyline")
	}
}

// GEN-55: the write answer is the content and its nouns. A replacements member a model still
// sends parses to exactly the answer without it, and never fails the write.
func TestParseWriteAnswerIgnoresAReplacementsMember(t *testing.T) {
	const content = `"title":"성수 카페","summary":"요약","tags":["카페"],"nouns":["성수","카페"],"blocks":[{"type":"TEXT","content":"성수동 카페"}]`
	want, err := ParseWriteAnswer("{"+content+"}", 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, member := range map[string]string{
		"null":         `,"replacements":null`,
		"not an array": `,"replacements":{"surface":"title"}`,
		"items":        `,"replacements":[{"surface":"body","index":0,"source":"성수동 카페","phrases":["분위기 좋은 카페"]}]`,
	} {
		got, err := ParseWriteAnswer("{"+content+member+"}", 4, nil)
		if err != nil {
			t.Fatalf("%s: the write failed over a replacements member: %v", name, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: answer = %+v, want %+v", name, got, want)
		}
	}
}

// GEN-67: the schema reaches the provider as its bytes (json.RawMessage), so the key order in
// the file is the order the model is asked to answer in: storyline first, before a block.
func TestWriteAnswerSchemaAsksForTheStorylineFirst(t *testing.T) {
	firstKey := func(t *testing.T, decoder *json.Decoder) string {
		t.Helper()
		token, err := decoder.Token()
		if err != nil {
			t.Fatal(err)
		}
		key, ok := token.(string)
		if !ok {
			t.Fatalf("expected a key, got %v", token)
		}
		return key
	}
	decoder := json.NewDecoder(bytes.NewReader(WriteAnswerSchema()))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		t.Fatalf("schema opens with %v (%v)", token, err)
	}
	var properties, required string
	for decoder.More() {
		key := firstKey(t, decoder)
		switch key {
		case "properties":
			if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
				t.Fatalf("properties opens with %v (%v)", token, err)
			}
			properties = firstKey(t, decoder)
			depth := 1
			for depth > 0 {
				token, err := decoder.Token()
				if err != nil {
					t.Fatal(err)
				}
				switch token {
				case json.Delim('{'), json.Delim('['):
					depth++
				case json.Delim('}'), json.Delim(']'):
					depth--
				}
			}
		case "required":
			var names []string
			if err := decoder.Decode(&names); err != nil || len(names) == 0 {
				t.Fatalf("required = %v (%v)", names, err)
			}
			required = names[0]
		default:
			var skip json.RawMessage
			if err := decoder.Decode(&skip); err != nil {
				t.Fatal(err)
			}
		}
	}
	if properties != "storyline" || required != "storyline" {
		t.Fatalf("first property %q, first required %q; want storyline for both", properties, required)
	}
}

// GEN-67: the parser is the storyline's bound, whatever the model sends, and nothing about the
// storyline fails a paid write.
func TestParseWriteAnswerBoundsTheStoryline(t *testing.T) {
	const content = `"title":"제목","summary":"요약","tags":["a"],"blocks":[{"type":"TEXT","content":"본문"}],"nouns":[]`
	attachments := []string{"a.jpg", "b.jpg", "clip.mp4"}
	long := strings.Repeat("가", StorylineTextMaxChars+5)
	many := make([]string, 0, StorylineParagraphMax+3)
	for i := 0; i < StorylineParagraphMax+3; i++ {
		many = append(many, fmt.Sprintf(`{"text":"문단 %d","files":[]}`, i))
	}
	for name, test := range map[string]struct {
		member string
		want   []StorylineParagraph
	}{
		"kept, trimmed, in order": {
			member: `,"storyline":[{"text":" 가게 앞을 보여줍니다. ","files":["a.jpg"]},{"text":"영상으로 분위기를 보여줍니다.","files":["clip.mp4","b.jpg"]}]`,
			want: []StorylineParagraph{
				{Text: "가게 앞을 보여줍니다.", Files: []string{"a.jpg"}},
				{Text: "영상으로 분위기를 보여줍니다.", Files: []string{"clip.mp4", "b.jpg"}},
			},
		},
		"an unknown name dropped, a repeated one kept in its first paragraph": {
			member: `,"storyline":[{"text":"하나","files":["a.jpg","ghost.jpg","a.jpg"]},{"text":"둘","files":["a.jpg","A.JPG","b.jpg"]}]`,
			want: []StorylineParagraph{
				{Text: "하나", Files: []string{"a.jpg"}},
				{Text: "둘", Files: []string{"b.jpg"}},
			},
		},
		"a paragraph with neither text nor a file dropped": {
			member: `,"storyline":[{"text":"  ","files":[]},{"text":"","files":["ghost.jpg"]},{"text":"","files":["b.jpg"]}]`,
			want:   []StorylineParagraph{{Files: []string{"b.jpg"}}},
		},
		"text cut at a rune boundary": {
			member: `,"storyline":[{"text":"` + long + `","files":[]}]`,
			want:   []StorylineParagraph{{Text: strings.Repeat("가", StorylineTextMaxChars)}},
		},
		"missing":      {member: ``},
		"null":         {member: `,"storyline":null`},
		"empty":        {member: `,"storyline":[]`},
		"not an array": {member: `,"storyline":"첫 문단"`},
		"not objects":  {member: `,"storyline":["첫 문단"]`},
	} {
		answer, err := ParseWriteAnswer("{"+content+test.member+"}", 4, attachments)
		if err != nil {
			t.Fatalf("%s: a paid write failed over its storyline: %v", name, err)
		}
		if answer.Storyline == nil {
			t.Fatalf("%s: a parsed write has no storyline value", name)
		}
		if !reflect.DeepEqual(answer.Storyline.Paragraphs, test.want) {
			t.Errorf("%s: storyline = %+v, want %+v", name, answer.Storyline.Paragraphs, test.want)
		}
	}

	capped, err := ParseWriteAnswer(`{"storyline":[`+strings.Join(many, ",")+`],`+content+`}`, 4, attachments)
	if err != nil || len(capped.Storyline.Paragraphs) != StorylineParagraphMax || capped.Storyline.Paragraphs[0].Text != "문단 0" {
		t.Fatalf("capped storyline = %d paragraphs (%v)", len(capped.Storyline.Paragraphs), err)
	}
}
