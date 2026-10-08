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
// SourceModel.Stages. Owned here so the contexts on
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
	Ref    ModelRef
	Label  string
	Vision bool
	// VideoInput: the model takes a video part. Narrower than Vision and checked per
	// request, not per stage — a video-blind model still serves a post with no clip.
	VideoInput            bool
	VideoDelivery         VideoDelivery
	StructuredOutput      bool
	ContextTokens         int64
	InputUSDPerMillion    string
	OutputUSDPerMillion   string
	PricingCheckedAt      string
	ReasoningNativeEffort bool
	// Stages this model may serve, passed through from the source verbatim (see
	// SourceModel.Stages). Empty means no user-facing stage lists it.
	Stages []string
	// Levels is the source's curated grade per stage, passed through verbatim.
	Levels         map[string]string
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
	VideoInput          bool
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
	// silently changed photo observation (MODEL-7).
	Reasoning map[string]ReasoningEffort
	// ReasoningEfforts is the model's recorded native vocabulary. NativeEffort distinguishes
	// models whose provider consumes the effort string directly and therefore need completion
	// headroom for writing and revision.
	ReasoningEfforts      []ReasoningEffort
	ReasoningNativeEffort bool
	// Stages this model is registered to serve, in the stable string form the StageName
	// constants spell ("observe"/"write"/"analyze"). The strings are the
	// source's to define — the registry passes them through without interpreting them, the
	// same posture it takes to labels. An empty set is a model curated for a purpose no
	// stage consumes yet (image/video generation).
	Stages []string
	// Levels is the curated classification per stage, used by selector/admission entitlement
	// checks. Execution with grade enforcement requires a trusted admitted grade; these source
	// values alone confer no rights. An absent stage is unclassified for a new selection.
	Levels map[string]string
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

// Options tune every call the registry dispatches.
type Options struct {
	// SpeechAdapters are optional; the completion catalog remains unchanged.
	SpeechAdapters map[string]SpeechAdapterFactory
	// Timeout bounds one provider call (PRD §6.6: 단계당 5분).
	Timeout time.Duration
	// MaxTokens is the completion cap when a request sets none.
	MaxTokens int
	// EndpointCacheTTL is how long an adapter may reuse one successfully read
	// endpoint document for clip qualification, and EndpointFetchTimeout bounds one
	// such read. They are the catalog's own TTL and fetch timeout, so a route is
	// never trusted longer than the model list that named it. Zero means the
	// adapter's defaults.
	EndpointCacheTTL     time.Duration
	EndpointFetchTimeout time.Duration
}

// The yaml shape. Field names are the contract documented in providers.yaml; unknown
// fields are an error, which is also what retires the old per-provider `models:` list and
// the old `recommendation_sets:` list (those are rows now, MODEL-10): a stack still mounting
// either is told at boot rather than quietly serving an empty catalog or losing its sets.
type registryFile struct {
	Providers      []providerEntry      `yaml:"providers"`
	SpeechProvider *speechProviderEntry `yaml:"speech_provider"`
}

