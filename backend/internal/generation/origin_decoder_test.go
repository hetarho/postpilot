package generation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

const originDecoderCore = `"title":"카페😀","summary":"한 줄","tags":[],"blocks":[{"type":"TEXT","content":"본문😀"}]`
const originDecoderWriteCore = `"storyline":[{"text":"본문😀","files":[]}],` + originDecoderCore + `,"nouns":[]`

func TestOriginTailSalvageRequiresWholeTypedStageOutput(t *testing.T) {
	tests := []struct {
		name, core string
		parse      func(string) error
	}{
		{"content", originDecoderCore, func(raw string) error { _, err := ParseContent(raw, 4); return err }},
		{"revision", originDecoderCore, func(raw string) error { _, err := ParseRevisionContent(raw, 4, PostContent{}); return err }},
		{"direct write", originDecoderWriteCore, func(raw string) error { _, err := ParseWriteAnswer(raw, 4, nil); return err }},
		{"along plan", originDecoderCore + `,"nouns":[]`, func(raw string) error { _, err := ParseWriteAlongStorylineAnswer(raw, 4, nil); return err }},
		{"plan", `"storyline":[{"text":"본문😀","files":[]}]`, func(raw string) error { _, err := ParseStorylineAnswer(raw, nil); return err }},
		{"observe", `"observations":[{"file":"photo.jpg","scene":"카페😀","mood":"밝다","visible_text":"","objects":[],"people_present":false}]`, func(raw string) error { _, err := parseObservations(raw); return err }},
	}
	for _, stage := range tests {
		t.Run(stage.name, func(t *testing.T) {
			for _, tail := range []string{"", `:`, `:[`, `:[{"field":`, `:[{"quote":"잘린😀`, `:[broken]}`, `:false`, `:[]`} {
				raw := "{" + stage.core + `,"origins"` + tail
				for _, wrap := range []func(string) string{func(s string) string { return s }, func(s string) string { return "```json\n" + s + "\n```" }, func(s string) string { return "결과입니다.\n" + s }} {
					if err := stage.parse(wrap(raw)); err != nil {
						t.Errorf("complete canonical stage rejected origin-only tail %q: %v", tail, err)
					}
				}
			}
			if err := stage.parse("{" + stage.core + "}"); err != nil {
				t.Fatalf("valid legacy stage rejected: %v", err)
			}
		})
	}
	for _, raw := range []string{
		"{" + originDecoderCore + `,"origins":[`,
		"{" + originDecoderCore + `,"nouns":[],"origins":[`,
		"{" + originDecoderWriteCore[:len(originDecoderWriteCore)-2] + `"wrong","origins":[`,
		"{" + originDecoderCore + `,"nouns":"wrong","storyline":[],"origins":[`,
		"{" + originDecoderCore + `,"nouns":[],"storyline":"wrong","origins":[`,
	} {
		if _, err := ParseWriteAnswer(raw, 4, nil); err == nil {
			t.Fatalf("direct-write salvage admitted missing/malformed complete stage member: %s", raw)
		}
	}
	// Full legacy JSON retains the original leniency for these auxiliary members.
	for _, member := range []string{"", `,"nouns":"wrong"`, `,"storyline":"wrong"`} {
		if _, err := ParseWriteAnswer("{"+originDecoderCore+member+"}", 4, nil); err != nil {
			t.Fatal("origin support tightened a full legacy write", err)
		}
	}
}

func TestObservationCandidatesBindOnlyUniqueExactReturnedFiles(t *testing.T) {
	const observation = `{"file":"photo.jpg","scene":"카페😀","mood":"밝다","visible_text":"","objects":["컵"],"people_present":false}`
	const origins = `[{"file":"photo.jpg","field":{"kind":"observation_scene"},"quote":"카페😀","category":"photo_interpretation","source_refs":["media.0"]},{"file":"another.jpg","field":{"kind":"observation_scene"},"quote":"다른 사진","category":"photo_interpretation","source_refs":["media.1"]}]`
	parsed, err := parseObservations(`{"observations":[` + observation + `],"origins":` + origins + `}`)
	if err != nil || len(parsed) != 1 || len(parsed[0].OriginCandidates) != 1 || parsed[0].OriginCandidates[0].Field != "scene" || parsed[0].OriginCandidates[0].Quote != "카페😀" {
		t.Fatalf("observation candidate locator changed: %+v %v", parsed, err)
	}
	duplicates, err := parseObservations(`{"observations":[` + observation + `,` + observation + `],"origins":` + origins + `}`)
	if err != nil || len(duplicates) != 2 || len(duplicates[0].OriginCandidates) != 0 || len(duplicates[1].OriginCandidates) != 0 {
		t.Fatalf("ambiguous file matched an arbitrary observation: %+v %v", duplicates, err)
	}
}

