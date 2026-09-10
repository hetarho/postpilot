package llm

import (
	"math/big"
	"regexp"
	"slices"
)

// CallPolicy is an immutable, provider-neutral snapshot approved for one stage.
// Prices are USD per million tokens, not floats and not an eventual provider bill.
type CallPolicy struct {
	Ref                                     ModelRef
	Stage                                   string
	CompletionTokens                        int
	Reasoning                               ReasoningEffort
	DisableReasoning                        bool
	InputUSDPerMillion, OutputUSDPerMillion string
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
	return p.Ref.ProviderID != "" && p.Ref.ModelID != "" && p.Stage != "" && p.CompletionTokens > 0 && p.Reasoning.Valid() && ValidUnitPrice(p.InputUSDPerMillion) && ValidUnitPrice(p.OutputUSDPerMillion)
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
	p := CallPolicy{Ref: ref, Stage: stage, CompletionTokens: budget, Reasoning: reasoning, DisableReasoning: reasoning == ReasoningNone && !slices.Contains(m.ReasoningEfforts, ReasoningNone), InputUSDPerMillion: m.InputUSDPerMillion, OutputUSDPerMillion: m.OutputUSDPerMillion}
	if !p.Reasoning.Valid() || budget <= 0 {
		return CallPolicy{}, ErrUnsupported
	}
	return p, nil
}
