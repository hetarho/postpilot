package llm

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

// DisabledReasonNoKey is the reason shown for every model of a provider whose
// `api_key_env` is unset. The frontend displays it as is.
const DisabledReasonNoKey = "API key not configured"

// DisabledReasonDelisted is the reason shown for a curated model the upstream catalog no
// longer offers. The row is kept and served disabled rather than dropped, so a user whose
// saved choice it was is told why instead of finding the entry silently gone, and an
// operator decides when to retire it.
const DisabledReasonDelisted = "the provider no longer lists this model"

// The user-facing stage names, as the stable strings that cross this boundary in
// RecommendationSelection.Stage and SourceModel.Stages. Owned here so the contexts on
// either side of the port spell them identically — the strings are a cross-package
// contract a typo would break with no compile error.
const (
	StageNameObserve = "observe"
	StageNameWrite   = "write"
	StageNameAnalyze = "analyze"
)

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
	// Stages this model may serve, passed through from the source verbatim (see
	// SourceModel.Stages). Empty means no user-facing stage lists it.
	Stages         []string
	Disabled       bool
	DisabledReason string
}

// ServesStage reports whether the catalog registered this model for a stage. It is the
// membership test every execution boundary uses, so a purpose deregistration is enforced
// wherever a client-supplied ref arrives — not only in the picker.
func (m ModelInfo) ServesStage(stage string) bool {
	return slices.Contains(m.Stages, stage)
}

// SourceModel is one curated model as the catalog source holds it: the model's own facts,
// with nothing yet said about whether its provider can be reached.
type SourceModel struct {
	ModelID             string
	Label               string
	Vision              bool
	StructuredOutput    bool
	ContextTokens       int64
	InputUSDPerMillion  string
	OutputUSDPerMillion string
	PricingCheckedAt    string
	// Reasoning is the strict operator override PER STAGE, keyed by the stable stage names
	// StageNameObserve/Write/Analyze. A stage absent from the map, or present as
	// Unspecified, defers to the stage policy the caller set; Unset deliberately omits the
	// wire key.
	//
	// Per stage rather than per model because the policy it overrides is per stage: one
	// value for the whole model erased that distinction, so lowering the effort for writing
	// silently changed photo observation (change 24).
	Reasoning map[string]ReasoningEffort
	// Stages this model is registered to serve, in the same stable string form
	// RecommendationSelection.Stage uses ("observe"/"write"/"analyze"). The strings are the
	// source's to define — the registry passes them through without interpreting them, the
	// same posture it takes to labels. An empty set is a model curated for a purpose no
	// stage consumes yet (image/video generation).
	Stages []string
	// Delisted marks a model the upstream catalog no longer offered at the last successful
	// refresh.
	Delisted bool
}

