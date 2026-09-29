package llm

import (
	"encoding/json"
	"math/big"
)

type FreePath string

const (
	FreeText        FreePath = "text"
	FreeImageInput  FreePath = "image_input"
	FreeVideoInput  FreePath = "video_input"
	FreeImageOutput FreePath = "image_output"
	FreeVideoOutput FreePath = "video_output"
)

// FreePriceProfile accepts only known, exact-zero rates for every price unit
// this request path can incur. A missing optional unit is zero under the
// provider's documented pricing envelope; missing prompt/completion is unsafe.
// Conditional overrides are checked alongside the base profile.
func FreePriceProfile(pricing map[string]json.RawMessage, path FreePath) bool {
	if len(pricing) == 0 {
		return false
	}
	applicable := map[string]bool{
		"prompt": true, "completion": true, "request": true,
		"internal_reasoning": true, "input_cache_read": true,
		"input_cache_write": true, "input_cache_write_1h": true,
	}
	switch path {
	case FreeText:
	case FreeImageInput:
		applicable["image"], applicable["image_token"] = true, true
	case FreeVideoInput:
		for _, key := range []string{"image", "image_token", "audio", "input_audio_cache", "audio_output"} {
			applicable[key] = true
		}
	case FreeImageOutput:
		for _, key := range []string{"image", "image_token", "image_output"} {
			applicable[key] = true
		}
	case FreeVideoOutput:
		for _, key := range []string{"image", "image_token", "image_output", "audio", "audio_output"} {
			applicable[key] = true
		}
	default:
		return false
	}
	var inspect func(map[string]json.RawMessage, bool) bool
	inspect = func(values map[string]json.RawMessage, override bool) bool {
		for key, raw := range values {
			switch key {
			case "overrides":
				if override {
					return false
				}
				var variants []map[string]json.RawMessage
				if json.Unmarshal(raw, &variants) != nil || len(variants) > 128 {
					return false
				}
				for _, variant := range variants {
					if !inspect(variant, true) {
						return false
					}
				}
			case "min_prompt_tokens", "utc_start", "utc_end", "utc_days":
				if !override {
					return false
				}
				// Conditions select a rate, but none can make a nonzero rate free.
			case "discount":
				if _, ok := freeNumber(raw); !ok {
					return false
				}
			case "prompt", "completion", "request", "image", "image_token", "image_output", "audio", "audio_output", "input_audio_cache", "input_cache_read", "input_cache_write", "input_cache_write_1h", "internal_reasoning", "web_search":
				rate, ok := freeNumber(raw)
				if !ok || applicable[key] && rate.Sign() != 0 {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	if _, ok := pricing["prompt"]; !ok {
		return false
	}
	if _, ok := pricing["completion"]; !ok {
		return false
	}
	return inspect(pricing, false)
}

func freeNumber(raw json.RawMessage) (*big.Rat, bool) {
	value := string(raw)
	if len(value) > 0 && value[0] == '"' && json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	if !ValidUnitPrice(value) {
		return nil, false
	}
	rate, ok := new(big.Rat).SetString(value)
	return rate, ok
}
