package generation

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The durable payload's wire shape, pinned byte for byte.
//
// A generate job queued before a deploy is decoded by the worker after it, so the payload is
// a stored contract rather than an implementation detail: a renamed key, a tag that drifts to
// `omitempty`, or a field the encoder stops writing would strand work already in the queue.
// The envelope structs live beside the domain types they map (Go has no way to put a mapper
// in a package the mapped types import), so this test is what keeps them from becoming one.
func TestGenerationPayloadWireShapeIsPinned(t *testing.T) {
	length := 1200
	observe := []string{"a.jpg"}
	raw, err := EncodeGenerationPayload(GenerationOptions{
		TargetLanguage: LanguageKorean,
		TargetLength:   &length,
		TagCount:       7,
		Template: &TemplateBrief{
			Name: "여행", Body: "# 제목",
			Facts: []TemplateFact{{Label: "장소", Value: "제주"}},
		},
		Guidelines:   []string{"문장은 짧게"},
		Memories:     []string{"매운 음식을 못 먹는다"},
		QualityRules: []string{"제목에 같은 말을 되풀이하지 않는다"},
		FieldPhrases: []string{"분위기 좋은 카페"},
		ObserveFiles: &observe,
		Observations: []Observation{{
			File: "a.jpg", Scene: "바다", Objects: []string{"파도"}, Model: "p/m",
			Events: []string{"파도가 친다"}, Speech: "좋다",
		}},
		WriteNativeEffort: true,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	const want = `{"target_language":"ko","target_length":1200,"tag_count":7,` +
		`"template":{"name":"여행","body":"# 제목","facts":[{"label":"장소","value":"제주"}]},` +
		`"guidelines":["문장은 짧게"],"memories":["매운 음식을 못 먹는다"],` +
		`"quality_rules":["제목에 같은 말을 되풀이하지 않는다"],"field_phrases":["분위기 좋은 카페"],"observe_files":["a.jpg"],` +
		`"observations":[{"file":"a.jpg","scene":"바다","objects":["파도"],"model":"p/m",` +
		`"events":["파도가 친다"],"speech":"좋다"}],"write_native_effort":true}`
	if string(raw) != want {
		t.Fatalf("payload shape drifted:\n got %s\nwant %s", raw, want)
	}

	// And it round trips: what the worker decodes is what enqueue froze.
	back, err := DecodeGenerationPayload(raw)
	if err != nil || back.TagCount != 7 || back.TargetLength == nil || *back.TargetLength != 1200 {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	if back.ObserveFiles == nil || len(*back.ObserveFiles) != 1 || !back.WriteNativeEffort {
		t.Fatalf("round trip lost presence: %+v", back)
	}
	if len(back.Observations) != 1 || back.Observations[0].Speech != "좋다" {
		t.Fatalf("round trip lost an observation: %+v", back.Observations)
	}
	if !reflect.DeepEqual(back.QualityRules, []string{"제목에 같은 말을 되풀이하지 않는다"}) || !reflect.DeepEqual(back.FieldPhrases, []string{"분위기 좋은 카페"}) {
		t.Fatalf("round trip lost the rules or the phrases: %+v %+v", back.QualityRules, back.FieldPhrases)
	}

	// The three states of the re-observation set survive the edge, including the empty one
	// that means "observe nothing" — the silent double-spend this contract prevents.
	none := []string{}
	empty, err := EncodeGenerationPayload(GenerationOptions{
		TargetLanguage: LanguageKorean, TagCount: 4, ObserveFiles: &none,
	})
	if err != nil {
		t.Fatalf("encode empty: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(empty, &keys); err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if string(keys["observe_files"]) != "[]" {
		t.Fatalf("observe_files = %s, want []", keys["observe_files"])
	}
	absent, err := EncodeGenerationPayload(GenerationOptions{TargetLanguage: LanguageKorean})
	if err != nil {
		t.Fatalf("encode absent: %v", err)
	}
	if string(absent) != `{"target_language":"ko","observe_files":null}` {
		t.Fatalf("absent payload = %s", absent)
	}
}