func TestParsedPlanCandidatesFollowExplicitDroppedParagraphMap(t *testing.T) {
	raw := `{"storyline":[{"text":" ","files":[]},{"text":"  본문😀  ","files":[]}],"origins":[{"field":{"kind":"storyline_paragraph","paragraph_index":1},"quote":"본문😀","category":"ai_added","source_refs":[]},{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":" ","category":"ai_added","source_refs":[]}]}`
	paragraphs, candidates, err := ParseStorylineAnswerWithOrigins(raw, nil)
	if err != nil || len(paragraphs) != 1 || paragraphs[0].Text != "본문😀" || len(candidates) != 2 || candidates[0].ParagraphIndex != 0 || candidates[1].ParagraphIndex != -1 {
		t.Fatalf("plan locators guessed after paragraph filtering: %+v %+v %v", paragraphs, candidates, err)
	}
}

func TestOriginTailNeverRepairsCoreOrSelectsNestedOuterObjects(t *testing.T) {
	full := "{" + originDecoderCore + "}"
	for name, raw := range map[string]string{
		"duplicate core":        "{" + originDecoderCore + `,"title":"second"}`,
		"duplicate before tail": "{" + originDecoderCore + `,"title":"second","origins":[`,
		"wrong canonical type":  `{"title":7,"summary":"s","tags":[],"blocks":[],"origins":[`,
		"missing core":          `{"title":"t","tags":[],"blocks":[],"origins":[`,
		"unfinished block":      `{"title":"t","summary":"s","tags":[],"blocks":[{"type":"TEXT","content":"cut","origins":[`,
		"origins before core":   `{"origins":[{"quote":"cut",` + originDecoderCore,
		"unknown before tail":   "{" + originDecoderCore + `,"answer":{},"origins":[`,
		"following member":      "{" + originDecoderCore + `,"origins":[broken],"title":"later"}`,
		"following object":      "{" + originDecoderCore + `,"origins":[broken]} {"second":true}`,
		"following array":       "{" + originDecoderCore + `,"origins":[broken]} ]`,
		"array":                 "[" + full + "]",
		"prefaced array":        "Here: [" + full + "]",
		"quoted object":         `Here: "` + full + `"`,
		"nested object":         `{"answer":` + full + `}`,
		"second object":         full + " " + full,
		"closing array":         "Here: " + full + "]",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseContent(raw, 4); !errors.Is(err, llm.ErrBadOutput) {
				t.Fatalf("ambiguous/incomplete canonical output was salvaged: %v", err)
			}
		})
	}
	for _, raw := range []string{full, "```json\n" + full + "\n```", "결과입니다.\n" + full + "\n끝"} {
		if _, err := ParseContent(raw, 4); err != nil {
			t.Fatalf("known complete wrapper rejected: %v", err)
		}
	}
	if _, err := ParseContent("{"+originDecoderCore+`,"replacements":{"legacy":"ignored"}}`, 4); err != nil {
		t.Fatal("fully decoded legacy auxiliary field changed canonical parsing", err)
	}
}

func TestOriginCandidateTransportUsesQuotesAndPreservesCanonicalContent(t *testing.T) {
	annotation := `[{"field":{"kind":"title"},"quote":"카페😀","category":"owner_input","source_refs":["memo"],"start":999,"end":1000},{"field":{"kind":"storyline_paragraph","paragraph_index":0},"quote":"본문😀","occurrence":0,"category":"ai_added","source_refs":[]}]`
	answer, err := ParseWriteAnswer("{"+originDecoderWriteCore+`,"origins":`+annotation+"}", 4, nil)
	if err != nil || len(answer.OriginCandidates) != 1 || answer.OriginCandidates[0].Quote != "카페😀" || answer.OriginCandidates[0].Field.Kind != post.OriginFieldTitle || len(answer.Storyline.OriginCandidates) != 1 || answer.Storyline.OriginCandidates[0].ParagraphIndex != 0 {
		t.Fatalf("quote transport changed: %+v %v", answer, err)
	}
	legacy, err := ParseWriteAnswer("{"+originDecoderWriteCore+"}", 4, nil)
	if err != nil || !reflect.DeepEqual(answer.Content, legacy.Content) {
		t.Fatal("annotations changed usable canonical content", err)
	}
	for _, member := range []string{`"origins":null`, `"origins":"wrong"`, `"origins":[{"quote":7}]`, `"origins":[],"origins":[]`} {
		parsed, err := ParseWriteAnswer("{"+originDecoderWriteCore+","+member+"}", 4, nil)
		if err != nil || !reflect.DeepEqual(parsed.Content, legacy.Content) || len(parsed.OriginCandidates) != 0 {
			t.Fatalf("annotation failure rejected/changed content: %+v %v", parsed, err)
		}
	}
	if parsed, err := ParseWriteAnswer("{"+originDecoderWriteCore+`,"origins":[],"origins":[`, 4, nil); err != nil || len(parsed.OriginCandidates) != 0 || !reflect.DeepEqual(parsed.Content, legacy.Content) {
		t.Fatal("optional duplicate/malformed origins discarded a complete core", err)
	}
	// Metadata may be reordered when the whole answer is valid, never when core
	// members are unfinished behind that metadata.
	if _, err := ParseContent(`{"origins":[],`+originDecoderCore+"}", 4); err != nil {
		t.Fatal("fully decoded reordered optional metadata rejected", err)
	}
}

