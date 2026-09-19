package usage

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type failureReason string

func (e failureReason) Error() string        { return string(e) }
func (e failureReason) Failure() llm.Failure { return llm.Failure{Reason: string(e)} }

const ErrPricingUnavailable = failureReason("CLIP_MODEL_PRICING_UNAVAILABLE")
const ErrApprovalRequired = failureReason("CLIP_QUOTE_REQUIRED")

// Bound before converting to machine integers: even an extreme reported supplier
// overage cannot wrap into a negative charge or manufacture an oversized refund.
func boundedCharge(microusd int64, ceiling int) int {
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

const FailureReasonCreditCeiling = "CLIP_CREDIT_CEILING_EXCEEDED"

func (e *CreditCeilingError) Error() string {
	return fmt.Sprintf("work needs %d credits above the approved %d", e.Required, e.Approved)
}
func (e *CreditCeilingError) Failure() llm.Failure {
	return llm.Failure{Reason: FailureReasonCreditCeiling, Params: map[string]string{"required": fmt.Sprint(e.Required), "approved": fmt.Sprint(e.Approved)}}
}

type PricedCall struct {
	Policy llm.CallPolicy
	Count  int
}
type Reservation struct {
	CancellationPolicyVersion int
	ApprovedMaxCredits        int
	Calls                     []PricedCall
}

// The fee uses the unused job hold, with upward integer rounding and no overflow.
func cancelledCharge(confirmedMicrousd int64, reservation int) (confirmed, fee int) {
	if confirmedMicrousd > 0 {
		confirmed = boundedCharge(confirmedMicrousd, reservation)
	}
	unused := max(0, reservation-confirmed)
	return confirmed, unused/2 + unused%2
}

// ReservationCredits is shared by quoting and admission. Unknown/overflowing rates cannot
// silently turn a paid model into a base-only quote. The ordinary estimator is unchanged.
func ReservationCredits(calls []PricedCall) (int, error) {
	if len(calls) == 0 || len(calls) > 2 {
		return 0, ErrPricingUnavailable
	}
	var total int64
	for _, call := range calls {
		if !call.Policy.Valid() || call.Count < 0 || call.Count > 49*(1+call.Policy.ResponseRetries) {
			return 0, ErrPricingUnavailable
		}
		if call.Count == 0 {
			continue
		}
		cost, ok := call.Policy.QuoteMicrousd()
		if !ok || cost > (math.MaxInt64-total)/int64(call.Count) {
			return 0, ErrPricingUnavailable
		}
		total += cost * int64(call.Count)
	}
	// Charge multiplies in integer space; reject before either int64 or int can wrap.
	if total > (math.MaxInt64-9999)/int64(plan.ChargeMultiplier) {
		return 0, ErrPricingUnavailable
	}
	if total == 0 {
		anyCalls := false
		for _, c := range calls {
			anyCalls = anyCalls || c.Count > 0
		}
		if !anyCalls {
			return 0, nil
		}
	}
	credits := plan.Charge(total)
	if credits < plan.ChargeBase || credits > math.MaxInt32 {
		return 0, ErrPricingUnavailable
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