type speechProviderEntry struct {
	ID        string `yaml:"id"`
	Adapter   string `yaml:"adapter"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
}

type providerEntry struct {
	ID              string `yaml:"id"`
	Adapter         string `yaml:"adapter"`
	BaseURL         string `yaml:"base_url"`
	APIKeyEnv       string `yaml:"api_key_env"`
	ReasoningFormat string `yaml:"reasoning_format"`
}

// Registry joins the two halves of "which model can I call": the connection, which is
// configuration read once at boot, and the usable-model list, which is curated data read
// live from the source.
type Registry struct {
	speech               SpeechProvider
	speechID             string
	speechDisabled       bool
	speechDisabledReason string
	modelGrades          bool
	providerID           string
	provider             Provider
	// disabled/disabledReason describe the provider, not one model: an unset key takes the
	// whole endpoint out at once.
	disabled       bool
	disabledReason string
	baseURL        string
	source         ModelSource
	opts           Options
}

// WithModelGrades requires a trusted, durable admission on every provider call.
func (r *Registry) WithModelGrades() *Registry { r.modelGrades = true; return r }

type admittedCall struct {
	ref          ModelRef
	stage, grade string
}
type admittedCallsKey struct{}

// WithAdmittedCalls is called only by the composition root after reading the
// ledger. Request payloads cannot grant a model grade or free routing.
func WithAdmittedCalls(ctx context.Context, calls []AdmittedCall) context.Context {
	trusted := make([]admittedCall, 0, len(calls))
	for _, call := range calls {
		trusted = append(trusted, admittedCall{ref: call.Ref, stage: call.Stage, grade: call.Grade})
	}
	return context.WithValue(ctx, admittedCallsKey{}, trusted)
}

type AdmittedCall struct {
	Ref          ModelRef
	Stage, Grade string
}

func admittedGrade(ctx context.Context, ref ModelRef, stage string) (string, bool) {
	calls, _ := ctx.Value(admittedCallsKey{}).([]admittedCall)
	for _, call := range calls {
		if call.ref == ref && call.stage == stage {
			return call.grade, true
		}
	}
	return "", false
}

// QualifyFree checks a provider leaf's current complete price envelope for
// operator curation. The catalog service owns the policy and supplies the
// applicable path; the adapter owns provider document interpretation.
func (r *Registry) QualifyFree(ctx context.Context, modelID string, path FreePath) (bool, error) {
	checker, ok := r.provider.(interface {
		HasFreePath(context.Context, string, FreePath) (bool, error)
	})
	if !ok || r.disabled {
		return false, nil
	}
	return checker.HasFreePath(ctx, modelID, path)
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
	// change (MODEL-10), and this is where it announces itself.
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
		EndpointCacheTTL: opts.EndpointCacheTTL, EndpointFetchTimeout: opts.EndpointFetchTimeout,
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
	if p := file.SpeechProvider; p != nil {
		if strings.TrimSpace(p.ID) == "" || p.ID == reg.providerID || strings.TrimSpace(p.APIKeyEnv) == "" {
			return nil, fmt.Errorf("speech_provider: distinct id and api_key_env are required")
		}
		factory, ok := opts.SpeechAdapters[p.Adapter]
		if !ok {
			return nil, fmt.Errorf("speech_provider: unknown adapter %q", p.Adapter)
		}
		key := getenv(p.APIKeyEnv)
		reg.speech, err = factory(SpeechAdapterConfig{ProviderID: p.ID, BaseURL: p.BaseURL, APIKey: key})
		if err != nil {
			return nil, fmt.Errorf("speech_provider: %w", err)
		}
		if reg.speech == nil {
			return nil, fmt.Errorf("speech_provider: adapter returned no provider")
		}
		reg.speechID = p.ID
		if key == "" {
			reg.speechDisabled = true
			reg.speechDisabledReason = DisabledReasonNoKey
		}
	}
	return reg, nil
}

// ProviderID is the id every ModelRef the registry serves carries.
func (r *Registry) ProviderID() string { return r.providerID }

// BaseURL is the registered endpoint. The catalog client derives the upstream model list
// from it, so the address is configured in one place; it never crosses to a client.
func (r *Registry) BaseURL() string { return r.baseURL }

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
		Ref:                   ModelRef{ProviderID: r.providerID, ModelID: m.ModelID},
		Label:                 m.Label,
		Vision:                m.Vision,
		VideoInput:            m.VideoInput,
		StructuredOutput:      m.StructuredOutput,
		ContextTokens:         m.ContextTokens,
		InputUSDPerMillion:    m.InputUSDPerMillion,
		OutputUSDPerMillion:   m.OutputUSDPerMillion,
		PricingCheckedAt:      m.PricingCheckedAt,
		ReasoningNativeEffort: m.ReasoningNativeEffort,
		Stages:                append([]string(nil), m.Stages...),
		Levels:                maps.Clone(m.Levels),
	}
	if delivery, ok := r.provider.(VideoDeliveryProvider); ok && m.VideoInput {
		info.VideoDelivery = delivery.VideoDelivery(m.ModelID)
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
	if err := req.ValidateParts(); err != nil {
		return SourceModel{}, err
	}
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
	if req.HasVideos() && !found.VideoInput {
		return SourceModel{}, fmt.Errorf("%w: %s does not take video", ErrUnsupported, ref)
	}
	if req.JSONSchema != nil && !found.StructuredOutput {
		return SourceModel{}, fmt.Errorf("%w: %s does not support structured output", ErrUnsupported, ref)
	}
	return found, nil
}

// resolveRequest is the shared pure preparation path. It reads registration and
// trusted admission context, but calls no adapter, endpoint qualifier or ledger.
func (r *Registry) resolveRequest(ctx context.Context, ref ModelRef, req Request) (Request, EffectiveRequestConditions, error) {
	var conditions EffectiveRequestConditions
	if err := ctx.Err(); err != nil {
		return Request{}, conditions, err
	}
	if r.modelGrades {
		grade, found := admittedGrade(ctx, ref, req.Stage)
		if !found {
			return Request{}, conditions, ErrModelUnavailable
		}
		switch grade {
		case "free", "value", "balanced", "premium", "top":
		default:
			return Request{}, conditions, ErrModelUnavailable
		}
		req.FreeCall = grade == "free"
	}
	resolved, err := r.resolve(ref, req)
	if err != nil {
		return Request{}, conditions, err
	}
	req.Model = ref.ModelID
	frozen := req.Execution != nil
	defaultBudget := false
	if frozen {
		if !req.Execution.Matches(ref, req) || !r.modelGrades && !slices.Contains(resolved.Stages, req.Stage) {
			return Request{}, conditions, ErrUnsupported
		}
		if req.Execution.Delivery == ExecutionInlineStatic && !resolved.VideoInput {
			return Request{}, conditions, ErrVideoInputAbsent
		}
	} else {
		if override, ok := resolved.Reasoning[req.Stage]; ok && override != ReasoningUnspecified {
			req.Reasoning = override
		}
		if !req.Reasoning.Valid() {
			return Request{}, conditions, fmt.Errorf("invalid reasoning effort %q", req.Reasoning)
		}
		req.DisableReasoning = req.Reasoning == ReasoningNone && !slices.Contains(resolved.ReasoningEfforts, ReasoningNone)
		if req.MaxTokens <= 0 {
			req.MaxTokens = r.opts.MaxTokens
			defaultBudget = true
		}
	}
	if err := ctx.Err(); err != nil {
		return Request{}, conditions, err
	}
	budget := int64(req.MaxTokens)
	effort := req.Reasoning
	structured := req.JSONSchema != nil
	disable, free := req.DisableReasoning, req.FreeCall
	omitted := effort == ReasoningUnspecified || effort == ReasoningUnset || disable
	conditions = EffectiveRequestConditions{Model: &ref, MaxCompletionTokens: &budget, ReasoningEffort: &effort, StructuredOutput: &structured, DisableReasoning: &disable, FreeCall: &free, DefaultBudget: &defaultBudget, FrozenExecution: &frozen, ReasoningOmitted: &omitted}
	return req, conditions, nil
}

// Prepare observes current effective product conditions without invoking an
// adapter. A model-grade registry requires the same trusted admission as Complete.
func (r *Registry) Prepare(ctx context.Context, ref ModelRef, req Request) (RequestInspection, error) {
	effective, conditions, err := r.resolveRequest(ctx, ref, req)
	if err != nil {
		return RequestInspection{}, err
	}
	inspection, err := PreparedRequestInspection(effective)
	if err != nil {
		return RequestInspection{}, err
	}
	if inspection.Status == InspectionUnavailable {
		return inspection, nil
	}
	inspection.Conditions = &conditions
	return inspection, inspection.Validate()
}

// Complete uses exactly the same resolution as Prepare. Capture is an adapter
// invocation witness; it does not imply network acceptance or a successful result.
func (r *Registry) Complete(ctx context.Context, ref ModelRef, req Request) (Response, error) {
	effective, conditions, err := r.resolveRequest(ctx, ref, req)
	if err != nil {
		return Response{}, err
	}
	prepared, captureErr := PreparedRequestInspection(effective)
	if captureErr == nil && prepared.Status != InspectionUnavailable {
		prepared.Conditions = &conditions
	}
	callCtx, cancel := context.WithTimeout(ctx, r.opts.Timeout)
	defer cancel()
	if err := callCtx.Err(); err != nil {
		return Response{}, err
	}
	invokedAt := time.Now().UTC()
	response, callErr := r.provider.Complete(callCtx, effective)
	callErr = stripProviderInspectionError(callErr)
	// Provider-returned metadata cannot supply registry evidence.
	response.Inspection = nil
	if captureErr == nil && prepared.Status != InspectionUnavailable {
		captured, err := CapturedRequestInspection(prepared, invokedAt, response.Usage)
		if err == nil {
			response.Inspection = &captured
			callErr = WithRequestInspectionError(callErr, captured)
		}
	}
	return response, callErr
}
