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
      - id: text-only
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
