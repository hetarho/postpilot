package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func geminiPriceFixture(t *testing.T) ([]byte, []pricedEndpoint) {
	t.Helper()
	raw, err := os.ReadFile("testdata/gemini_priced_endpoints.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Data struct {
			Endpoints []pricedEndpoint `json:"endpoints"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return raw, fixture.Data.Endpoints
}

func TestNonzeroMediaQuoteAndExecutionUseOneFrozenProfile(t *testing.T) {
	metadata, _ := geminiPriceFixture(t)
	var gets, posts atomic.Int32
	req := strictRequest()
	c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			gets.Add(1)
			_, _ = w.Write(metadata)
			return
		}
		posts.Add(1)
		raw, _ := io.ReadAll(r.Body)
		var wire chatRequest
		if json.Unmarshal(raw, &wire) != nil || wire.Provider == nil {
			t.Error("missing strict routing")
			return
		}
		p := wire.Provider
		if strings.Join(p.Order, ",") != "google-ai-studio/flex" || strings.Join(p.Only, ",") != "google-ai-studio/flex" || p.AllowFallbacks || !p.RequireParameters || p.MaxPrice != (strictPrices{Prompt: "0.15", Completion: "1.25", Request: "0", Image: "0.00000015", Audio: "0.0000005"}) {
			t.Errorf("wrong route/prices: %+v", p)
		}
		for _, forbidden := range []string{`"tools"`, `"plugins"`, `"models"`, `"cache_control"`, `"modalities"`} {
			if strings.Contains(string(raw), forbidden) {
				t.Errorf("optional billing enabled: %s", forbidden)
			}
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}],"usage":{"cost":0.004}}`)
	})
	p, err := c.FreezePricing(context.Background(), req.Execution.Call, req.Execution.Delivery)
	if err != nil || p.InputUSDPerMillion != "0.5" || p.OutputUSDPerMillion != "1.25" || p.Pricing.AggregateUsageSufficient {
		t.Fatalf("quote=%+v err=%v", p, err)
	}
	// 30k mixed input tokens at max(prompt,audio,image-token/cache); 8192 output
	// tokens including reasoning; 60 separately priced frames; no optional search.
	if cost, ok := p.QuoteMicrousd(); !ok || cost != 25249 {
		t.Fatalf("quote cost=%d valid=%v", cost, ok)
	}
	if gets.Load() != 1 || posts.Load() != 0 {
		t.Fatal("quoting issued a completion")
	}
	req.Execution.Call = p
	out, err := c.Complete(context.Background(), req)
	if err != nil || out.Usage.CostMicrousd != 4000 || gets.Load() != 2 || posts.Load() != 1 {
		t.Fatalf("out=%+v err=%v gets=%d posts=%d", out, err, gets.Load(), posts.Load())
	}
}

func TestFrozenProfileDriftRefusesBeforeCompletion(t *testing.T) {
	for _, dimension := range []string{"prompt", "completion", "image", "audio", "input_cache_read", "internal_reasoning"} {
		t.Run(dimension, func(t *testing.T) {
			_, endpoints := geminiPriceFixture(t)
			e := endpoints[0]
			req := strictRequest()
			req.Execution.Call, _ = e.freeze(req.Execution.Call, req.Execution.Delivery)
			e.Pricing[dimension] = json.RawMessage(`"0.01"`)
			var posts atomic.Int32
			c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					posts.Add(1)
				}
				endpointResponse(w, req.Model, e)
			})
			if _, err := c.Complete(context.Background(), req); !errors.Is(err, llm.ErrUnsupported) || posts.Load() != 0 {
				t.Fatalf("err=%v posts=%d", err, posts.Load())
			}
		})
	}
}

func TestConditionalRatesAndCacheApplicability(t *testing.T) {
	_, endpoints := geminiPriceFixture(t)
	e := endpoints[0]
	call := strictRequest().Execution.Call
	e.Pricing["overrides"] = json.RawMessage(`[{"min_prompt_tokens":200000,"prompt":"0.000004","completion":"0.000008"},{"utc_start":100,"utc_end":400,"utc_days":["monday"],"audio":"0.000006"}]`)
	p, ok := e.freeze(call, llm.ExecutionInlineStatic)
	if !ok || p.InputUSDPerMillion != "6" || p.OutputUSDPerMillion != "8" || p.Pricing.PromptUSDPerMillion != "4" || p.Pricing.AudioUSDPerToken != "0.000006" {
		t.Fatalf("uncovered conditional price: %+v valid=%v", p, ok)
	}
	delete(e.Pricing, "overrides")
	text, ok := e.freeze(call, llm.ExecutionTextOnly)
	if !ok || text.InputUSDPerMillion != "0.3" || text.Pricing.AggregateUsageSufficient {
		t.Fatal("implicit cache discount cannot use aggregate estimate", text, ok)
	}
	for _, model := range []string{"openai/gpt-5.6", "deepseek/deepseek-v3"} {
		candidate := strictEndpointFixture(model)
		candidate.Pricing["input_cache_write"] = json.RawMessage(`"0.000004"`)
		call.Ref.ModelID = model
		frozen, valid := candidate.freeze(call, llm.ExecutionTextOnly)
		if !valid || frozen.InputUSDPerMillion != "4" || frozen.Pricing.AggregateUsageSufficient {
			t.Fatalf("automatic writes not covered: %+v", frozen)
		}
	}
}

func TestUnknownOrMissingApplicablePriceRefusesQuote(t *testing.T) {
	for name, mutate := range map[string]func(*pricedEndpoint){
		"missing audio":  func(e *pricedEndpoint) { delete(e.Pricing, "audio") },
		"missing visual": func(e *pricedEndpoint) { delete(e.Pricing, "image") },
		"unknown":        func(e *pricedEndpoint) { e.Pricing["video_second"] = json.RawMessage(`"0"`) },
		"unknown condition": func(e *pricedEndpoint) {
			e.Pricing["overrides"] = json.RawMessage(`[{"new_condition":1,"prompt":"0"}]`)
		},
		"null overrides": func(e *pricedEndpoint) { e.Pricing["overrides"] = json.RawMessage(`null`) },
		"bad time":       func(e *pricedEndpoint) { e.Pricing["overrides"] = json.RawMessage(`[{"utc_start":1260}]`) },
		"bad discount":   func(e *pricedEndpoint) { e.Pricing["discount"] = json.RawMessage(`1.01`) },
	} {
		t.Run(name, func(t *testing.T) {
			_, endpoints := geminiPriceFixture(t)
			e := endpoints[0]
			mutate(&e)
			if _, ok := e.freeze(strictRequest().Execution.Call, llm.ExecutionInlineStatic); ok {
				t.Fatal("unsafe price admitted")
			}
		})
	}
}

func TestParentAndDuplicateTagsAreNotExactRoutes(t *testing.T) {
	e := strictEndpointFixture("google/gemini-2.5-flash")
	variant := e
	variant.Tag += "/priority"
	if uniqueLeaf(e, []pricedEndpoint{e, variant}) || uniqueLeaf(e, []pricedEndpoint{e, e}) || !uniqueLeaf(variant, []pricedEndpoint{e, variant}) {
		t.Fatal("ambiguous endpoint selection")
	}
}
