package usage

import (
	"context"
	"fmt"
	"math"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

type failureReason string

func (e failureReason) Error() string        { return string(e) }
func (e failureReason) Failure() llm.Failure { return llm.Failure{Reason: string(e)} }

const ErrPricingUnavailable = failureReason("CLIP_MODEL_PRICING_UNAVAILABLE")
const ErrApprovalRequired = failureReason("CLIP_QUOTE_REQUIRED")

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
	Rate                      plan.RateSnapshot
}

// ReservationCost sums the bounded per-call worst cases before any currency
// conversion. This is shared by a quote and its later admission.
func ReservationCost(calls []PricedCall) (int64, error) {
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
	return total, nil
}

// ReservationCreditsAt converts ReservationCost once at a frozen rate; it is shared by a quote
// and its admission. An unknown price, an overflow or a paid reservation without a valid
// rate refuses rather than quoting a paid model as free.
func ReservationCreditsAt(calls []PricedCall, rate plan.RateSnapshot) (int, error) {
	total, err := ReservationCost(calls)
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	credits, err := plan.ChargeAt(total, rate)
	if err != nil {
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
