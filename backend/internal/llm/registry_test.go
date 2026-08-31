package llm_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// fakeProvider records calls so a test can prove a capability refusal never reached it.
type fakeProvider struct {
	name  string
	calls int
	last  llm.Request
	ctx   context.Context
}

func TestLoad_CurrentShippedCatalogAndRecommendation(t *testing.T) {
	provider := &fakeProvider{}
	registry, err := llm.Load("../../config/providers.yaml", env(map[string]string{"OPENROUTER_API_KEY": "test"}), map[string]llm.AdapterFactory{
		"openai_compatible": func(cfg llm.AdapterConfig) (llm.Provider, error) {
			provider.name = cfg.ProviderID
			return provider, nil
		},
	}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Models()) != 12 {
		t.Fatalf("models = %d, want the free router entry plus the eleven pinned models", len(registry.Models()))
	}
	sets := registry.RecommendationSets()
	if len(sets) != 1 || sets[0].ID != "balanced-2026-08" || len(sets[0].Selections) != 3 {
		t.Fatalf("sets = %+v", sets)
	}
	sonnet := llm.ModelRef{ProviderID: "openrouter", ModelID: "anthropic/claude-sonnet-5"}
	if _, err := registry.Complete(context.Background(), sonnet, llm.Request{Reasoning: llm.ReasoningLow}); err != nil {
		t.Fatal(err)
	}
	if provider.last.Reasoning != llm.ReasoningUnset {
		t.Fatalf("shipped Sonnet reasoning = %q, want unset override", provider.last.Reasoning)
	}
}

func TestParse_RejectsBrokenPriceAndRecommendationMetadata(t *testing.T) {
	base := `
providers:
  - id: p
    adapter: fake
    base_url: https://example.test
    models:
      - id: vision-a
        vision: true
        min_plan: free
        context_tokens: 1000
        input_usd_per_million: "1"
        output_usd_per_million: "2"
        pricing_checked_at: "2026-08-29"
      - id: vision-b
        vision: true
        min_plan: free
        context_tokens: 1000
        input_usd_per_million: "1"
        output_usd_per_million: "2"
        pricing_checked_at: "2026-08-29"
      - id: text
        min_plan: free
        context_tokens: 1000
        input_usd_per_million: "1"
        output_usd_per_million: "2"
        pricing_checked_at: "2026-08-29"
recommendation_sets:
  - id: set
    label: Set
    selections:
      - stage: observe
        active: {provider_id: p, model_id: vision-a}
        candidate_a: {provider_id: p, model_id: vision-a}
        candidate_b: {provider_id: p, model_id: vision-b}
      - stage: analyze
        active: {provider_id: p, model_id: text}
        candidate_a: {provider_id: p, model_id: text}
        candidate_b: {provider_id: p, model_id: vision-b}
      - stage: write
        active: {provider_id: p, model_id: text}
        candidate_a: {provider_id: p, model_id: text}
        candidate_b: {provider_id: p, model_id: vision-b}
`
	cases := map[string]string{
		"negative price": strings.Replace(base, `input_usd_per_million: "1"`, `input_usd_per_million: "-1"`, 1),
		"bad date":       strings.Replace(base, `pricing_checked_at: "2026-08-29"`, `pricing_checked_at: "August"`, 1),
		"missing ref":    strings.Replace(base, `model_id: vision-b}`, `model_id: gone}`, 1),
		"duplicate pair": strings.Replace(base, `candidate_b: {provider_id: p, model_id: vision-b}`, `candidate_b: {provider_id: p, model_id: vision-a}`, 1),
		"observe text":   strings.Replace(base, `active: {provider_id: p, model_id: vision-a}`, `active: {provider_id: p, model_id: text}`, 1),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := llm.Parse([]byte(content), env(nil), adaptersWith(&fakeProvider{}), opts); err == nil {
				t.Fatal("broken catalog was accepted")
			}
		})
	}
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	f.calls++
	f.last = req
	f.ctx = ctx
	return llm.Response{Text: "ok"}, nil
}

func adaptersWith(p *fakeProvider) map[string]llm.AdapterFactory {
	return map[string]llm.AdapterFactory{
		"fake": func(cfg llm.AdapterConfig) (llm.Provider, error) {
			if cfg.BaseURL == "" {
				return nil, errors.New("fake: base_url is required")
			}
			p.name = cfg.ProviderID
			return p, nil
		},
	}
}

var opts = llm.Options{Timeout: time.Minute, MaxTokens: 1024}

const goodYAML = `
providers:
  - id: openrouter
    adapter: fake
    base_url: https://example.test/v1
    api_key_env: TEST_KEY
    models:
      - id: vision-json
        label: Vision JSON
        vision: true
        structured_output: true
        min_plan: basic
      - id: text-only
        min_plan: free
`

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestParse_LoadsModelsInOrderWithDefaults(t *testing.T) {
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), opts)
	if err != nil {
		t.Fatal(err)
	}
	models := reg.Models()
	if len(models) != 2 {
		t.Fatalf("models = %d, want 2", len(models))
	}
	if models[0].Label != "Vision JSON" || !models[0].Vision || !models[0].StructuredOutput {
		t.Errorf("first model = %+v", models[0])
	}
	// The label defaults to the id; flags default to false.
	if models[1].Label != "text-only" || models[1].Vision || models[1].StructuredOutput || models[1].Disabled {
		t.Errorf("second model = %+v", models[1])
	}
}

