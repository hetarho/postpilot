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
	Light  Plan = "light"
	Basic  Plan = "basic"
	Pro    Plan = "pro"
	Max    Plan = "max"
	Master Plan = "master"
)

// rank orders the ladder. It is unexported: nothing outside compares tiers any more —
// model access is decided by the balance, not by the rung — so the numbers exist only to
// keep Parse total and to give the admin surface a stable display order.
var rank = map[Plan]int{Free: 0, Light: 1, Basic: 2, Pro: 3, Max: 4, Master: 5}

// Parse converts a stored value into a Plan. An unknown value is an error rather than a
// silent Free: a corrupted row must fail loudly, not quietly change what an account may
// spend.
func Parse(value string) (Plan, error) {
	candidate := Plan(strings.TrimSpace(value))
	if _, ok := rank[candidate]; !ok {
		return "", fmt.Errorf("unknown plan %q (want free, light, basic, pro, max, or master)", value)
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
func Ladder() []Plan { return []Plan{Free, Light, Basic, Pro, Max, Master} }

// PaymentMethodBonusCredits is the one non-expiring grant earned by registering a
// payment method (BILL-10, QUOTA-9). The usage context persists it; billing decides
// when the account qualifies.
const PaymentMethodBonusCredits = 100

// monthlyCredits is the product rule for what a tier is granted each month. Zero means
// unlimited, not a zero allowance: only master carries it, and master is never refused.
var monthlyCredits = map[Plan]int{
	Free:   50,
	Light:  0,
	Basic:  330,
	Pro:    1150,
	Max:    2400,
	Master: 0,
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
	Plan          Plan
	MonthlyKRW    int
	AnnualKRW     int
	DailyCredits  int
	MonthlyBonus  int
	ModelCeiling  string
	ServerExports int
	// Recommended marks the one rung the screen highlights.
	Recommended bool
}

// Offers are the rungs on offer, in ladder order. Master is absent: it is the operator
// tier, not something anyone is offered.
func Offers() []Offer {
	rungs := []Plan{Free, Light, Basic, Pro, Max}
	offers := make([]Offer, 0, len(rungs))
	for _, rung := range rungs {
		offers = append(offers, Offer{
			Plan:          rung,
			MonthlyKRW:    offerRules[rung].monthlyKRW,
			AnnualKRW:     offerRules[rung].annualKRW,
			DailyCredits:  offerRules[rung].dailyCredits,
			MonthlyBonus:  offerRules[rung].monthlyBonus,
			ModelCeiling:  offerRules[rung].modelCeiling,
			ServerExports: offerRules[rung].serverExports,
			Recommended:   Recommended(rung),
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

// EstimatorClipSourceSeconds is the disclosed length of each original video in the
// plan comparison. The chosen duration is the finished clip, not its source footage.
const EstimatorClipSourceSeconds = 60

// ClipRates prices source observation, flow and narration. These are comparison
// assumptions; execution still quotes and settles its actual work independently.
type ClipRates struct {
	PerSource       int
	PerOutputSecond int
	PerClipBase     int
}

// estimatorTokenAllowance budgets 50% more input/output tokens for AI revisions after
// generation. It changes comparison estimates only, never reservations or ledger charges.
func estimatorTokenAllowance(tokens int64) int64 { return (tokens*3 + 1) / 2 }

// Recommended reports whether this rung is the one a comparison screen marks.
func Recommended(p Plan) bool { return p == recommended }

// Unlimited reports whether this tier is exempt from the balance check. Its work is still
// held, recorded and settled — unlimited spend is exactly the account whose spend the
// operator most wants to be able to read.
func Unlimited(p Plan) bool { return p == Master }

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
