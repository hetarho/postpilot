package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// Sanitized endpoint SHAPES (testdata/README.md): the ids name which shape each
// fixture proves and are not an allowlist; nothing here asserts that a route or
// a price exists live.
func fixtureEndpoints(t *testing.T, name string) (string, []pricedEndpoint) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Data struct {
			ID        string           `json:"id"`
			Endpoints []pricedEndpoint `json:"endpoints"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Data.ID, fixture.Data.Endpoints
}

func observeCall(model string, structured bool) llm.CallPolicy {
	return llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "test", ModelID: model}, Stage: "observe", CompletionTokens: 8192, Reasoning: llm.ReasoningLow, StructuredOutput: structured, InputUSDPerMillion: "1", OutputUSDPerMillion: "1"}
}

// Four metadata shapes, one contract: explicit per-unit media prices, prompt/
// completion-only pricing (media billed as prompt tokens), conditional tiers,
// and a route without schema parameters that only the parser fallback may use.
func TestQualificationAdmitsEveryDocumentedShapeWithoutAFamilyList(t *testing.T) {
	for name, tc := range map[string]struct {
		fixture              string
		structured           bool
		leaf, image, audio   string
		input, output        string
		fallbackOnly, static bool
	}{
		"google explicit media":  {"gemini_priced_endpoints.json", true, "google-ai-studio/flex", "0.00000015", "0.0000005", "0.5", "1.25", false, true},
		"qwen prompt only":       {"qwen_priced_endpoints.json", true, "alibaba", "0", "0", "0.2", "1.2", false, false},
		"bytedance conditional":  {"bytedance_priced_endpoints.json", true, "bytedance", "0.0000005", "0", "0.5", "4", false, false},
		"amazon parser fallback": {"amazon_priced_endpoints.json", false, "amazon-bedrock", "0", "0", "0.3", "2.5", true, false},
	} {
		t.Run(name, func(t *testing.T) {
			model, endpoints := fixtureEndpoints(t, tc.fixture)
			if tc.fallbackOnly {
				// The schema request needs response_format/structured_outputs; this
				// route lists neither, so that request is refused by parameters…
				if _, err := selectEndpoint(endpoints, observeCall(model, true), llm.ExecutionInlineStatic); !errors.Is(err, llm.ErrRequiredParametersUnsupported) {
					t.Fatalf("schema request on a schema-less route: %v", err)
				}
			}
			// …and the request the model actually gets (its capability decides the
			// shape) qualifies.
			p, err := selectEndpoint(endpoints, observeCall(model, tc.structured), llm.ExecutionInlineStatic)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if p.Pricing.Endpoint != tc.leaf || p.Pricing.ImageUSD != tc.image || p.Pricing.AudioUSDPerToken != tc.audio || p.InputUSDPerMillion != tc.input || p.OutputUSDPerMillion != tc.output || !p.Pricing.Valid() {
				t.Fatalf("frozen = %+v", p)
			}
			want := strings.Join(p.RequiredParameters(), ",")
			if p.Pricing.RequiredParameters != want || strings.Contains(want, "structured_outputs") == !tc.structured {
				t.Fatalf("required parameters %q for structured=%v", p.Pricing.RequiredParameters, tc.structured)
			}
			// The wire: one leaf named twice, fallbacks off, parameters required, and
			// `processing: static` only on the documented profile (CLIP-43).
			c := New(llm.AdapterConfig{ProviderID: "test", BaseURL: "http://stub.test/v1", ReasoningFormat: "openrouter"}, nil)
			req := strictRequest()
			req.Model, req.Execution.Call = model, p
			if !tc.structured {
				req.JSONSchema = nil
			}
			body, _, err := c.strictEnvelope(req, p.Pricing.Endpoint, clipLimits)
			if err != nil {
				t.Fatal(err)
			}
			var wire chatRequest
			if json.Unmarshal(body, &wire) != nil || wire.Provider == nil || wire.Provider.AllowFallbacks || !wire.Provider.RequireParameters || strings.Join(wire.Provider.Only, ",") != tc.leaf || strings.Join(wire.Provider.Order, ",") != tc.leaf {
				t.Fatalf("routing = %+v", wire.Provider)
			}
			if strings.Contains(string(body), `"processing":"static"`) != tc.static || strings.Contains(string(body), `"agentic"`) {
				t.Fatalf("processing control wrong for %s: %s", name, body)
			}
		})
	}
}

// The three stable failures are told apart by the furthest check any leaf
// reached, and a parent tag with variants is not a route at all.
func TestQualificationFailurePrecedenceIsPinned(t *testing.T) {
	model, base := fixtureEndpoints(t, "qwen_priced_endpoints.json")
	leaf := base[0]
	degraded, small := -1, 100
	for name, tc := range map[string]struct {
		endpoints []pricedEndpoint
		want      error
	}{
		"empty document":    {nil, llm.ErrInlineEndpointUnavailable},
		"other model":       {[]pricedEndpoint{func() pricedEndpoint { e := leaf; e.ModelID = "qwen/other"; return e }()}, llm.ErrInlineEndpointUnavailable},
		"degraded status":   {[]pricedEndpoint{func() pricedEndpoint { e := leaf; e.Status = &degraded; return e }()}, llm.ErrInlineEndpointUnavailable},
		"completion cap":    {[]pricedEndpoint{func() pricedEndpoint { e := leaf; e.MaxCompletionTokens = &small; return e }()}, llm.ErrInlineEndpointUnavailable},
		"context too small": {[]pricedEndpoint{func() pricedEndpoint { e := leaf; e.ContextLength = &small; return e }()}, llm.ErrInlineEndpointUnavailable},
		"malformed tag":     {[]pricedEndpoint{func() pricedEndpoint { e := leaf; e.Tag = "Alibaba Cloud"; return e }()}, llm.ErrInlineEndpointUnavailable},
		"ambiguous parent": {[]pricedEndpoint{leaf, func() pricedEndpoint {
			e := leaf
			e.Tag += "/priority"
			e.Pricing = map[string]json.RawMessage{"prompt": json.RawMessage(`"x"`)}
			return e
		}()}, llm.ErrPriceCeilingUnavailable},
		"duplicate tags": {[]pricedEndpoint{leaf, leaf}, llm.ErrInlineEndpointUnavailable},
		"parameters": {[]pricedEndpoint{func() pricedEndpoint {
			e := leaf
			e.SupportedParameters = []string{"max_tokens", "reasoning"}
			return e
		}()}, llm.ErrRequiredParametersUnsupported},
		"no reasoning": {[]pricedEndpoint{func() pricedEndpoint {
			e := leaf
			e.SupportedParameters = []string{"max_tokens", "response_format", "structured_outputs"}
			return e
		}()}, llm.ErrRequiredParametersUnsupported},
		"missing prompt": {[]pricedEndpoint{func() pricedEndpoint {
			e := leaf
			e.Pricing = map[string]json.RawMessage{"completion": json.RawMessage(`"0.000001"`)}
			return e
		}()}, llm.ErrPriceCeilingUnavailable},
		"unknown unit": {[]pricedEndpoint{func() pricedEndpoint {
			e := leaf
			e.Pricing = map[string]json.RawMessage{"prompt": json.RawMessage(`"0.000001"`), "completion": json.RawMessage(`"0.000001"`), "video_second": json.RawMessage(`"0"`)}
			return e
		}()}, llm.ErrPriceCeilingUnavailable},
		"malformed price": {[]pricedEndpoint{func() pricedEndpoint {
			e := leaf
			e.Pricing = map[string]json.RawMessage{"prompt": json.RawMessage(`"free"`), "completion": json.RawMessage(`"0.000001"`)}
			return e
		}()}, llm.ErrPriceCeilingUnavailable},
		"unknown cache write": {[]pricedEndpoint{func() pricedEndpoint {
			e := leaf
			e.Pricing = map[string]json.RawMessage{"prompt": json.RawMessage(`"0.000001"`), "completion": json.RawMessage(`"0.000001"`), "input_cache_write": json.RawMessage(`"0.000002"`)}
			return e
		}()}, llm.ErrPriceCeilingUnavailable},
		// Two leaves: one stops at parameters, one at price — the answer is price.
		"furthest check wins": {[]pricedEndpoint{
			func() pricedEndpoint {
				e := leaf
				e.Tag = "a"
				e.SupportedParameters = []string{"max_tokens"}
				return e
			}(),
			func() pricedEndpoint {
				e := leaf
				e.Tag = "b"
				e.Pricing = map[string]json.RawMessage{"prompt": json.RawMessage(`null`)}
				return e
			}(),
		}, llm.ErrPriceCeilingUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := selectEndpoint(tc.endpoints, observeCall(model, true), llm.ExecutionInlineStatic)
			if !errors.Is(err, tc.want) || !errors.Is(err, llm.ErrUnsupported) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
	// Determinism: the cheapest qualified leaf, and on a tie the first by tag.
	cheap, dear := leaf, leaf
	cheap.Tag, dear.Tag = "zeta", "alpha"
	dear.Pricing = map[string]json.RawMessage{"prompt": json.RawMessage(`"0.000009"`), "completion": json.RawMessage(`"0.000009"`)}
	if p, err := selectEndpoint([]pricedEndpoint{dear, cheap}, observeCall(model, true), llm.ExecutionInlineStatic); err != nil || p.Pricing.Endpoint != "zeta" {
		t.Fatalf("cheapest leaf not chosen: %+v %v", p, err)
	}
	twin := cheap
	twin.Tag = "beta"
	if p, err := selectEndpoint([]pricedEndpoint{cheap, twin}, observeCall(model, true), llm.ExecutionInlineStatic); err != nil || p.Pricing.Endpoint != "beta" {
		t.Fatalf("tie not broken by tag order: %+v %v", p, err)
	}
}

// A document that cannot be read, or that is not a document, proves nothing:
// the model is "endpoint unavailable", the diagnostic keeps the operation and
// status only, and nothing is posted.
func TestUnreadableOrMalformedDocumentIsNotEligible(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"http error": func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) },
		"not json":   func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "<html>") },
		"wrong id": func(w http.ResponseWriter, r *http.Request) {
			endpointResponse(w, "someone/else", strictEndpointFixture("someone/else"))
		},
		"no leaves": func(w http.ResponseWriter, r *http.Request) { endpointResponse(w, "google/gemini-2.5-flash") },
	} {
		t.Run(name, func(t *testing.T) {
			var posts atomic.Int32
			c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					posts.Add(1)
					return
				}
				handler(w, r)
			})
			call := observeCall("google/gemini-2.5-flash", true)
			_, err := c.FreezePricing(context.Background(), call, llm.ExecutionInlineStatic)
			if !errors.Is(err, llm.ErrInlineEndpointUnavailable) || posts.Load() != 0 {
				t.Fatalf("err = %v posts = %d", err, posts.Load())
			}
			if strings.Contains(err.Error(), "html") || strings.Contains(err.Error(), "someone") {
				t.Fatalf("document content leaked: %v", err)
			}
		})
	}
}

// Concurrent readers of one model share a single fetch; the read-only
// eligibility list (AllowCachedEndpoints) is served from an unexpired document
// and reads again once it expires; a failed read is never cached; a quote, an
// admission and the pre-completion recheck always read the live document.
func TestEndpointDocumentCacheTTLConcurrencyAndFailure(t *testing.T) {
	var gets atomic.Int32
	var fail atomic.Bool
	model, endpoints := fixtureEndpoints(t, "qwen_priced_endpoints.json")
	c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}],"usage":{"cost":0.001}}`)
			return
		}
		gets.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		time.Sleep(20 * time.Millisecond)
		endpointResponse(w, model, endpoints...)
	})
	now := time.Unix(1_700_000_000, 0)
	c.endpointDocs.now = func() time.Time { return now }
	call := observeCall(model, true)
	listing := llm.AllowCachedEndpoints(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.FreezePricing(listing, call, llm.ExecutionInlineStatic); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if gets.Load() != 1 {
		t.Fatalf("%d concurrent reads of one model reached the provider", gets.Load())
	}
	// The eligibility list inside the TTL is answered from the document it read.
	if _, err := c.FreezePricing(listing, call, llm.ExecutionInlineStatic); err != nil || gets.Load() != 1 {
		t.Fatalf("listing re-read an unexpired document: err=%v gets=%d", err, gets.Load())
	}
	// A quote reads live, and so does the recheck before its completion.
	frozen, err := c.FreezePricing(context.Background(), call, llm.ExecutionInlineStatic)
	if err != nil || gets.Load() != 2 {
		t.Fatalf("quote served from cache: err=%v gets=%d", err, gets.Load())
	}
	req := strictRequest()
	req.Model, req.Execution.Call = model, frozen
	if _, err := c.Complete(context.Background(), req); err != nil || gets.Load() != 3 {
		t.Fatalf("recheck served from cache: err=%v gets=%d", err, gets.Load())
	}
	// Past the TTL the listing reads again — and a read that fails is not kept,
	// so the next caller reads again too.
	now = now.Add(c.endpointDocs.ttl)
	fail.Store(true)
	if _, err := c.FreezePricing(listing, call, llm.ExecutionInlineStatic); !errors.Is(err, llm.ErrInlineEndpointUnavailable) || gets.Load() != 4 {
		t.Fatalf("expired document served or failure cached: err=%v gets=%d", err, gets.Load())
	}
	fail.Store(false)
	if _, err := c.FreezePricing(listing, call, llm.ExecutionInlineStatic); err != nil || gets.Load() != 5 {
		t.Fatalf("failed read was cached: err=%v gets=%d", err, gets.Load())
	}
	if c.endpointDocs.ttl != defaultEndpointCacheTTL || c.endpointDocs.timeout != defaultEndpointFetchTimout || cap(c.endpointDocs.sem) != endpointFetchConcurrency {
		t.Fatal("cache bounds are not the code-owned defaults")
	}
}

