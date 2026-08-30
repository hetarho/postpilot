package llm

import (
	"context"
	"fmt"
	"maps"
	"math/big"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// DisabledReasonNoKey is the reason shown for every model of a provider whose
// `api_key_env` is unset. The frontend displays it as is.
const DisabledReasonNoKey = "API key not configured"

// ModelRef names one model of one provider. It is what a job records and what the
// dropdowns save.
type ModelRef struct {
	ProviderID string
	ModelID    string
}

func (r ModelRef) String() string { return r.ProviderID + "/" + r.ModelID }

// ModelInfo is a registry entry as the catalog exposes it — ids, label and flags only.
type ModelInfo struct {
	Ref                 ModelRef
	Label               string
	Vision              bool
	StructuredOutput    bool
	ContextTokens       int64
	InputUSDPerMillion  string
	OutputUSDPerMillion string
	PricingCheckedAt    string
	Disabled            bool
	DisabledReason      string
}

// RecommendationSelection is the registry-owned, versioned selection for one stage.
// Stage remains its stable string form so the llm boundary does not import provider.
type RecommendationSelection struct {
	Stage      string
	Active     ModelRef
	CandidateA ModelRef
	CandidateB ModelRef
}

// RecommendationSet is display/config metadata. Applying one is provider-context
// behavior because it owns model_selections and the acting account.
type RecommendationSet struct {
	ID         string
	Label      string
	Selections []RecommendationSelection
}

// Options tune every call the registry dispatches.
type Options struct {
	// Timeout bounds one provider call (PRD §6.6: 단계당 5분).
	Timeout time.Duration
	// MaxTokens is the completion cap when a request sets none.
	MaxTokens int
}

// The yaml shape. Field names are the contract documented in providers.yaml; unknown
// fields are an error so a typo (`vison: true`) cannot silently disable a capability.
type registryFile struct {
	Providers          []providerEntry          `yaml:"providers"`
	RecommendationSets []recommendationSetEntry `yaml:"recommendation_sets"`
}

type providerEntry struct {
	ID              string       `yaml:"id"`
	Adapter         string       `yaml:"adapter"`
	BaseURL         string       `yaml:"base_url"`
	APIKeyEnv       string       `yaml:"api_key_env"`
	ReasoningFormat string       `yaml:"reasoning_format"`
	Models          []modelEntry `yaml:"models"`
}

type modelEntry struct {
	ID                  string          `yaml:"id"`
	Label               string          `yaml:"label"`
	Vision              bool            `yaml:"vision"`
	StructuredOutput    bool            `yaml:"structured_output"`
	ReasoningEffort     ReasoningEffort `yaml:"reasoning_effort"`
	ContextTokens       int64           `yaml:"context_tokens"`
	InputUSDPerMillion  string          `yaml:"input_usd_per_million"`
	OutputUSDPerMillion string          `yaml:"output_usd_per_million"`
	PricingCheckedAt    string          `yaml:"pricing_checked_at"`
}

type recommendationSetEntry struct {
	ID         string                         `yaml:"id"`
	Label      string                         `yaml:"label"`
	Selections []recommendationSelectionEntry `yaml:"selections"`
}

type recommendationSelectionEntry struct {
	Stage      string        `yaml:"stage"`
	Active     modelRefEntry `yaml:"active"`
	CandidateA modelRefEntry `yaml:"candidate_a"`
	CandidateB modelRefEntry `yaml:"candidate_b"`
}

type modelRefEntry struct {
	ProviderID string `yaml:"provider_id"`
	ModelID    string `yaml:"model_id"`
}

type entry struct {
	info      ModelInfo
	provider  Provider
	reasoning ReasoningEffort
}

// Registry is the loaded providers.yaml: every model with its flags, and the provider
// that serves it. Read-only after Load, so it is safe to share across goroutines.
type Registry struct {
	entries map[ModelRef]*entry
	// order is the yaml order — the order the dropdowns show.
	order           []ModelRef
	recommendations []RecommendationSet
	opts            Options
}

// Load reads and validates the registry file. Any problem is returned as an error the
// composition root turns into a refused boot: a broken registry must be loud, like a
// broken migration.
//
// `getenv` resolves each provider's `api_key_env`; `adapters` maps adapter names to the
// factories the composition root chose to ship (the only place that imports them).
func Load(path string, getenv func(string) string, adapters map[string]AdapterFactory, opts Options) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read providers config %s: %w", path, err)
	}
	reg, err := Parse(data, getenv, adapters, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return reg, nil
}