func TestTerminalLengthRetainsUsableCanonicalMembersAndExistingFailureUsage(t *testing.T) {
	usable := "{" + originDecoderWriteCore + `,"origins":[{"quote":"cut`
	_, err := ParseWriteAnswer(usable, 4, nil)
	response := llm.Response{Text: usable, FinishReason: "length", Usage: llm.Usage{PromptTokens: 12, CompletionTokens: 30, ReasoningTokens: 7}}
	if err := responseParseError(response, err); err != nil {
		t.Fatal("normal terminal length discarded a complete canonical response", err)
	}
	response.Text = `{"title":"unfinished`
	_, err = ParseWriteAnswer(response.Text, 4, nil)
	err = responseParseError(response, err)
	var truncated *llm.TruncatedError
	if !errors.Is(err, llm.ErrOutputTruncated) || !errors.As(err, &truncated) || truncated.ReasoningTokens != 7 || truncated.CompletionTokens != 30 || response.Usage.PromptTokens != 12 {
		t.Fatalf("canonical truncation/usage policy changed: %v", err)
	}
}

func TestResponseDecoderBoundsAndFrozenLegacySchemaBytes(t *testing.T) {
	if _, err := ParseContent(strings.Repeat(" ", answerDecodeMaxBytes)+"{}", 4); !errors.Is(err, llm.ErrBadOutput) {
		t.Fatal("oversized canonical response was decoded")
	}
	var keys []string
	for i := 0; i < answerDecodeMaxMembers; i++ {
		keys = append(keys, `"unknown`+strings.Repeat("x", i)+`":0`)
	}
	if _, err := ParseContent("{"+originDecoderCore+","+strings.Join(keys, ",")+"}", 4); !errors.Is(err, llm.ErrBadOutput) {
		t.Fatal("unbounded outer members were accepted")
	}
	legacy := append(append(append(LegacyWriteAnswerSchema(), LegacyObservationsSchema()...), LegacyVideoObservationsSchema()...), LegacyStorylineAnswerSchema()...)
	sum := sha256.Sum256(legacy)
	const admittedLegacyHash = "3eede03e87a719d9176013ad0b3614ad3b2bd8afb45b9ccaa0340028400806e4"
	if hex.EncodeToString(sum[:]) != admittedLegacyHash {
		t.Fatal("pre-origin schema bytes no longer match admitted paid snapshot hash")
	}
	for _, schemas := range [][2][]byte{{PostContentSchema(), LegacyPostContentSchema()}, {WriteAnswerSchema(), LegacyWriteAnswerSchema()}, {WriteAlongStorylineAnswerSchema(), LegacyWriteAlongStorylineAnswerSchema()}, {StorylineAnswerSchema(), LegacyStorylineAnswerSchema()}, {ObservationsSchema(), LegacyObservationsSchema()}, {VideoObservationsSchema(), LegacyVideoObservationsSchema()}} {
		var current, old map[string]json.RawMessage
		if json.Unmarshal(schemas[0], &current) != nil || json.Unmarshal(schemas[1], &old) != nil {
			t.Fatal("invalid embedded schema JSON")
		}
		var properties map[string]json.RawMessage
		var required []string
		if json.Unmarshal(current["properties"], &properties) != nil || json.Unmarshal(current["required"], &required) != nil || properties["origins"] == nil {
			t.Fatal("origin-aware schema omitted optional final annotations")
		}
		for _, key := range required {
			if key == "origins" {
				t.Fatal("optional origin metadata became canonical required output")
			}
		}
		if bytes.Contains(properties["origins"], []byte(`"start"`)) || bytes.Contains(properties["origins"], []byte(`"end"`)) {
			t.Fatal("schema asks the model for Unicode offsets")
		}
	}
}