// AC2: an unset key disables every model of the provider with the exact reason.
func TestParse_MissingKeyDisablesProviderWithReason(t *testing.T) {
	reg, err := llm.Parse([]byte(goodYAML), env(nil), adaptersWith(&fakeProvider{}), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range reg.Models() {
		if !m.Disabled || m.DisabledReason != "API key not configured" {
			t.Errorf("%s: disabled=%v reason=%q", m.Ref, m.Disabled, m.DisabledReason)
		}
	}
	_, err = reg.Complete(context.Background(), llm.ModelRef{ProviderID: "openrouter", ModelID: "text-only"}, llm.Request{})
	if !errors.Is(err, llm.ErrProviderDisabled) {
		t.Errorf("Complete on disabled = %v, want ErrProviderDisabled", err)
	}
}

// A provider with no api_key_env takes no key (a local Ollama): enabled as is.
func TestParse_KeylessProviderIsEnabled(t *testing.T) {
	yaml := strings.Replace(goodYAML, "    api_key_env: TEST_KEY\n", "", 1)
	reg, err := llm.Parse([]byte(yaml), env(nil), adaptersWith(&fakeProvider{}), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range reg.Models() {
		if m.Disabled {
			t.Errorf("%s disabled without an api_key_env: %q", m.Ref, m.DisabledReason)
		}
	}
}

// AC1: a broken file is refused with a clear error.
func TestParse_RejectsBrokenConfigs(t *testing.T) {
	cases := map[string]struct {
		yaml string
		want string
	}{
		"unknown adapter": {
			yaml: strings.Replace(goodYAML, "adapter: fake", "adapter: nope", 1),
			want: `unknown adapter "nope"`,
		},
		"duplicate provider id": {
			yaml: goodYAML + strings.Replace(goodYAML, "providers:\n", "", 1),
			want: "duplicate id",
		},
		"duplicate model id": {
			yaml: strings.Replace(goodYAML, "      - id: text-only", "      - id: vision-json", 1),
			want: `duplicate model id "vision-json"`,
		},
		"adapter validation (missing base_url)": {
			yaml: strings.Replace(goodYAML, "    base_url: https://example.test/v1\n", "", 1),
			want: "base_url is required",
		},
		"typo in a field name": {
			yaml: strings.Replace(goodYAML, "vision: true", "vison: true", 1),
			want: "invalid yaml",
		},
		"no providers": {
			yaml: "providers: []\n",
			want: "no providers",
		},
		"not yaml": {
			yaml: "providers: [\n",
			want: "invalid yaml",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := llm.Parse([]byte(tc.yaml), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// The missing-key case must not hide a broken entry: validation runs either way.
func TestParse_ValidatesEntryEvenWithoutKey(t *testing.T) {
	yaml := strings.Replace(goodYAML, "    base_url: https://example.test/v1\n", "", 1)
	_, err := llm.Parse([]byte(yaml), env(nil), adaptersWith(&fakeProvider{}), opts)
	if err == nil || !strings.Contains(err.Error(), "base_url is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	_, err := llm.Load("/nonexistent/providers.yaml", env(nil), adaptersWith(&fakeProvider{}), opts)
	if err == nil || !strings.Contains(err.Error(), "/nonexistent/providers.yaml") {
		t.Fatalf("err = %v, want the path named", err)
	}
}

// AC9: capability refusals happen before any network call.
func TestResolve_RefusesUnsupportedBeforeCalling(t *testing.T) {
	p := &fakeProvider{}
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(p), opts)
	if err != nil {
		t.Fatal(err)
	}
	textOnly := llm.ModelRef{ProviderID: "openrouter", ModelID: "text-only"}

	withImage := llm.Request{Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.ImagePart([]byte{1}, "image/jpeg")}}}}
	if _, err := reg.Complete(context.Background(), textOnly, withImage); !errors.Is(err, llm.ErrUnsupported) {
		t.Errorf("image on text-only = %v, want ErrUnsupported", err)
	}
	withSchema := llm.Request{JSONSchema: []byte(`{"type":"object"}`)}
	if _, err := reg.Complete(context.Background(), textOnly, withSchema); !errors.Is(err, llm.ErrUnsupported) {
		t.Errorf("schema on text-only = %v, want ErrUnsupported", err)
	}
	if _, err := reg.Complete(context.Background(), llm.ModelRef{ProviderID: "openrouter", ModelID: "ghost"}, llm.Request{}); !errors.Is(err, llm.ErrModelUnavailable) {
		t.Errorf("unknown model = %v, want ErrModelUnavailable", err)
	}
	if p.calls != 0 {
		t.Fatalf("provider was called %d times for refused requests", p.calls)
	}
}

func TestComplete_FillsModelDefaultCapAndTimeout(t *testing.T) {
	p := &fakeProvider{}
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(p), opts)
	if err != nil {
		t.Fatal(err)
	}
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vision-json"}
	req := llm.Request{
		JSONSchema: []byte(`{}`),
		Messages:   []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{llm.ImagePart([]byte{1}, "image/jpeg")}}},
	}
	if _, err := reg.Complete(context.Background(), ref, req); err != nil {
		t.Fatal(err)
	}
	if p.last.Model != "vision-json" {
		t.Errorf("Model = %q, want the ref's model id", p.last.Model)
	}
	if p.last.MaxTokens != opts.MaxTokens {
		t.Errorf("MaxTokens = %d, want the default %d", p.last.MaxTokens, opts.MaxTokens)
	}
	if deadline, ok := p.ctx.Deadline(); !ok || time.Until(deadline) > opts.Timeout {
		t.Errorf("call ran without the stage timeout (deadline %v, ok=%v)", deadline, ok)
	}

	if _, err := reg.Complete(context.Background(), ref, llm.Request{MaxTokens: 7}); err != nil {
		t.Fatal(err)
	}
	if p.last.MaxTokens != 7 {
		t.Errorf("a caller's own cap was overwritten: %d", p.last.MaxTokens)
	}
	if _, err := reg.Complete(context.Background(), ref, llm.Request{MaxTokens: -3}); err != nil {
		t.Fatal(err)
	}
	if p.last.MaxTokens != opts.MaxTokens {
		t.Errorf("a negative cap went out as %d, want the default", p.last.MaxTokens)
	}
}

