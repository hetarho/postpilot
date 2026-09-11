package openaicompat

import (
	"encoding/json"
	"math/big"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/llm"
)

func endpointPrice(raw json.RawMessage) (*big.Rat, bool) {
	value := string(raw)
	if strings.HasPrefix(value, `"`) && json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	if !llm.ValidUnitPrice(value) {
		return nil, false
	}
	rate, ok := new(big.Rat).SetString(value)
	return rate, ok
}

func priceWithin(raw json.RawMessage, perMillion string) bool {
	rate, ok := endpointPrice(raw)
	maximum, valid := new(big.Rat).SetString(perMillion)
	return ok && valid && rate.Mul(rate, big.NewRat(1_000_000, 1)).Cmp(maximum) <= 0
}

// Finite decimal inputs only: multiplication by powers of ten stays exact.
func priceDecimal(r *big.Rat) string {
	return strings.TrimRight(strings.TrimRight(r.FloatString(1128), "0"), ".")
}

func wirePrice(value string) string {
	r, _ := new(big.Rat).SetString(value)
	return priceDecimal(r)
}

type priceEnvelope map[string]*big.Rat

func (p priceEnvelope) rate(key string) *big.Rat {
	if r := p[key]; r != nil {
		return new(big.Rat).Set(r)
	}
	return new(big.Rat)
}

func (p priceEnvelope) maximum(keys ...string) *big.Rat {
	out := new(big.Rat)
	for _, key := range keys {
		if r := p[key]; r != nil && r.Cmp(out) > 0 {
			out.Set(r)
		}
	}
	return out
}

// Include every recognized tier, irrespective of when/at what context length it
// applies. This overestimates quotes but cannot miss a later conditional rate.
func readPriceEnvelope(raw map[string]json.RawMessage) (priceEnvelope, bool) {
	p := priceEnvelope{}
	merge := func(values map[string]json.RawMessage, override bool) bool {
		for key, value := range values {
			switch key {
			case "prompt", "completion", "request", "image", "image_token", "audio", "audio_output", "image_output", "input_audio_cache", "input_cache_read", "input_cache_write", "input_cache_write_1h", "internal_reasoning", "web_search", "discount":
				rate, ok := endpointPrice(value)
				if !ok || (key == "discount" && rate.Cmp(big.NewRat(1, 1)) > 0) {
					return false
				}
				if p[key] == nil || rate.Cmp(p[key]) > 0 {
					p[key] = rate
				}
			case "overrides":
				if override {
					return false
				}
			case "min_prompt_tokens", "utc_start", "utc_end":
				rate, ok := endpointPrice(value)
				if !override || !ok || !rate.IsInt() || !rate.Num().IsInt64() {
					return false
				}
				if key != "min_prompt_tokens" && (rate.Num().Int64() > 2359 || rate.Num().Int64()%100 >= 60) {
					return false
				}
			case "utc_days":
				var days []string
				if !override || json.Unmarshal(value, &days) != nil || len(days) == 0 {
					return false
				}
				for _, day := range days {
					if !slices.Contains([]string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}, day) {
						return false
					}
				}
			default:
				return false
			}
		}
		return true
	}
	if !merge(raw, false) || p["prompt"] == nil || p["completion"] == nil {
		return nil, false
	}
	if value, ok := raw["overrides"]; ok {
		var overrides []map[string]json.RawMessage
		if string(value) == "null" || json.Unmarshal(value, &overrides) != nil || len(overrides) > 128 {
			return nil, false
		}
		for _, override := range overrides {
			if override == nil || !merge(override, true) {
				return nil, false
			}
		}
	}
	return p, true
}

// freeze prices the leaf for the request the policy describes. Under the
// OpenRouter pricing contract a pricing key is present when that unit is
// charged separately and absent when it is not, so an inline request on a leaf
// that lists no `image`/`audio` rate is billed as prompt tokens and those
// envelopes are zero — included, not unknown. A key that is present but
// malformed, a dimension this code does not know, or a cache-write semantics
// it cannot place still refuse: an unknown charge is never treated as free.
func (e pricedEndpoint) freeze(call llm.CallPolicy, delivery llm.ExecutionDelivery) (llm.CallPolicy, bool) {
	p, ok := readPriceEnvelope(e.Pricing)
	if !ok {
		return llm.CallPolicy{}, false
	}
	inputKeys := []string{"prompt", "input_cache_read"}
	if delivery == llm.ExecutionInlineStatic {
		inputKeys = append(inputKeys, "audio", "input_audio_cache", "image_token")
	}
	// No cache-control, tools, plugins or output modalities exist in strict requests.
	// Gemini/Anthropic explicit cache writes are inactive. OpenAI/DeepSeek automatic
	// token-priced writes replace ordinary input pricing and must be covered.
	// Unknown write/storage semantics refuse rather than silently ignoring charges.
	if p.maximum("input_cache_write", "input_cache_write_1h").Sign() > 0 {
		switch {
		case strings.HasPrefix(e.ModelID, "google/gemini-"), strings.HasPrefix(e.ModelID, "anthropic/"):
		case strings.HasPrefix(e.ModelID, "openai/"), strings.HasPrefix(e.ModelID, "deepseek/"):
			inputKeys = append(inputKeys, "input_cache_write", "input_cache_write_1h")
		default:
			return llm.CallPolicy{}, false
		}
	}
	in, out := p.maximum(inputKeys...), p.maximum("completion", "internal_reasoning")
	// Aggregate usage cannot distinguish cached tokens or conditional tiers. Keep
	// reported cost, but never turn the conservative bound into measured usage.
	uniform := delivery == llm.ExecutionTextOnly && p.rate("request").Sign() == 0 && p.rate("discount").Sign() == 0 && e.Pricing["overrides"] == nil
	// Absence of a cache-read breakdown/rate does not prove that an automatically
	// caching family charged every reported prompt token at the uncached price.
	if p["input_cache_read"] == nil && (strings.HasPrefix(e.ModelID, "google/gemini-") || strings.HasPrefix(e.ModelID, "openai/") || strings.HasPrefix(e.ModelID, "deepseek/")) {
		uniform = false
	}
	for _, key := range inputKeys {
		if rate := p[key]; rate != nil && rate.Cmp(in) != 0 {
			uniform = false
		}
	}
	for _, key := range []string{"completion", "internal_reasoning"} {
		if rate := p[key]; rate != nil && rate.Cmp(out) != 0 {
			uniform = false
		}
	}
	fingerprint, ok := e.fingerprint()
	if !ok {
		return llm.CallPolicy{}, false
	}
	call.InputUSDPerMillion = priceDecimal(in.Mul(in, big.NewRat(1_000_000, 1)))
	call.OutputUSDPerMillion = priceDecimal(out.Mul(out, big.NewRat(1_000_000, 1)))
	call.Pricing = llm.CallPricing{
		Version: llm.CallPricingVersion, Fingerprint: fingerprint, Delivery: delivery,
		Endpoint: e.Tag, RequiredParameters: strings.Join(call.RequiredParameters(), ","),
		PromptUSDPerMillion:     priceDecimal(p.rate("prompt").Mul(p.rate("prompt"), big.NewRat(1_000_000, 1))),
		CompletionUSDPerMillion: call.OutputUSDPerMillion,
		RequestUSD:              priceDecimal(p.rate("request")), ImageUSD: priceDecimal(p.rate("image")), AudioUSDPerToken: priceDecimal(p.rate("audio")),
		AggregateUsageSufficient: uniform,
	}
	return call, call.Valid()
}
