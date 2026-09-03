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
	Basic:  200,
	Pro:    500,
	Max:    1000,
	Master: 0,
}

// monthlyPriceUSDCents is what each tier is intended to cost. It lives beside the grant it
// sizes: the two are one product decision, and a price that drifted from its grant would be
// a promise the ladder cannot keep.
//
// Nothing here moves money (PRD §9). These figures are published so a comparison screen can
// name them, not charged.
var monthlyPriceUSDCents = map[Plan]int{
	Free:  0,
	Basic: 200,
	Pro:   500,
	Max:   1000,
}

// Offer is one rung as a comparison screen lists it.
type Offer struct {
	Plan           Plan
	MonthlyCredits int
	PriceUSDCents  int
}

// Offers are the rungs on offer, in ladder order. Master is absent: it is the operator
// tier, not something anyone is offered.
func Offers() []Offer {
	rungs := []Plan{Free, Basic, Pro, Max}
	offers := make([]Offer, 0, len(rungs))
	for _, rung := range rungs {
		offers = append(offers, Offer{
			Plan: rung, MonthlyCredits: monthlyCredits[rung], PriceUSDCents: monthlyPriceUSDCents[rung],
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
