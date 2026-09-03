package llm_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

// fakeProvider records calls so a test can prove a capability refusal never reached it.
type fakeProvider struct {
	name  string
	calls int
	last  llm.Request
	ctx   context.Context
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	f.calls++
	f.last = req
	f.ctx = ctx
	return llm.Response{Text: "ok"}, nil
}

// fakeSource stands in for the curated catalog the registry now reads.
type fakeSource struct{ models []llm.SourceModel }

func (f fakeSource) Models() []llm.SourceModel { return f.models }

func (f fakeSource) Lookup(modelID string) (llm.SourceModel, bool) {
	for _, m := range f.models {
		if m.ModelID == modelID {
			return m, true
		}
	}
	return llm.SourceModel{}, false
}

// twoModels mirrors the shape the catalog holds: one vision + structured-output model on a
// paid floor, and one plain text model on free.
func twoModels() fakeSource {
	return fakeSource{models: []llm.SourceModel{
		{ModelID: "vision-json", Label: "Vision JSON", Vision: true, StructuredOutput: true},
		{ModelID: "text-only", Label: "text-only"},
	}}
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
`

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

// The shipped file now declares the CONNECTION and the recommendation sets; its models
// live in catalog_models, so loading it must not depend on any catalog being present.
func TestLoad_ShippedConnectionAndRecommendation(t *testing.T) {
	provider := &fakeProvider{}
	registry, err := llm.Load("../../config/providers.yaml", env(map[string]string{"OPENROUTER_API_KEY": "test"}), map[string]llm.AdapterFactory{
		"openai_compatible": func(cfg llm.AdapterConfig) (llm.Provider, error) {
			provider.name = cfg.ProviderID
			return provider, nil
		},
	}, fakeSource{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if registry.ProviderID() != "openrouter" {
		t.Errorf("provider id = %q", registry.ProviderID())
	}
	if registry.BaseURL() != "https://openrouter.ai/api/v1" {
		t.Errorf("base url = %q", registry.BaseURL())
	}
	sets := registry.RecommendationSets()
	if len(sets) != 1 || sets[0].ID != "balanced-2026-08" || len(sets[0].Selections) != 3 {
		t.Fatalf("sets = %+v", sets)
	}
}

// A9: a fresh install has curated nothing. That is a working process with an empty
// dropdown, not a refused boot.
func TestParse_EmptyCatalogIsValid(t *testing.T) {
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), fakeSource{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if models := reg.Models(); len(models) != 0 {
		t.Fatalf("models = %+v, want none", models)
	}
	if _, ok := reg.Lookup(llm.ModelRef{ProviderID: "openrouter", ModelID: "anything"}); ok {
		t.Error("an empty catalog resolved a model")
	}
}

func TestParse_ServesTheCuratedCatalog(t *testing.T) {
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), twoModels(), opts)
	if err != nil {
		t.Fatal(err)
	}
	models := reg.Models()
	if len(models) != 2 {
		t.Fatalf("models = %d, want 2", len(models))
	}
	if models[0].Ref.ProviderID != "openrouter" {
		t.Errorf("ref = %+v, want the registered provider attached", models[0].Ref)
	}
	if models[0].Label != "Vision JSON" || !models[0].Vision || !models[0].StructuredOutput {
		t.Errorf("first model = %+v", models[0])
	}
	if models[1].Vision || models[1].StructuredOutput || models[1].Disabled {
		t.Errorf("second model = %+v", models[1])
	}
	// The registry only carries the declared floor; comparing it to an account is the
	// caller's job.
}

// A ref naming another provider belongs to nobody the registry can serve.
func TestLookup_RefusesAForeignProviderID(t *testing.T) {
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), twoModels(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup(llm.ModelRef{ProviderID: "elsewhere", ModelID: "text-only"}); ok {
		t.Error("a foreign provider id resolved")
	}
}

// A6: a model the provider stopped listing is served disabled with its own reason, and
// refused before any call — but the row is still there, so the user learns why.
func TestParse_DelistedModelIsDisabledWithReason(t *testing.T) {
	source := fakeSource{models: []llm.SourceModel{
		{ModelID: "retired", Label: "Retired", Delisted: true},
	}}
	p := &fakeProvider{}
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(p), source, opts)
	if err != nil {
		t.Fatal(err)
	}
	models := reg.Models()
	if len(models) != 1 || !models[0].Disabled || models[0].DisabledReason != llm.DisabledReasonDelisted {
		t.Fatalf("models = %+v", models)
	}
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "retired"}
	if _, err := reg.Complete(context.Background(), ref, llm.Request{}); !errors.Is(err, llm.ErrModelUnavailable) {
		t.Errorf("Complete on a delisted model = %v, want ErrModelUnavailable", err)
	}
	if p.calls != 0 {
		t.Fatalf("provider was called %d times for a delisted model", p.calls)
	}
}

// AC2: an unset key disables every model with the exact reason. It outranks a delisting,
// which is the narrower problem.
func TestParse_MissingKeyDisablesProviderWithReason(t *testing.T) {
	reg, err := llm.Parse([]byte(goodYAML), env(nil), adaptersWith(&fakeProvider{}), twoModels(), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range reg.Models() {
		if !m.Disabled || m.DisabledReason != llm.DisabledReasonNoKey {
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
	reg, err := llm.Parse([]byte(yaml), env(nil), adaptersWith(&fakeProvider{}), twoModels(), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range reg.Models() {
		if m.Disabled {
			t.Errorf("%s disabled without an api_key_env: %q", m.Ref, m.DisabledReason)
		}
	}
}

// AC1 / A9: a broken file is refused with a clear error — including a leftover models list,
// which a stack mounting an old override would otherwise serve silently.
func TestParse_RejectsBrokenConfigs(t *testing.T) {
	cases := map[string]struct {
		yaml string
		want string
	}{
		"unknown adapter": {
			yaml: strings.Replace(goodYAML, "adapter: fake", "adapter: nope", 1),
			want: `unknown adapter "nope"`,
		},
		"a second provider": {
			yaml: goodYAML + strings.Replace(strings.Replace(goodYAML, "providers:\n", "", 1), "id: openrouter", "id: other", 1),
			want: "exactly one provider",
		},
		"leftover models list": {
			yaml: goodYAML + "    models:\n      - id: text-only\n        min_plan: free\n",
			want: "invalid yaml",
		},
		"adapter validation (missing base_url)": {
			yaml: strings.Replace(goodYAML, "    base_url: https://example.test/v1\n", "", 1),
			want: "base_url is required",
		},
		"typo in a field name": {
			yaml: strings.Replace(goodYAML, "adapter: fake", "adaptor: fake", 1),
			want: "invalid yaml",
		},
		"no providers": {
			yaml: "providers: []\n",
			want: "exactly one provider",
		},
		"not yaml": {
			yaml: "providers: [\n",
			want: "invalid yaml",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := llm.Parse([]byte(tc.yaml), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), twoModels(), opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// A10: a recommendation set is SHAPE-checked at boot only. Whether its models exist is
// curated data that changes while the process runs, so it is settled where the set is
// applied — a set naming an unknown model must not stop the API from starting.
func TestParse_RecommendationShapeOnly(t *testing.T) {
	withSet := goodYAML + `
recommendation_sets:
  - id: set
    label: Set
    selections:
      - stage: observe
        active: {provider_id: openrouter, model_id: nobody-has-this}
        candidate_a: {provider_id: openrouter, model_id: nobody-has-this}
        candidate_b: {provider_id: openrouter, model_id: nor-this}
      - stage: analyze
        active: {provider_id: openrouter, model_id: text-only}
        candidate_a: {provider_id: openrouter, model_id: text-only}
        candidate_b: {provider_id: openrouter, model_id: vision-json}
      - stage: write
        active: {provider_id: openrouter, model_id: text-only}
        candidate_a: {provider_id: openrouter, model_id: text-only}
        candidate_b: {provider_id: openrouter, model_id: vision-json}
`
	if _, err := llm.Parse([]byte(withSet), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), twoModels(), opts); err != nil {
		t.Fatalf("an unresolvable ref stopped boot: %v", err)
	}

	for name, broken := range map[string]string{
		"duplicate pair": strings.Replace(withSet, "model_id: nor-this}", "model_id: nobody-has-this}", 1),
		"missing stage":  strings.Replace(withSet, "      - stage: write\n", "", 1),
		"empty ref":      strings.Replace(withSet, "active: {provider_id: openrouter, model_id: nobody-has-this}", "active: {provider_id: openrouter, model_id: \"\"}", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := llm.Parse([]byte(broken), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(&fakeProvider{}), twoModels(), opts); err == nil {
				t.Fatal("a malformed recommendation set was accepted")
			}
		})
	}
}

// The missing-key case must not hide a broken entry: validation runs either way.
func TestParse_ValidatesEntryEvenWithoutKey(t *testing.T) {
	yaml := strings.Replace(goodYAML, "    base_url: https://example.test/v1\n", "", 1)
	_, err := llm.Parse([]byte(yaml), env(nil), adaptersWith(&fakeProvider{}), twoModels(), opts)
	if err == nil || !strings.Contains(err.Error(), "base_url is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	_, err := llm.Load("/nonexistent/providers.yaml", env(nil), adaptersWith(&fakeProvider{}), twoModels(), opts)
	if err == nil || !strings.Contains(err.Error(), "/nonexistent/providers.yaml") {
		t.Fatalf("err = %v, want the path named", err)
	}
}

// AC9: capability refusals happen before any network call, from the curated flags.
func TestResolve_RefusesUnsupportedBeforeCalling(t *testing.T) {
	p := &fakeProvider{}
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(p), twoModels(), opts)
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
	reg, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(p), twoModels(), opts)
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

// A11: the curated per-model override wins over the stage value, `unset` still means "send
// nothing", and an empty override defers to the stage.
func TestComplete_CuratedReasoningOverrideWins(t *testing.T) {
	for _, tc := range []struct {
		name     string
		override llm.ReasoningEffort
		want     llm.ReasoningEffort
	}{
		{name: "no override defers to the stage", override: llm.ReasoningUnspecified, want: llm.ReasoningLow},
		{name: "unset beats stage low", override: llm.ReasoningUnset, want: llm.ReasoningUnset},
		{name: "high beats stage low", override: llm.ReasoningHigh, want: llm.ReasoningHigh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fakeSource{models: []llm.SourceModel{
				{ModelID: "text-only", Reasoning: tc.override},
			}}
			provider := &fakeProvider{}
			registry, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(provider), source, opts)
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
}

func TestComplete_RejectsInvalidCallerReasoning(t *testing.T) {
	provider := &fakeProvider{}
	registry, err := llm.Parse([]byte(goodYAML), env(map[string]string{"TEST_KEY": "k"}), adaptersWith(provider), twoModels(), opts)
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
