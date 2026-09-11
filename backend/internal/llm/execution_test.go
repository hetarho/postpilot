package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestPartsHaveExactlyOneVariantAndInlineReadersCannotBeSerialized(t *testing.T) {
	video := llm.InlineVideo{MIME: "video/mp4", Size: 3, DurationMS: 1000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("abc")), nil }}
	for name, part := range map[string]llm.Part{"text": llm.TextPart("hello"), "empty text": llm.TextPart(""), "image": llm.ImagePart([]byte{1}, "image/jpeg"), "url": llm.VideoPart("https://object.test/a.mp4", "video/mp4"), "inline": llm.InlineVideoPart(video)} {
		t.Run(name, func(t *testing.T) {
			if !part.Valid() {
				t.Fatal("valid variant refused")
			}
		})
	}
	for name, part := range map[string]llm.Part{"empty": {}, "empty image": llm.ImagePart([]byte{}, "image/jpeg"), "text and url": {Text: "x", VideoURL: "https://object.test"}, "image and inline": {Image: []byte{1}, InlineVideo: &video}, "url and inline": {VideoURL: "https://object.test", InlineVideo: &video}} {
		t.Run(name, func(t *testing.T) {
			if part.Valid() {
				t.Fatal("invalid variant accepted")
			}
		})
	}
	r := llm.Request{Messages: []llm.Message{{Parts: []llm.Part{llm.InlineVideoPart(video)}}}}
	if !r.HasVideos() || r.HasImages() || r.ValidateParts() == nil {
		t.Fatal("inline capability or missing policy validation")
	}
	if _, err := json.Marshal(r); err == nil {
		t.Fatal("runtime reader entered a durable payload")
	}
}

