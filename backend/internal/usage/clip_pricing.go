package usage

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type clipFailure string

func (e clipFailure) Error() string        { return string(e) }
func (e clipFailure) Failure() llm.Failure { return llm.Failure{Reason: string(e)} }

const ErrClipPricing = clipFailure("CLIP_MODEL_PRICING_UNAVAILABLE")
const ErrClipApproval = clipFailure("CLIP_QUOTE_REQUIRED")

// Bound before converting to machine integers: even an extreme reported supplier
// overage cannot wrap into a negative charge or manufacture an oversized refund.
func boundedClipCharge(microusd int64, ceiling int) int {
	if ceiling <= 0 {
		return 0
	}
	if microusd <= 0 {
		return min(plan.ChargeBase, ceiling)
	}
	n := new(big.Int).SetInt64(microusd)
	n.Mul(n, big.NewInt(int64(plan.ChargeMultiplier)))
	n.Add(n, big.NewInt(9999)).Quo(n, big.NewInt(10000)).Add(n, big.NewInt(int64(plan.ChargeBase)))
	if n.Cmp(big.NewInt(int64(ceiling))) >= 0 {
		return ceiling
	}
	return int(n.Int64())
}

type CreditCeilingError struct{ Required, Approved int }

const FailureReasonClipCreditCeiling = "CLIP_CREDIT_CEILING_EXCEEDED"

func (e *CreditCeilingError) Error() string {
	return fmt.Sprintf("clip needs %d credits above approved %d", e.Required, e.Approved)
}
func (e *CreditCeilingError) Failure() llm.Failure {
	return llm.Failure{Reason: FailureReasonClipCreditCeiling, Params: map[string]string{"required": fmt.Sprint(e.Required), "approved": fmt.Sprint(e.Approved)}}
}

type PricedCall struct {
	Policy llm.CallPolicy
	Count  int
}
type ClipReservation struct {
	ApprovedMaxCredits int
	Calls              []PricedCall
}

// ClipCredits is shared by quoting and admission. Unknown/overflowing rates cannot
// silently turn a paid model into a base-only quote. The ordinary estimator is unchanged.
func ClipCredits(calls []PricedCall) (int, error) {
	if len(calls) == 0 || len(calls) > 2 {
		return 0, ErrClipPricing
	}
	var total int64
	for _, call := range calls {
		if !call.Policy.Valid() || call.Count <= 0 || call.Count > 49 {
			return 0, ErrClipPricing
		}
		cost, ok := call.Policy.QuoteMicrousd()
		if !ok || cost > (math.MaxInt64-total)/int64(call.Count) {
			return 0, ErrClipPricing
		}
		total += cost * int64(call.Count)
	}
	// Charge multiplies in integer space; reject before either int64 or int can wrap.
	if total > (math.MaxInt64-9999)/int64(plan.ChargeMultiplier) {
		return 0, ErrClipPricing
	}
	credits := plan.Charge(total)
	if credits < plan.ChargeBase || credits > math.MaxInt32 {
		return 0, ErrClipPricing
	}
	return credits, nil
}

type callPriceKey struct{}
type callPrice struct {
	user, job string
	policy    llm.CallPolicy
}

// WithCallPrice is installed by the metered boundary from its reserved allowance,
// never from RPC fields, so token-derived usage survives later catalog changes.
func WithCallPrice(ctx context.Context, user, job string, policy llm.CallPolicy) context.Context {
	return context.WithValue(ctx, callPriceKey{}, callPrice{user, job, policy})
}

func frozenCallPrice(ctx context.Context, call Call) (llm.CallPolicy, bool) {
	p, ok := ctx.Value(callPriceKey{}).(callPrice)
	return p.policy, ok && p.user == call.UserID && p.job == call.JobID && p.policy.Ref == call.Model && p.policy.Stage == call.Stage && p.policy.Valid()
}
