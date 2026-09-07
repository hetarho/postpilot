// Package plan is the authorization ladder: the tier an account is on, how many credits
// that tier is granted each month, and what one piece of work costs in credits.
//
// It is a leaf domain package — stdlib only — because every other context asks it the
// same question from a different layer: auth carries a Plan on the session, usage holds
// and settles credits against it, and the rpc edges render its refusals.
package plan

import (
	"fmt"
	"strings"
	"time"
)

// Plan is one rung of the ladder. The string form is what the database column stores, so
// it is the canonical spelling in both directions.
type Plan string

const (
	Free   Plan = "free"
	Basic  Plan = "basic"
	Pro    Plan = "pro"
	Max    Plan = "max"
	Master Plan = "master"
)

// rank orders the ladder. It is unexported: nothing outside compares tiers any more —
// model access is decided by the balance, not by the rung — so the numbers exist only to
// keep Parse total and to give the admin surface a stable display order.
var rank = map[Plan]int{Free: 0, Basic: 1, Pro: 2, Max: 3, Master: 4}

// Parse converts a stored value into a Plan. An unknown value is an error rather than a
// silent Free: a corrupted row must fail loudly, not quietly change what an account may
// spend.
func Parse(value string) (Plan, error) {
	candidate := Plan(strings.TrimSpace(value))
	if _, ok := rank[candidate]; !ok {
		return "", fmt.Errorf("unknown plan %q (want free, basic, pro, max, or master)", value)
	}
	return candidate, nil
}

// Valid reports whether p is a known rung.
func (p Plan) Valid() bool { _, ok := rank[p]; return ok }

func (p Plan) String() string { return string(p) }

// Rank is the ladder position, for ordering a display. It is not an authorization
// comparison: nothing in the product gates on one tier being above another except the
// master-only procedure set, which compares against Master directly.
func (p Plan) Rank() int { return rank[p] }

// Ladder is every rung in order, for a surface that lists the tiers.
func Ladder() []Plan { return []Plan{Free, Basic, Pro, Max, Master} }

// A credit is the product's billing unit: a fixed $0.01 of list value, stored as an
// integer. It is not a cost measurement — the ledger keeps recording true provider cost
// in micro-USD underneath — which is why the two never need to reconcile.
const microusdPerCredit = 10_000

// The charge rule, owned by code rather than config: two deploys must never disagree
// about what a request costs, and neither is an operator knob.
//
// ChargeBase recovers the per-request infrastructure a pure cost multiple cannot see
// (storage, database, worker) and keeps a near-free model from being effectively
// unmetered. ChargeMultiplier covers the provider top-up fee, card fees, VAT and margin.
const (
	ChargeBase       = 2
	ChargeMultiplier = 3
)

// SignupBonusCredits is the one-time grant a free account is provisioned with, on top of
// its first monthly lot.
const SignupBonusCredits = 50

// monthlyCredits is the product rule for what a tier is granted each month. Zero means
// unlimited, not a zero allowance: only master carries it, and master is never refused.
var monthlyCredits = map[Plan]int{
	Free:   50,
	Basic:  220,
	Pro:    575,
	Max:    1200,
	Master: 0,
}

// monthlyPriceUSDCents is what each tier is intended to cost. It lives beside the grant it
// sizes: the two are one product decision, and a price that drifted from its grant would be
// a promise the ladder cannot keep.
//
// A paid rung grants MORE than its price buys at the par purchase rate of one credit per
// US cent: basic +10 %, pro +15 %, max +20 %. Subscribing must beat topping up, and more so
// the higher the rung.
//
// Charging these figures is BILLING's, not this package's.
var monthlyPriceUSDCents = map[Plan]int{
	Free:  0,
	Basic: 200,
	Pro:   500,
	Max:   1000,
}

// The reference post a comparison screen quotes in product terms: ten photos observed in
// batches of four, then one write. Every call is priced at the worst case the admission gate
// itself holds against — holdInputTokens of prompt — so the figure can never promise a post
// the gate would then refuse.
//
// These are constants rather than reads of internal/platform/config: OBSERVE_BATCH_SIZE and
// LLM_MAX_TOKENS_DEFAULT are per-installation env values, and a comparison figure that moved
// with an operator's environment would have two deploys quoting different post counts. They
// mirror the shipped defaults (batch 4, 512 completion tokens per photo, an 8 192 fallback)
// and must be revisited when those move.
const (
	referencePhotos                = 10
	referenceObserveBatch          = 4
	referenceObserveCompletion     = referenceObserveBatch * 512
	referenceWriteCompletion       = 8_192
	referenceInputTokensPerCall    = 30_000
	referenceInputMicrousdPerMTok  = 300_000
	referenceOutputMicrousdPerMTok = 2_500_000
)

