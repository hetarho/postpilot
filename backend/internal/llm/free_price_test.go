package llm

import (
	"encoding/json"
	"testing"
)

func TestFreePriceProfileChecksApplicableConditionalAndMediaCharges(t *testing.T) {
	base := map[string]json.RawMessage{"prompt": json.RawMessage(`"0"`), "completion": json.RawMessage(`"0"`)}
	if !FreePriceProfile(base, FreeText) {
		t.Fatal("known zero text prices refused")
	}
	for name, change := range map[string]json.RawMessage{
		"request":     json.RawMessage(`"0.0001"`),
		"reasoning":   json.RawMessage(`"0.1"`),
		"conditional": json.RawMessage(`[{"min_prompt_tokens":1000,"completion":"0.2"}]`),
		"unknown":     json.RawMessage(`"0"`),
	} {
		pricing := map[string]json.RawMessage{}
		for key, value := range base {
			pricing[key] = value
		}
		key := name
		if name == "reasoning" {
			key = "internal_reasoning"
		}
		if name == "conditional" {
			key = "overrides"
		}
		pricing[key] = change
		if FreePriceProfile(pricing, FreeText) {
			t.Errorf("%s charge accepted", name)
		}
	}
	withImage := map[string]json.RawMessage{"prompt": json.RawMessage(`"0"`), "completion": json.RawMessage(`"0"`), "image": json.RawMessage(`"0.01"`)}
	if !FreePriceProfile(withImage, FreeText) || FreePriceProfile(withImage, FreeImageInput) {
		t.Fatal("image charge was not path-specific")
	}
}