// Parse is Load without the file — the testable core.
func Parse(data []byte, getenv func(string) string, adapters map[string]AdapterFactory, opts Options) (*Registry, error) {
	if opts.Timeout <= 0 {
		return nil, fmt.Errorf("registry options: timeout must be positive")
	}
	if opts.MaxTokens <= 0 {
		return nil, fmt.Errorf("registry options: max tokens must be positive")
	}

	var file registryFile
	if err := yaml.UnmarshalWithOptions(data, &file, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("invalid yaml: %s", yaml.FormatError(err, false, true))
	}
	if len(file.Providers) == 0 {
		return nil, fmt.Errorf("no providers declared")
	}

	reg := &Registry{entries: map[ModelRef]*entry{}, opts: opts}
	seenProviders := map[string]bool{}
	for i, p := range file.Providers {
		where := fmt.Sprintf("providers[%d]", i)
		if p.ID != "" {
			where = fmt.Sprintf("provider %q", p.ID)
		}
		if strings.TrimSpace(p.ID) == "" {
			return nil, fmt.Errorf("%s: id is required", where)
		}
		if seenProviders[p.ID] {
			return nil, fmt.Errorf("%s: duplicate id", where)
		}
		seenProviders[p.ID] = true

		factory, ok := adapters[p.Adapter]
		if !ok {
			return nil, fmt.Errorf("%s: unknown adapter %q (known: %s)", where, p.Adapter, strings.Join(slices.Sorted(maps.Keys(adapters)), ", "))
		}
		if len(p.Models) == 0 {
			return nil, fmt.Errorf("%s: at least one model is required", where)
		}

		// No `api_key_env` means the endpoint takes no key (a local Ollama, vLLM, LM
		// Studio); with one, the key is read from the environment and its absence is what
		// disables the provider — never a boot failure.
		key := ""
		if env := strings.TrimSpace(p.APIKeyEnv); env != "" {
			key = getenv(env)
		}
		// Built even without a key so the entry is validated either way; a bad base_url
		// must not hide behind a missing key until the day the key is set.
		provider, err := factory(AdapterConfig{
			ProviderID: p.ID, BaseURL: p.BaseURL, APIKey: key, ReasoningFormat: p.ReasoningFormat,
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		disabled := p.APIKeyEnv != "" && key == ""
		reason := ""
		if disabled {
			reason = DisabledReasonNoKey
		}

		seenModels := map[string]bool{}
		for j, m := range p.Models {
			if strings.TrimSpace(m.ID) == "" {
				return nil, fmt.Errorf("%s: models[%d]: id is required", where, j)
			}
			if seenModels[m.ID] {
				return nil, fmt.Errorf("%s: duplicate model id %q", where, m.ID)
			}
			seenModels[m.ID] = true
			label := m.Label
			if label == "" {
				label = m.ID
			}
			ref := ModelRef{ProviderID: p.ID, ModelID: m.ID}
			if err := validateModelMetadata(m); err != nil {
				return nil, fmt.Errorf("%s: model %q: %w", where, m.ID, err)
			}
			if !m.ReasoningEffort.Valid() {
				return nil, fmt.Errorf("%s: model %q: reasoning_effort %q is invalid (want unset, none, minimal, low, medium, high, xhigh, or max)", where, m.ID, m.ReasoningEffort)
			}
			reg.entries[ref] = &entry{
				info: ModelInfo{
					Ref:                 ref,
					Label:               label,
					Vision:              m.Vision,
					StructuredOutput:    m.StructuredOutput,
					ContextTokens:       m.ContextTokens,
					InputUSDPerMillion:  m.InputUSDPerMillion,
					OutputUSDPerMillion: m.OutputUSDPerMillion,
					PricingCheckedAt:    m.PricingCheckedAt,
					Disabled:            disabled,
					DisabledReason:      reason,
				},
				provider:  provider,
				reasoning: m.ReasoningEffort,
			}
			reg.order = append(reg.order, ref)
		}
	}
	if err := reg.loadRecommendations(file.RecommendationSets); err != nil {
		return nil, err
	}
	return reg, nil
}

func validateModelMetadata(m modelEntry) error {
	values := []string{m.InputUSDPerMillion, m.OutputUSDPerMillion, m.PricingCheckedAt}
	provided := m.ContextTokens != 0 || slices.ContainsFunc(values, func(value string) bool { return value != "" })
	if !provided {
		return nil
	}
	if m.ContextTokens <= 0 {
		return fmt.Errorf("context_tokens must be positive when pricing metadata is supplied")
	}
	for name, value := range map[string]string{
		"input_usd_per_million":  m.InputUSDPerMillion,
		"output_usd_per_million": m.OutputUSDPerMillion,
	} {
		parsed, ok := new(big.Rat).SetString(value)
		if !ok || parsed.Sign() < 0 {
			return fmt.Errorf("%s must be a non-negative decimal", name)
		}
	}
	if _, err := time.Parse(time.DateOnly, m.PricingCheckedAt); err != nil {
		return fmt.Errorf("pricing_checked_at must be YYYY-MM-DD")
	}
	return nil
}

func (r *Registry) loadRecommendations(entries []recommendationSetEntry) error {
	seenSets := map[string]bool{}
	for i, item := range entries {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Label) == "" {
			return fmt.Errorf("recommendation_sets[%d]: id and label are required", i)
		}
		if seenSets[item.ID] {
			return fmt.Errorf("recommendation set %q: duplicate id", item.ID)
		}
		seenSets[item.ID] = true
		if len(item.Selections) != 3 {
			return fmt.Errorf("recommendation set %q: exactly three stage selections are required", item.ID)
		}
		set := RecommendationSet{ID: item.ID, Label: item.Label}
		seenStages := map[string]bool{}
		for _, selection := range item.Selections {
			if selection.Stage != "observe" && selection.Stage != "write" && selection.Stage != "analyze" {
				return fmt.Errorf("recommendation set %q: unknown stage %q", item.ID, selection.Stage)
			}
			if seenStages[selection.Stage] {
				return fmt.Errorf("recommendation set %q: duplicate stage %q", item.ID, selection.Stage)
			}
			seenStages[selection.Stage] = true
			converted := RecommendationSelection{
				Stage:      selection.Stage,
				Active:     toModelRef(selection.Active),
				CandidateA: toModelRef(selection.CandidateA),
				CandidateB: toModelRef(selection.CandidateB),
			}
			if converted.CandidateA == converted.CandidateB {
				return fmt.Errorf("recommendation set %q stage %q: candidates must differ", item.ID, selection.Stage)
			}
			for _, ref := range []ModelRef{converted.Active, converted.CandidateA, converted.CandidateB} {
				info, ok := r.Lookup(ref)
				if !ok {
					return fmt.Errorf("recommendation set %q stage %q: model %s is not registered", item.ID, selection.Stage, ref)
				}
				if selection.Stage == "observe" && !info.Vision {
					return fmt.Errorf("recommendation set %q stage observe: model %s has no vision", item.ID, ref)
				}
			}
			set.Selections = append(set.Selections, converted)
		}
		r.recommendations = append(r.recommendations, set)
	}
	return nil
}

func toModelRef(ref modelRefEntry) ModelRef {
	return ModelRef{ProviderID: strings.TrimSpace(ref.ProviderID), ModelID: strings.TrimSpace(ref.ModelID)}
}

// Models is the catalog in yaml order.
func (r *Registry) Models() []ModelInfo {
	out := make([]ModelInfo, 0, len(r.order))
	for _, ref := range r.order {
		out = append(out, r.entries[ref].info)
	}
	return out
}

// RecommendationSets returns defensive copies in yaml order.
func (r *Registry) RecommendationSets() []RecommendationSet {
	out := make([]RecommendationSet, len(r.recommendations))
	for i, set := range r.recommendations {
		out[i] = set
		out[i].Selections = append([]RecommendationSelection(nil), set.Selections...)
	}
	return out
}

// Lookup returns the entry for a ref, if registered.
func (r *Registry) Lookup(ref ModelRef) (ModelInfo, bool) {
	e, ok := r.entries[ref]
	if !ok {
		return ModelInfo{}, false
	}
	return e.info, true
}

// resolve finds the provider for a ref and checks that the model can take what the
// request asks of it — before any network call, so a caller learns "this model cannot
// see images" in microseconds rather than after a round trip.
//
// Unexported on purpose: handing the Provider out would let a caller skip the timeout
// and the defaults that Complete applies. Callers that need the flags use Lookup.
func (r *Registry) resolve(ref ModelRef, req Request) (*entry, error) {
	e, ok := r.entries[ref]
	if !ok {
		return nil, fmt.Errorf("%w: %s is not registered", ErrModelUnavailable, ref)
	}
	if e.info.Disabled {
		return nil, fmt.Errorf("%w: %s", ErrProviderDisabled, ref)
	}
	if req.HasImages() && !e.info.Vision {
		return nil, fmt.Errorf("%w: %s does not take images", ErrUnsupported, ref)
	}
	if req.JSONSchema != nil && !e.info.StructuredOutput {
		return nil, fmt.Errorf("%w: %s does not support structured output", ErrUnsupported, ref)
	}
	return e, nil
}

// Complete resolves the ref, fills in the model and the default cap, and runs the call
// under the stage timeout. This is the one way the contexts above call a model.
func (r *Registry) Complete(ctx context.Context, ref ModelRef, req Request) (Response, error) {
	resolved, err := r.resolve(ref, req)
	if err != nil {
		return Response{}, err
	}
	req.Model = ref.ModelID
	if resolved.reasoning != ReasoningUnspecified {
		req.Reasoning = resolved.reasoning
	}
	if !req.Reasoning.Valid() {
		return Response{}, fmt.Errorf("invalid reasoning effort %q", req.Reasoning)
	}
	// Zero and negative both mean "no cap of my own": a caller computing a budget must
	// not send a negative number to the provider and learn about it as a 400.
	if req.MaxTokens <= 0 {
		req.MaxTokens = r.opts.MaxTokens
	}
	ctx, cancel := context.WithTimeout(ctx, r.opts.Timeout)
	defer cancel()
	return resolved.provider.Complete(ctx, req)
}