func TestRegistryStrictCallPreservesFrozenReasoningAndRejectsPolicyChanges(t *testing.T) {
	source := twoModels()
	source.models[0].InputUSDPerMillion = "0.1"
	source.models[0].OutputUSDPerMillion = "0.7"
	source.models[0].Reasoning = map[string]llm.ReasoningEffort{"write": llm.ReasoningLow}
	provider := &fakeProvider{}
	registry, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(provider), source, opts)
	if err != nil {
		t.Fatal(err)
	}
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}
	frozen, err := registry.FreezeCall(ref, "write", 80, llm.ReasoningHigh)
	if err != nil {
		t.Fatal(err)
	}
	source.models[0].Reasoning["write"] = llm.ReasoningMax
	frozen.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionTextOnly, Endpoint: "leaf", RequiredParameters: "max_tokens,reasoning", PromptUSDPerMillion: "0.1", CompletionUSDPerMillion: "0.7", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0", AggregateUsageSufficient: true}
	req := llm.Request{Stage: "write", MaxTokens: 80, Reasoning: llm.ReasoningLow, JSONSchema: []byte(`{"type":"object"}`), Execution: &llm.ExecutionPolicy{Call: frozen, Delivery: llm.ExecutionTextOnly, NoFallback: true, RequireParameters: true}}
	if _, err := registry.Complete(context.Background(), ref, req); err != nil {
		t.Fatal(err)
	}
	if provider.last.Reasoning != llm.ReasoningLow {
		t.Fatal("catalog update overwrote approved reasoning")
	}
	for name, mutate := range map[string]func(*llm.Request){
		"budget":           func(r *llm.Request) { r.MaxTokens++ },
		"reasoning":        func(r *llm.Request) { r.Reasoning = llm.ReasoningMax },
		"stage":            func(r *llm.Request) { r.Stage = "observe" },
		"disable":          func(r *llm.Request) { r.DisableReasoning = true },
		"fallback":         func(r *llm.Request) { r.Execution.NoFallback = false },
		"missing price":    func(r *llm.Request) { r.Execution.Call.InputUSDPerMillion = "" },
		"legacy price":     func(r *llm.Request) { r.Execution.Call.Pricing = llm.CallPricing{} },
		"changed delivery": func(r *llm.Request) { r.Execution.Call.Pricing.Delivery = llm.ExecutionInlineStatic },
	} {
		t.Run(name, func(t *testing.T) {
			copy := req
			policy := *req.Execution
			copy.Execution = &policy
			mutate(&copy)
			if _, err := registry.Complete(context.Background(), ref, copy); !errors.Is(err, llm.ErrUnsupported) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if provider.calls != 1 {
		t.Fatalf("calls = %d", provider.calls)
	}
}

type pricedTestProvider struct {
	fakeProvider
	ready  bool
	quotes int
}

func (p *pricedTestProvider) VideoDelivery(string) llm.VideoDelivery {
	return llm.VideoDelivery{InlineStaticVideo: p.ready}
}
func (p *pricedTestProvider) FreezePricing(_ context.Context, call llm.CallPolicy, delivery llm.ExecutionDelivery) (llm.CallPolicy, error) {
	p.quotes++
	call.InputUSDPerMillion, call.OutputUSDPerMillion = "1", "2"
	call.Pricing = llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: delivery, Endpoint: "leaf", RequiredParameters: "max_tokens,reasoning", PromptUSDPerMillion: "1", CompletionUSDPerMillion: "2", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}
	return call, nil
}

func TestRegistryQuotesOnlyVideoInputModelsAndKeepsFrozenPolicy(t *testing.T) {
	source := twoModels()
	p := &pricedTestProvider{}
	r, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "test"}), map[string]llm.AdapterFactory{"fake": func(llm.AdapterConfig) (llm.Provider, error) { return p, nil }}, source, opts)
	if err != nil {
		t.Fatal(err)
	}
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}
	// No video input in the catalog: the one admission the registry decides, and
	// it costs no endpoint read.
	if _, err = r.FreezeExecution(context.Background(), ref, "observe", 8192, llm.ReasoningLow, llm.ExecutionInlineStatic); !errors.Is(err, llm.ErrVideoInputAbsent) || !errors.Is(err, llm.ErrUnsupported) || p.quotes != 0 {
		t.Fatal("raw modality absent yet the adapter was consulted", err)
	}
	// With video input the adapter's document decides — a Google-family flag is
	// no longer consulted, so `ready` false must not refuse (CLIP-30).
	source.models[0].VideoInput = true
	frozen, err := r.FreezeExecution(context.Background(), ref, "observe", 8192, llm.ReasoningLow, llm.ExecutionInlineStatic)
	if err != nil || !frozen.Valid() || !frozen.StructuredOutput || p.calls != 0 || p.quotes != 1 {
		t.Fatalf("freeze=%+v err=%v", frozen, err)
	}
	req := llm.Request{Stage: "observe", MaxTokens: 8192, Reasoning: llm.ReasoningLow, JSONSchema: []byte(`{"type":"object"}`), Execution: &llm.ExecutionPolicy{Call: frozen, Delivery: llm.ExecutionInlineStatic, NoFallback: true, RequireParameters: true}, Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.InlineVideoPart(llm.InlineVideo{MIME: "video/mp4", Size: 3, DurationMS: 1000, Sampling: llm.VideoSamplingFixed, Open: func(context.Context) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("abc")), nil }})}}}}
	if _, err = r.Complete(context.Background(), ref, req); err != nil || p.calls != 1 {
		t.Fatal(err, p.calls)
	}
	// The frozen policy says a schema goes; a request without one is a different
	// request and is refused before the provider sees it.
	plain := req
	plain.JSONSchema = nil
	if _, err = r.Complete(context.Background(), ref, plain); !errors.Is(err, llm.ErrUnsupported) || p.calls != 1 {
		t.Fatal("schema presence drifted from the frozen policy", err, p.calls)
	}
	// A catalog that stops advertising video refuses the call it admitted.
	source.models[0].VideoInput = false
	if _, err = r.Complete(context.Background(), ref, req); !errors.Is(err, llm.ErrUnsupported) || p.calls != 1 {
		t.Fatal("removed modality dispatched", err, p.calls)
	}
	info, _ := r.Lookup(ref)
	if info.VideoInput || info.VideoDelivery.InlineStaticVideo {
		t.Fatal("catalog capability overwritten")
	}
}
