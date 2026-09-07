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

// What one unit of a post costs in TOKENS. A comparison screen's post count is proportional
// to the work the reader says they will do (QUOTA-40), so the estimate is assembled from
// these rather than from one fixed case.
//
// They are constants rather than reads of internal/platform/config: OBSERVE_BATCH_SIZE and
// the completion budgets are per-installation env values, and a comparison figure that moved
// with an operator's environment would have two deploys quoting different post counts
// (QUOTA-8). They mirror the shipped defaults and must be revisited when those move.
const (
	// What one observation call and one write call carry before any attachment: the
	// instructions, the template, the voice material and, for the write, the observations.
	estimatorObservePromptTokens = 2_000
	estimatorWritePromptTokens   = 6_000
	// A photo reaches the model already converted to a 1024 px JPEG (ARCH-19).
	estimatorTokensPerPhoto = 320
	// A clip is priced at an assumed length rather than at VIDEO-3's 60 s ceiling: the
	// ceiling is what a post may hold, not what one usually holds.
	estimatorAssumedVideoSeconds = 15
	estimatorTokensPerVideoSec   = 300
	// One structured observation entry per attachment.
	estimatorObserveOutputPerItem = 200
	// Korean runs about 1.2 tokens per character on the tokenizers this product meets.
	estimatorOutputTokensPer100Chars = 120
	// Photos and clips are observed in batches, so one call's overhead is shared by the
	// items in it.
	estimatorObserveBatch = 4
)

// recommended is the rung the comparison screen marks. It lives beside the grants it
// compares because which rung to recommend is a product decision, not a client's.
const recommended = Pro

// Offer is one rung as a comparison screen lists it.
type Offer struct {
	Plan           Plan
	MonthlyCredits int
	PriceUSDCents  int
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

// Pricer prices one call's tokens in micro-USD. The llm package's cost resolver satisfies
// it, which is how this stdlib-only package prices work without learning what a model is.
type Pricer func(promptTokens, completionTokens int64) (int64, bool)

// Rates are one combo's unit costs in MILLI-credits, the shape a comparison screen
// multiplies: a post costs
// `PerPostBase + photos×PerPhoto + videos×PerVideo + ceil(chars/1000)×Per1000Chars`.
//
// Milli rather than credits so a client stays in integers, and per unit rather than per post
// so a slider needs no round trip (QUOTA-40).
type Rates struct {
	PerPhoto     int
	PerVideo     int
	Per1000Chars int
	PerPostBase  int
}

// EstimatorRates derives one combo's unit rates from what its two models charge.
//
// Each rate carries the call overhead it is responsible for. The write call belongs to every
// post, so its ChargeBase and prompt sit in PerPostBase. An observation call is shared by the
// batch it carries, so a photo or a clip carries one batch-share of that call's base and
// prompt — amortized rather than counted with a ceiling, because the client is only allowed
// to multiply. A partial batch therefore reads up to three quarters of one ChargeBase cheaper
// than the gate will hold; the figure is labelled an estimate and the refusal stays
// authoritative (QUOTA-36).
//
// False means a model published no usable price, and a combo that cannot be priced is not
// published at all.
func EstimatorRates(observe, write Pricer) (Rates, bool) {
	observeShare, ok := observe(estimatorObservePromptTokens/estimatorObserveBatch, 0)
	if !ok {
		return Rates{}, false
	}
	photoCost, ok := observe(estimatorTokensPerPhoto, estimatorObserveOutputPerItem)
	if !ok {
		return Rates{}, false
	}
	videoCost, ok := observe(estimatorAssumedVideoSeconds*estimatorTokensPerVideoSec, estimatorObserveOutputPerItem)
	if !ok {
		return Rates{}, false
	}
	writePrompt, ok := write(estimatorWritePromptTokens, 0)
	if !ok {
		return Rates{}, false
	}
	charsCost, ok := write(0, 10*estimatorOutputTokensPer100Chars)
	if !ok {
		return Rates{}, false
	}

	itemShare := milliCredits(observeShare) + chargeBaseMilli/estimatorObserveBatch
	return Rates{
		PerPhoto:     milliCredits(photoCost) + itemShare,
		PerVideo:     milliCredits(videoCost) + itemShare,
		Per1000Chars: milliCredits(charsCost),
		PerPostBase:  milliCredits(writePrompt) + chargeBaseMilli,
	}, true
}

// chargeBaseMilli is ChargeBase expressed in the same milli-credits the rates use.
const chargeBaseMilli = ChargeBase * 1_000

// milliCredits converts a provider cost into thousandths of a credit, applying the same
// multiplier Charge does and truncating rather than rounding up — the per-call rounding is
// gone with the per-call accounting, and an estimate must not accumulate a ceiling per unit.
func milliCredits(costMicrousd int64) int {
	return int(costMicrousd * ChargeMultiplier * 1_000 / microusdPerCredit)
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

// AnchorWindow returns the grant window containing now. The anchor contributes only its
// Seoul day of month: each boundary lands on that day, or the last day when the month is
// shorter, and returns to the original day in the next long month.
func AnchorWindow(anchor, now time.Time) (start, end time.Time) {
	anchorDay := anchor.In(seoul).Day()
	localNow := now.In(seoul)
	start = anchorBoundary(localNow.Year(), localNow.Month(), anchorDay)
	if start.After(localNow) {
		previous := time.Date(localNow.Year(), localNow.Month()-1, 1, 0, 0, 0, 0, seoul)
		start = anchorBoundary(previous.Year(), previous.Month(), anchorDay)
	}
	next := time.Date(start.Year(), start.Month()+1, 1, 0, 0, 0, 0, seoul)
	return start, anchorBoundary(next.Year(), next.Month(), anchorDay)
}

func anchorBoundary(year int, month time.Month, anchorDay int) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, seoul).Day()
	return time.Date(year, month, min(anchorDay, lastDay), 0, 0, 0, 0, seoul)
}

// NextRenewal is the exclusive end of the anchor window containing now.
func NextRenewal(anchor, now time.Time) time.Time {
	_, end := AnchorWindow(anchor, now)
	return end
}
