package llm

import (
	"context"
	"math/big"
	"regexp"
	"slices"
)

// CallPolicy is an immutable, provider-neutral snapshot approved for one stage.
// Prices are USD per million tokens, not floats and not an eventual provider bill.
type CallPolicy struct {
	Ref              ModelRef
	Stage            string
	CompletionTokens int
	Reasoning        ReasoningEffort
	DisableReasoning bool
	// StructuredOutput is whether the request this policy admits carries a JSON
	// schema. It is frozen with the price: a quote made for the parser fallback
	// cannot later execute with a schema parameter nobody qualified or priced.
	StructuredOutput                        bool
	InputUSDPerMillion, OutputUSDPerMillion string
	Pricing                                 CallPricing
}

// RequiredParameters is what the request this policy describes actually sends,
// derived from the request rather than a universal list: the completion cap
// always, the schema parameters only when a schema goes, and reasoning only
// when the request carries an effort or disables it. The caller asks the
// provider to route only where every one of them is supported.
func (p CallPolicy) RequiredParameters() []string {
	out := []string{"max_tokens"}
	if p.StructuredOutput {
		out = append(out, "response_format", "structured_outputs")
	}
	if p.DisableReasoning || (p.Reasoning != ReasoningUnspecified && p.Reasoning != ReasoningUnset) {
		out = append(out, "reasoning")
	}
	return out
}

var decimalPrice = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]{1,3})?$`)

func ValidUnitPrice(value string) bool {
	if len(value) == 0 || len(value) > 128 || !decimalPrice.MatchString(value) {
		return false
	}
	rate, ok := new(big.Rat).SetString(value)
	return ok && rate.Sign() >= 0
}

func (p CallPolicy) Valid() bool {
	return p.Ref.ProviderID != "" && p.Ref.ModelID != "" && p.Stage != "" && p.CompletionTokens > 0 && p.Reasoning.Valid() && ValidUnitPrice(p.InputUSDPerMillion) && ValidUnitPrice(p.OutputUSDPerMillion) && (p.Pricing == (CallPricing{}) || p.Pricing.Valid())
}

// FreezeExecution includes read-only adapter pricing discovery. This never calls
// a model, and a provider without an enforceable price profile cannot quote clips.
func (r *Registry) FreezeExecution(ctx context.Context, ref ModelRef, stage string, budget int, reasoning ReasoningEffort, delivery ExecutionDelivery) (CallPolicy, error) {
	p, err := r.FreezeCall(ref, stage, budget, reasoning)
	if err != nil {
		return CallPolicy{}, err
	}
	provider, ok := r.provider.(ExecutionPricingProvider)
	if !ok {
		return CallPolicy{}, ErrUnsupported
	}
	// The catalog's raw modality is the only admission the registry decides; whether
	// a current endpoint takes the bounded inline clip is the adapter's proof below
	// (CLIP-30). No provider slug or model family is consulted anywhere on this path.
	if delivery == ExecutionInlineStatic {
		info, found := r.Lookup(ref)
		if !found || !info.VideoInput {
			return CallPolicy{}, ErrVideoInputAbsent
		}
	}
	ctx, cancel := context.WithTimeout(ctx, r.opts.Timeout)
	defer cancel()
	return provider.FreezePricing(ctx, p, delivery)
}

// FreezeCall resolves the very same reasoning override as Complete, without a call.
func (r *Registry) FreezeCall(ref ModelRef, stage string, budget int, reasoning ReasoningEffort) (CallPolicy, error) {
	m, err := r.resolve(ref, Request{Stage: stage})
	if err != nil {
		return CallPolicy{}, err
	}
	if !slices.Contains(m.Stages, stage) {
		return CallPolicy{}, ErrModelUnavailable
	}
	if override, ok := m.Reasoning[stage]; ok && override != ReasoningUnspecified {
		reasoning = override
	}
	p := CallPolicy{Ref: ref, Stage: stage, CompletionTokens: budget, Reasoning: reasoning, DisableReasoning: reasoning == ReasoningNone && !slices.Contains(m.ReasoningEfforts, ReasoningNone), StructuredOutput: m.StructuredOutput, InputUSDPerMillion: m.InputUSDPerMillion, OutputUSDPerMillion: m.OutputUSDPerMillion}
	if !p.Reasoning.Valid() || budget <= 0 {
		return CallPolicy{}, ErrUnsupported
	}
	return p, nil
}