// recommended is the rung the comparison screen marks. It lives beside the grants it
// compares because which rung to recommend is a product decision, not a client's.
const recommended = Pro

// Offer is one rung as a comparison screen lists it.
type Offer struct {
	Plan           Plan
	MonthlyCredits int
	PriceUSDCents  int
	// EstimatedPosts is how many reference posts the grant covers (→ReferencePostCredits).
	EstimatedPosts int
	// Recommended marks the one rung the screen highlights.
	Recommended bool
}

// Offers are the rungs on offer, in ladder order. Master is absent: it is the operator
// tier, not something anyone is offered.
func Offers() []Offer {
	rungs := []Plan{Free, Basic, Pro, Max}
	offers := make([]Offer, 0, len(rungs))
	for _, rung := range rungs {
		offers = append(offers, Offer{
			Plan:           rung,
			MonthlyCredits: monthlyCredits[rung],
			PriceUSDCents:  monthlyPriceUSDCents[rung],
			EstimatedPosts: EstimatedPosts(rung),
			Recommended:    Recommended(rung),
		})
	}
	return offers
}

// MonthlyCredits returns the tier's monthly grant. An unknown plan gets the strictest
// known tier rather than zero, because zero here means unlimited — a corrupted row must
// not read as an operator account.
func MonthlyCredits(p Plan) int {
	found, ok := monthlyCredits[p]
	if !ok {
		return monthlyCredits[Free]
	}
	return found
}

// ReferencePostCredits is what one reference post holds, in credits.
//
// It runs the reference case through Charge, the same rule every real hold pays, so the two
// can never quote different arithmetic: three observe calls at 7 credits each plus one write
// call at 11.
func ReferencePostCredits() int {
	observeCalls := (referencePhotos + referenceObserveBatch - 1) / referenceObserveBatch
	perObserve := Charge(referenceCallMicrousd(referenceObserveCompletion))
	perWrite := Charge(referenceCallMicrousd(referenceWriteCompletion))
	return observeCalls*perObserve + perWrite
}

// referenceCallMicrousd prices one reference call: a worst-case prompt plus that call's own
// completion budget.
func referenceCallMicrousd(completionTokens int) int64 {
	const perMTok = 1_000_000
	input := int64(referenceInputTokensPerCall) * referenceInputMicrousdPerMTok / perMTok
	output := int64(completionTokens) * referenceOutputMicrousdPerMTok / perMTok
	return input + output
}

// EstimatedPosts is how many reference posts a tier's monthly grant covers.
//
// It floors: telling someone their grant covers three posts when the third would be refused
// is worse than telling them two. Master is never asked — it is not on offer.
func EstimatedPosts(p Plan) int {
	perPost := ReferencePostCredits()
	if perPost <= 0 {
		return 0
	}
	return MonthlyCredits(p) / perPost
}

// Recommended reports whether this rung is the one a comparison screen marks.
func Recommended(p Plan) bool { return p == recommended }

// Unlimited reports whether this tier is exempt from the balance check. Its work is still
// held, recorded and settled — unlimited spend is exactly the account whose spend the
// operator most wants to be able to read.
func Unlimited(p Plan) bool { return p == Master }

// Charge converts a provider cost into the credits it consumes.
//
// The arithmetic is integer-only: the ledger stores micro-USD and a credit is exactly
// 10 000 of them, so no float ever enters the money path. The division rounds up, which
// is why a call too cheap to reach one credit still costs ChargeBase + 1 rather than
// disappearing.
func Charge(costMicrousd int64) int {
	if costMicrousd <= 0 {
		return ChargeBase
	}
	scaled := costMicrousd*ChargeMultiplier + microusdPerCredit - 1
	return ChargeBase + int(scaled/microusdPerCredit)
}

// seoul is the product's home timezone, fixed at UTC+9.
//
// It is a FixedZone rather than a tzdata lookup because the runtime image is
// distroless/static and ships no zoneinfo — and because KST has had no DST since 1988,
// so a fixed offset and the IANA zone agree on every instant this product will ever see.
// The zone is a product constant, not env config: two deploys must never disagree about
// when a month ends.
var seoul = time.FixedZone("Asia/Seoul", 9*60*60)

// MonthWindow returns the calendar month containing t: its start (inclusive) and the
// instant it renews (exclusive). The renewal boundary is calendar rather than
// per-account-anniversary so every refusal can name a date a user already understands.
func MonthWindow(t time.Time) (start, end time.Time) {
	local := t.In(seoul)
	start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, seoul)
	return start, start.AddDate(0, 1, 0)
}

// NextRenewal is the instant the monthly grant after the one containing t opens.
func NextRenewal(t time.Time) time.Time {
	_, end := MonthWindow(t)
	return end
}