func TestParse_ModelReasoningOverrideWinsAndInvalidValueFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		want llm.ReasoningEffort
	}{
		{
			name: "unset beats stage low",
			yaml: strings.Replace(goodYAML, "      - id: text-only", "      - id: text-only\n        reasoning_effort: unset", 1),
			want: llm.ReasoningUnset,
		},
		{
			name: "high beats stage low",
			yaml: strings.Replace(goodYAML, "      - id: text-only", "      - id: text-only\n        reasoning_effort: high", 1),
			want: llm.ReasoningHigh,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &fakeProvider{}
			registry, err := llm.Parse([]byte(tc.yaml), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(provider), opts)
			if err != nil {
				t.Fatal(err)
			}
			ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "text-only"}
			if _, err := registry.Complete(context.Background(), ref, llm.Request{Reasoning: llm.ReasoningLow}); err != nil {
				t.Fatal(err)
			}
			if provider.last.Reasoning != tc.want {
				t.Fatalf("Reasoning = %q, want %q", provider.last.Reasoning, tc.want)
			}
		})
	}

	invalid := strings.Replace(goodYAML, "      - id: text-only", "      - id: text-only\n        reasoning_effort: enormous", 1)
	_, err := llm.Parse([]byte(invalid), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), opts)
	if err == nil || !strings.Contains(err.Error(), `reasoning_effort "enormous" is invalid`) {
		t.Fatalf("invalid reasoning_effort err = %v", err)
	}
}

func TestComplete_RejectsInvalidCallerReasoning(t *testing.T) {
	provider := &fakeProvider{}
	registry, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(provider), opts)
	if err != nil {
		t.Fatal(err)
	}
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "text-only"}
	if _, err := registry.Complete(context.Background(), ref, llm.Request{Reasoning: "enormous"}); err == nil {
		t.Fatal("invalid caller reasoning reached the provider")
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

// A11: min_plan is required on every model and validated against the ladder. A registry that
// does not validate must not serve — the same posture as a bad migration.
func TestParse_RequiresAValidMinPlanOnEveryModel(t *testing.T) {
	for name, replacement := range map[string]string{
		"missing": "",
		"unknown": "        min_plan: pro\n",
		"empty":   "        min_plan:\n",
	} {
		t.Run(name, func(t *testing.T) {
			broken := strings.Replace(goodYAML, "        min_plan: basic\n", replacement, 1)
			if _, err := llm.Parse([]byte(broken), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), opts); err == nil {
				t.Fatal("a model without a usable min_plan must fail boot")
			} else if !strings.Contains(err.Error(), "min_plan") {
				t.Errorf("err = %v, want it to name min_plan", err)
			}
		})
	}
}

// The registry only carries the declared floor; comparing it to an account is the caller's job.
func TestParse_CarriesTheDeclaredFloor(t *testing.T) {
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), opts)
	if err != nil {
		t.Fatal(err)
	}
	models := reg.Models()
	if got, want := models[0].MinPlan, plan.Basic; got != want {
		t.Errorf("first model min_plan = %q, want %q", got, want)
	}
	if got, want := models[1].MinPlan, plan.Free; got != want {
		t.Errorf("second model min_plan = %q, want %q", got, want)
	}
}
