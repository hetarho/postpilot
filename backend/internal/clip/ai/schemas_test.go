package ai

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
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
				for _, key := range []string{"type", "required", "enum", "additionalProperties"} {
					if !reflect.DeepEqual(contract[key], output[key]) {
						t.Fatalf("changed structural keyword %s", key)
					}
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
	plan, _ := BuildPlanPrompt(clip.PlanningInput{}, 200)
	if !strings.Contains(observe, string(chunkSchema)) || !strings.Contains(plan, string(planSchema)) {
		t.Fatal("provider projection weakened the full prompt contracts")
	}
	if string(ChunkSchema()) == string(chunkSchema) || string(PlanSchema()) == string(planSchema) {
		t.Fatal("provider output schema was not projected")
	}
	native, _ := BuildPlanPrompt(clip.PlanningInput{Composition: &clip.ProjectComposition{}}, 200)
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
