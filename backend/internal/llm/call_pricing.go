package llm

import (
	"context"
	"math/big"
	"strings"
)

// Version 2 froze the chosen provider leaf and the parameters the request
// requires beside the price; a version-1 envelope stored by an older quote or
// queued job is refused and re-quoted rather than read with a guessed route.
const CallPricingVersion = 2
const ClipInputUnits = 30_000

// MaxEndpointTag bounds a frozen provider leaf; the adapter's own tag grammar is
// stricter, this only keeps a stored envelope readable.
const MaxEndpointTag = 128

// CallPricing is a durable, provider-neutral admission envelope. The opaque
// fingerprint pins the adapter's documented route and applicable billing profile;
// it is not a client-supplied routing key. Token envelopes in CallPolicy are NOT
// measured prices when AggregateUsageSufficient is false.
//
// Endpoint is the one provider leaf the adapter qualified and will route to with
// fallbacks disabled; RequiredParameters is the comma-joined sorted list of
// request parameters that leaf must support (CallPolicy.RequiredParameters).
// Both are opaque to everything above the llm boundary.
type CallPricing struct {
	Version                                      int
	Fingerprint                                  string
	Delivery                                     ExecutionDelivery
	Endpoint                                     string
	RequiredParameters                           string
	PromptUSDPerMillion, CompletionUSDPerMillion string
	RequestUSD, ImageUSD, AudioUSDPerToken       string
	AggregateUsageSufficient                     bool
}

func (p CallPricing) Valid() bool {
	if p.Version != CallPricingVersion || len(p.Fingerprint) != 64 || (p.Delivery != ExecutionTextOnly && p.Delivery != ExecutionInlineStatic) {
		return false
	}
	if p.Endpoint == "" || len(p.Endpoint) > MaxEndpointTag || !strings.Contains(","+p.RequiredParameters+",", ",max_tokens,") {
		return false
	}
	for _, c := range p.Fingerprint {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	for _, price := range []string{p.PromptUSDPerMillion, p.CompletionUSDPerMillion, p.RequestUSD, p.ImageUSD, p.AudioUSDPerToken} {
		if !ValidUnitPrice(price) {
			return false
		}
	}
	return true
}

type ExecutionPricingProvider interface {
	FreezePricing(context.Context, CallPolicy, ExecutionDelivery) (CallPolicy, error)
}

// QuoteMicrousd sums exact decimal upper bounds and rounds up only once. Image
// units are distinct per-frame charges, not another copy of prompt image tokens.
// This is shared by the quote and exact-count hold, never by usage settlement.
func (p CallPolicy) QuoteMicrousd() (int64, bool) {
	if !p.Valid() {
		return 0, false
	}
	in, _ := new(big.Rat).SetString(p.InputUSDPerMillion)
	out, _ := new(big.Rat).SetString(p.OutputUSDPerMillion)
	cost := new(big.Rat).Mul(in, big.NewRat(ClipInputUnits, 1))
	cost.Add(cost, new(big.Rat).Mul(out, big.NewRat(int64(p.CompletionTokens), 1)))
	if p.Pricing.Version != 0 {
		request, _ := new(big.Rat).SetString(p.Pricing.RequestUSD)
		cost.Add(cost, request.Mul(request, big.NewRat(1_000_000, 1)))
		if p.Pricing.Delivery == ExecutionInlineStatic {
			image, _ := new(big.Rat).SetString(p.Pricing.ImageUSD)
			cost.Add(cost, image.Mul(image, big.NewRat(60*1_000_000, 1)))
		}
	}
	n := new(big.Int).Quo(cost.Num(), cost.Denom())
	if new(big.Int).Mod(cost.Num(), cost.Denom()).Sign() > 0 {
		n.Add(n, big.NewInt(1))
	}
	return n.Int64(), n.IsInt64() && n.Sign() >= 0
}