// ModelSource is where the registry's usable models come from — declared here by its
// consumer (ARCHITECTURE §2.2) and injected by the composition root.
//
// It is read on every catalog request rather than snapshotted at boot: curating a model
// must take effect for the next request, not the next deploy. Implementations are shared
// across request goroutines and must be safe for concurrent use.
type ModelSource interface {
	// Models returns every usable model, in the order the catalog should show them.
	Models() []SourceModel
	Lookup(modelID string) (SourceModel, bool)
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
// fields are an error, which is also what retires the old per-provider `models:` list: a
// stack still mounting one is told at boot rather than quietly serving an empty catalog.
type registryFile struct {
	Providers          []providerEntry          `yaml:"providers"`
	RecommendationSets []recommendationSetEntry `yaml:"recommendation_sets"`
}

type providerEntry struct {
	ID              string `yaml:"id"`
	Adapter         string `yaml:"adapter"`
	BaseURL         string `yaml:"base_url"`
	APIKeyEnv       string `yaml:"api_key_env"`
	ReasoningFormat string `yaml:"reasoning_format"`
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

// Registry joins the two halves of "which model can I call": the connection, which is
// configuration read once at boot, and the usable-model list, which is curated data read
// live from the source.
type Registry struct {
	providerID string
	provider   Provider
	// disabled/disabledReason describe the provider, not one model: an unset key takes the
	// whole endpoint out at once.
	disabled        bool
	disabledReason  string
	baseURL         string
	source          ModelSource
	recommendations []RecommendationSet
	opts            Options
}

// Load reads and validates the registry file. Any problem is returned as an error the
// composition root turns into a refused boot: a broken registry must be loud, like a
// broken migration.
//
// `getenv` resolves the provider's `api_key_env`; `adapters` maps adapter names to the
// factories the composition root chose to ship (the only place that imports them);
// `source` supplies the curated models.
func Load(path string, getenv func(string) string, adapters map[string]AdapterFactory, source ModelSource, opts Options) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read providers config %s: %w", path, err)
	}
	reg, err := Parse(data, getenv, adapters, source, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return reg, nil
}

// Parse is Load without the file — the testable core.
func Parse(data []byte, getenv func(string) string, adapters map[string]AdapterFactory, source ModelSource, opts Options) (*Registry, error) {
	if opts.Timeout <= 0 {
		return nil, fmt.Errorf("registry options: timeout must be positive")
	}
	if opts.MaxTokens <= 0 {
		return nil, fmt.Errorf("registry options: max tokens must be positive")
	}
	if source == nil {
		return nil, fmt.Errorf("registry options: a model source is required")
	}

	var file registryFile
	if err := yaml.UnmarshalWithOptions(data, &file, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("invalid yaml: %s", yaml.FormatError(err, false, true))
	}
	// Exactly one, not at least one: the curated catalog has no provider dimension — every
	// row is served by the registered endpoint — so a second entry would have models
	// attributed to it that nobody chose. Adding a genuinely different vendor is a design
	// change (plan 18 non-goals), and this is where it announces itself.
	if len(file.Providers) != 1 {
		return nil, fmt.Errorf("exactly one provider must be declared, found %d", len(file.Providers))
	}

	p := file.Providers[0]
	where := "providers[0]"
	if p.ID != "" {
		where = fmt.Sprintf("provider %q", p.ID)
	}
	if strings.TrimSpace(p.ID) == "" {
		return nil, fmt.Errorf("%s: id is required", where)
	}
	factory, ok := adapters[p.Adapter]
	if !ok {
		return nil, fmt.Errorf("%s: unknown adapter %q (known: %s)", where, p.Adapter, strings.Join(slices.Sorted(maps.Keys(adapters)), ", "))
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

	reg := &Registry{
		providerID: p.ID,
		provider:   provider,
		baseURL:    p.BaseURL,
		source:     source,
		opts:       opts,
	}
	if p.APIKeyEnv != "" && key == "" {
		reg.disabled = true
		reg.disabledReason = DisabledReasonNoKey
	}
	if err := reg.loadRecommendations(file.RecommendationSets); err != nil {
		return nil, err
	}
	return reg, nil
}

// ProviderID is the id every ModelRef the registry serves carries.
func (r *Registry) ProviderID() string { return r.providerID }

// BaseURL is the registered endpoint. The catalog client derives the upstream model list
// from it, so the address is configured in one place; it never crosses to a client.
func (r *Registry) BaseURL() string { return r.baseURL }

// loadRecommendations validates SHAPE only. Whether a referenced model is usable can no
// longer be settled at boot — the catalog is curated data that changes while the process
// runs — so existence, capability and plan are checked where the set is applied, against
// the registry as it is at that moment.
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
			if selection.Stage != StageNameObserve && selection.Stage != StageNameWrite && selection.Stage != StageNameAnalyze {
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
			for _, ref := range []ModelRef{converted.Active, converted.CandidateA, converted.CandidateB} {
				if ref.ProviderID == "" || ref.ModelID == "" {
					return fmt.Errorf("recommendation set %q stage %q: every ref needs a provider_id and a model_id", item.ID, selection.Stage)
				}
			}
			if converted.CandidateA == converted.CandidateB {
				return fmt.Errorf("recommendation set %q stage %q: candidates must differ", item.ID, selection.Stage)
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

// Models is the curated catalog in source order. An empty catalog is a valid state — a
// fresh install has nothing curated yet — and renders as an empty dropdown, not a failure.
func (r *Registry) Models() []ModelInfo {
	models := r.source.Models()
	out := make([]ModelInfo, 0, len(models))
	for _, m := range models {
		out = append(out, r.describe(m))
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

// Lookup returns the entry for a ref, if the catalog currently offers it.
func (r *Registry) Lookup(ref ModelRef) (ModelInfo, bool) {
	if ref.ProviderID != r.providerID {
		return ModelInfo{}, false
	}
	found, ok := r.source.Lookup(ref.ModelID)
	if !ok {
		return ModelInfo{}, false
	}
	return r.describe(found), true
}

// describe joins a curated model with what the process knows about reaching it. The two
// disabling reasons are ordered by what the reader can act on: a missing key takes out
// every model at once and is the operator's first move, so it wins over a delisting that
// concerns this row alone.
func (r *Registry) describe(m SourceModel) ModelInfo {
	info := ModelInfo{
		Ref:                 ModelRef{ProviderID: r.providerID, ModelID: m.ModelID},
		Label:               m.Label,
		Vision:              m.Vision,
		StructuredOutput:    m.StructuredOutput,
		ContextTokens:       m.ContextTokens,
		InputUSDPerMillion:  m.InputUSDPerMillion,
		OutputUSDPerMillion: m.OutputUSDPerMillion,
		PricingCheckedAt:    m.PricingCheckedAt,
		Stages:              append([]string(nil), m.Stages...),
	}
	switch {
	case r.disabled:
		info.Disabled, info.DisabledReason = true, r.disabledReason
	case m.Delisted:
		info.Disabled, info.DisabledReason = true, DisabledReasonDelisted
	}
	return info
}

// resolve finds the model for a ref and checks that it can take what the request asks of
// it — before any network call, so a caller learns "this model cannot see images" in
// microseconds rather than after a round trip.
//
// Unexported on purpose: handing the Provider out would let a caller skip the timeout
// and the defaults that Complete applies. Callers that need the flags use Lookup.
func (r *Registry) resolve(ref ModelRef, req Request) (SourceModel, error) {
	if ref.ProviderID != r.providerID {
		return SourceModel{}, fmt.Errorf("%w: %s is not registered", ErrModelUnavailable, ref)
	}
	found, ok := r.source.Lookup(ref.ModelID)
	if !ok {
		return SourceModel{}, fmt.Errorf("%w: %s is not registered", ErrModelUnavailable, ref)
	}
	if r.disabled {
		return SourceModel{}, fmt.Errorf("%w: %s", ErrProviderDisabled, ref)
	}
	if found.Delisted {
		return SourceModel{}, fmt.Errorf("%w: %s is no longer offered", ErrModelUnavailable, ref)
	}
	if req.HasImages() && !found.Vision {
		return SourceModel{}, fmt.Errorf("%w: %s does not take images", ErrUnsupported, ref)
	}
	if req.JSONSchema != nil && !found.StructuredOutput {
		return SourceModel{}, fmt.Errorf("%w: %s does not support structured output", ErrUnsupported, ref)
	}
	return found, nil
}

// Complete resolves the ref, fills in the model and the default cap, and runs the call
// under the stage timeout. This is the one way the contexts above call a model.
func (r *Registry) Complete(ctx context.Context, ref ModelRef, req Request) (Response, error) {
	resolved, err := r.resolve(ref, req)
	if err != nil {
		return Response{}, err
	}
	req.Model = ref.ModelID
	// This stage's override → the stage value the caller set → nothing sent. The order is
	// unchanged in shape; the override half is what gained the stage dimension.
	if override, ok := resolved.Reasoning[req.Stage]; ok && override != ReasoningUnspecified {
		req.Reasoning = override
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
	return r.provider.Complete(ctx, req)
}
