package llm

import "math/big"

// CostSource records how a call's cost was arrived at, because "0" means two very
// different things: a provider that charged nothing, and a provider that told us
// nothing. Both the A/B leaderboard and the account ledger keep it per row.
type CostSource string

const (
	CostReported    CostSource = "reported"
	CostEstimated   CostSource = "estimated"
	CostUnavailable CostSource = "unavailable"
)

// Cost is one resolved call cost in micro-USD (millionths of one USD) with its
// provenance.
type Cost struct {
	Microusd int64
	Source   CostSource
}

// CostInput is one call's reported usage together with the registry prices for the model
// that ran it. It is a plain struct rather than a ModelInfo so a caller holding only a
// stored price pair (an experiment row, a replayed event) can still resolve.
type CostInput struct {
	PromptTokens     int64
	CompletionTokens int64
	// ReportedMicrousd is meaningful only when Reported is set; a provider that omits
	// cost reports a zero we must not mistake for a free call.
	ReportedMicrousd    int64
	Reported            bool
	InputUSDPerMillion  string
	OutputUSDPerMillion string
}

// ResolveCost applies the one precedence the product uses everywhere: what the provider
// charged, else what the registry's prices say it should have cost, else nothing known.
//
// It lives here because pricing is registry metadata, and because both the experiment
// leaderboard and the account usage ledger must agree to the micro-USD — two resolvers
// would eventually disagree about the same call.
func ResolveCost(in CostInput) Cost {
	if in.Reported {
		return Cost{Microusd: in.ReportedMicrousd, Source: CostReported}
	}
	if in.PromptTokens == 0 && in.CompletionTokens == 0 {
		return Cost{Source: CostUnavailable}
	}
	microusd, ok := estimateMicrousd(in.PromptTokens, in.CompletionTokens, in.InputUSDPerMillion, in.OutputUSDPerMillion)
	if !ok {
		return Cost{Source: CostUnavailable}
	}
	return Cost{Microusd: microusd, Source: CostEstimated}
}

// estimateMicrousd multiplies token counts by the per-million rates in exact rational
// arithmetic and rounds half-up once at the end. Floating point would drift on rates like
// $0.075, and the drift would land in a ledger that a budget is compared against.
func estimateMicrousd(promptTokens, completionTokens int64, inputRate, outputRate string) (int64, bool) {
	if promptTokens < 0 || completionTokens < 0 || inputRate == "" || outputRate == "" {
		return 0, false
	}
	input, ok := new(big.Rat).SetString(inputRate)
	if !ok || input.Sign() < 0 {
		return 0, false
	}
	output, ok := new(big.Rat).SetString(outputRate)
	if !ok || output.Sign() < 0 {
		return 0, false
	}
	total := new(big.Rat).Mul(input, big.NewRat(promptTokens, 1))
	total.Add(total, new(big.Rat).Mul(output, big.NewRat(completionTokens, 1)))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(total.Num(), total.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(total.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, false
	}
	return quotient.Int64(), true
}