// The fingerprint pins the profile, not the weather: a status change passes the
// recheck, a price change refuses it before any completion, and a leaf that
// disappeared refuses it too — with no other leaf tried.
func TestRecheckRefusesDriftWithoutRetryOrSubstitute(t *testing.T) {
	model, endpoints := fixtureEndpoints(t, "gemini_priced_endpoints.json")
	frozen, err := selectEndpoint(endpoints, observeCall(model, true), llm.ExecutionInlineStatic)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		mutate func([]pricedEndpoint) []pricedEndpoint
		want   error
	}{
		"status only": {func(e []pricedEndpoint) []pricedEndpoint {
			degraded := 0
			e[1].Status = &degraded
			return e
		}, nil},
		"price drift": {func(e []pricedEndpoint) []pricedEndpoint {
			e[1].Pricing["prompt"] = json.RawMessage(`"0.01"`)
			return e
		}, llm.ErrPriceCeilingUnavailable},
		"parameter dropped": {func(e []pricedEndpoint) []pricedEndpoint {
			e[1].SupportedParameters = []string{"max_tokens"}
			return e
		}, llm.ErrRequiredParametersUnsupported},
		"leaf gone, another remains": {func(e []pricedEndpoint) []pricedEndpoint {
			return e[:1]
		}, llm.ErrInlineEndpointUnavailable},
		"leaf became a parent": {func(e []pricedEndpoint) []pricedEndpoint {
			variant := e[1]
			variant.Tag += "/priority"
			return append(e, variant)
		}, llm.ErrInlineEndpointUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			_, fresh := fixtureEndpoints(t, "gemini_priced_endpoints.json")
			current := tc.mutate(fresh)
			var posts atomic.Int32
			c := strictTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					posts.Add(1)
					_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}],"usage":{"cost":0.001}}`)
					return
				}
				endpointResponse(w, model, current...)
			})
			req := strictRequest()
			req.Model, req.Execution.Call = model, frozen
			_, err := c.Complete(context.Background(), req)
			if tc.want == nil {
				if err != nil || posts.Load() != 1 {
					t.Fatalf("status change refused the frozen route: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) || posts.Load() != 0 {
				t.Fatalf("err = %v posts = %d", err, posts.Load())
			}
		})
	}
}
