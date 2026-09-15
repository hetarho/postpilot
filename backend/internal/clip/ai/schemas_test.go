package ai

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestOutputSchemasRetainClosedShapeWithoutGrammarBounds(t *testing.T) {
	for _, fixture := range []struct {
		name             string
		contract, output []byte
	}{{"observe", chunkSchema, ChunkSchema()}, {"plan", planSchema, PlanSchema()}, {"composition", compositionPlanSchema, CompositionPlanSchema()}} {
		t.Run(fixture.name, func(t *testing.T) {
			var full, wire map[string]any
			if json.Unmarshal(fixture.contract, &full) != nil || json.Unmarshal(fixture.output, &wire) != nil {
				t.Fatal("invalid schema")
			}
			removed := 0
			var compare func(map[string]any, map[string]any)
			compare = func(contract, output map[string]any) {
				for _, key := range []string{"type", "required", "additionalProperties"} {
					if !reflect.DeepEqual(contract[key], output[key]) {
						t.Fatalf("changed structural keyword %s", key)
					}
				}
				// A string enum is the one bound the provider grammar can carry.
				want := contract["enum"]
				if contract["type"] != "string" {
					want = nil
				}
				if !reflect.DeepEqual(want, output["enum"]) {
					t.Fatalf("changed enum on a %v", contract["type"])
				}
				for _, key := range []string{"minimum", "maximum", "minItems", "maxItems", "minLength", "maxLength", "default"} {
					if _, exists := output[key]; exists {
						t.Fatalf("provider grammar retains %s", key)
					}
					if _, exists := contract[key]; exists {
						removed++
					}
				}
				if properties, ok := contract["properties"].(map[string]any); ok {
					projected, ok := output["properties"].(map[string]any)
					if !ok || len(projected) != len(properties) {
						t.Fatal("lost properties")
					}
					for key, value := range properties {
						child, ok := projected[key].(map[string]any)
						if !ok {
							t.Fatalf("lost property %s", key)
						}
						compare(value.(map[string]any), child)
					}
				}
				if items, ok := contract["items"].(map[string]any); ok {
					projected, ok := output["items"].(map[string]any)
					if !ok {
						t.Fatal("lost array item schema")
					}
					compare(items, projected)
				}
			}
			compare(full, wire)
			if removed == 0 {
				t.Fatal("fixture does not exercise domain bounds")
			}
		})
	}
}

// CLIP-98's rates reach the model as prompt text and are enforced by the plan
// validator. Sent as a provider enum they take Gemini's whole cut object with
// them: it answers {} for every cut and the run dies at output_shape.
func TestOutputProjectionSendsNoNonStringEnum(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		output []byte
	}{{"observe", ChunkSchema()}, {"plan", PlanSchema()}, {"composition", CompositionPlanSchema()}} {
		t.Run(fixture.name, func(t *testing.T) {
			var wire map[string]any
			if json.Unmarshal(fixture.output, &wire) != nil {
				t.Fatal("invalid schema")
			}
			var walk func(string, map[string]any)
			walk = func(path string, node map[string]any) {
				if _, carries := node["enum"]; carries && node["type"] != "string" {
					t.Fatalf("%s sends a %v enum to the provider", path, node["type"])
				}
				properties, _ := node["properties"].(map[string]any)
				for key, value := range properties {
					if child, ok := value.(map[string]any); ok {
						walk(path+"/"+key, child)
					}
				}
				if items, ok := node["items"].(map[string]any); ok {
					walk(path+"[]", items)
				}
			}
			walk("", wire)
		})
	}
}

func TestOutputProjectionKeepsConstraintNamedProperties(t *testing.T) {
	contract := []byte(`{"type":"object","additionalProperties":false,"required":["minimum","maxItems"],"properties":{"minimum":{"type":"integer","minimum":0},"maxItems":{"type":"string","maxLength":20}}}`)
	var result map[string]any
	if json.Unmarshal(structuralSchema(contract), &result) != nil {
		t.Fatal("invalid schema")
	}
	properties := result["properties"].(map[string]any)
	if len(properties) != 2 || properties["minimum"] == nil || properties["maxItems"] == nil {
		t.Fatal("stripped property names as if they were schema keywords")
	}
}

func TestPromptsKeepFullContractsAfterOutputProjection(t *testing.T) {
	observe, _ := BuildObservePrompt(clip.ChunkInput{})
	plan, _ := BuildPlanPrompt(clip.PlanningInput{}, 200, composition.Limits{})
	if !strings.Contains(observe, compactContract(chunkSchema)) || !strings.Contains(plan, string(planSchema)) {
		t.Fatal("provider projection weakened the full prompt contracts")
	}
	if string(ChunkSchema()) == string(chunkSchema) || string(PlanSchema()) == string(planSchema) {
		t.Fatal("provider output schema was not projected")
	}
	native, _ := BuildPlanPrompt(clip.PlanningInput{Composition: &clip.ProjectComposition{}}, 200, composition.Limits{})
	if !strings.Contains(native, compactContract(compositionPlanSchema)) || string(CompositionPlanSchema()) == string(compositionPlanSchema) {
		t.Fatal("native contract was weakened")
	}
}

// Both writer contracts offer exactly CLIP-98's rate set, so a prompt can never
// name a rate the parser would refuse and no rate the product supports can be
// missing from the grammar the model answers in.
func TestWriterSchemasOfferExactlyTheSupportedRates(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		contract []byte
	}{{"plan", planSchema}, {"composition", compositionPlanSchema}} {
		t.Run(fixture.name, func(t *testing.T) {
			var document map[string]any
			if err := json.Unmarshal(fixture.contract, &document); err != nil {
				t.Fatal(err)
			}
			cut := document["properties"].(map[string]any)["cuts"].(map[string]any)["items"].(map[string]any)
			rate, declared := cut["properties"].(map[string]any)["rate_permille"].(map[string]any)
			if !declared {
				t.Fatal("the contract states no rate")
			}
			required := false
			for _, key := range cut["required"].([]any) {
				required = required || key == "rate_permille"
			}
			if !required {
				t.Fatal("a cut may omit its rate")
			}
			want := clip.PlaybackRates()
			values := rate["enum"].([]any)
			if len(values) != len(want) {
				t.Fatalf("the contract offers %v, the product supports %v", values, want)
			}
			for i, value := range values {
				if int(value.(float64)) != want[i] {
					t.Fatalf("the contract offers %v, the product supports %v", values, want)
				}
			}
		})
	}
}

func TestCaptionSafeIsOptionalBoundedFactualSpace(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(chunkSchema, &schema); err != nil {
		t.Fatal(err)
	}
	segment := schema["properties"].(map[string]any)["segments"].(map[string]any)["items"].(map[string]any)
	for _, key := range segment["required"].([]any) {
		if key == "caption_safe" {
			t.Fatal("legacy observations invalidated")
		}
	}
	field := segment["properties"].(map[string]any)["caption_safe"].(map[string]any)
	if field["type"] != "array" || field["maxItems"] != float64(4) || strings.Count(string(chunkSchema), `"caption_safe"`) != 1 {
		t.Fatal("missing or duplicate contract")
	}
	for _, text := range []string{"Make NO editing decision", "no principal subject", "no readable footage text", "never an anchor, style, placement or exposure choice"} {
		if !strings.Contains(observePrompt, text) {
			t.Fatalf("lost observation boundary: %s", text)
		}
	}
	if clip.AnalysisContractVersion != "clip-observation-v2" {
		t.Fatal("additive evidence invalidated stored observations")
	}
}
