// Package plan is the authorization ladder: the tier an account is on, what that tier
// may spend, and the calendar windows those limits are measured over.
//
// It is a leaf domain package — stdlib only — because every other context asks it the
// same question from a different layer: auth carries a Plan on the session, llm declares
// a floor per model, usage measures a window, and the rpc edges render its refusals.
package plan

import (
	"fmt"
	"strings"
	"time"
)

// Plan is one rung of the ladder. The string form is what the database column and
// providers.yaml store, so it is the canonical spelling in both directions.
type Plan string

const (
	Free   Plan = "free"
	Basic  Plan = "basic"
	Max    Plan = "max"
	Master Plan = "master"
)

// rank orders the ladder. It is unexported: callers compare through Allows, so the
// numbers can never leak into a stored value or a wire message.
var rank = map[Plan]int{Free: 0, Basic: 1, Max: 2, Master: 3}

// Parse converts a stored or configured value into a Plan. An unknown value is an error
// rather than a silent Free: a typo in providers.yaml must fail boot, not quietly lock
// a model down.
func Parse(value string) (Plan, error) {
	candidate := Plan(strings.TrimSpace(value))
	if _, ok := rank[candidate]; !ok {
		return "", fmt.Errorf("unknown plan %q (want free, basic, max, or master)", value)
	}
	return candidate, nil
}

// Valid reports whether p is a known rung.
func (p Plan) Valid() bool { _, ok := rank[p]; return ok }

func (p Plan) String() string { return string(p) }

// Allows reports whether p satisfies a floor. An unknown acting plan allows nothing and
// an unknown floor is satisfied by nothing — the ladder fails closed at both ends.
func (p Plan) Allows(floor Plan) bool {
	actual, ok := rank[p]
	if !ok {
		return false
	}
	required, ok := rank[floor]
	if !ok {
		return false
	}
	return actual >= required
}

// Limits is one tier's numeric authority. Zero means unlimited, not zero-allowance: only
// master carries zeros, and it is the tier that is never refused.
type Limits struct {
	DailyJobStarts        int
	DailyBudgetMicrousd   int64
	MonthlyBudgetMicrousd int64
}

// Unlimited reports whether this tier is exempt from every numeric axis.
func (l Limits) Unlimited() bool {
	return l.DailyJobStarts == 0 && l.DailyBudgetMicrousd == 0 && l.MonthlyBudgetMicrousd == 0
}

// usd converts whole and fractional dollars to the micro-USD the ledger stores, so the
// table below reads in the currency the product rule is written in.
const usd = 1_000_000

// limits is the product rule, owned by code rather than config: two deploys must never
// disagree about what a tier allows, and a limit is not an operator knob. GetMyPlan
// publishes these numbers so the frontend never hardcodes one.
var limits = map[Plan]Limits{
	Free:   {DailyJobStarts: 10, DailyBudgetMicrousd: usd / 10, MonthlyBudgetMicrousd: 2 * usd},
	Basic:  {DailyJobStarts: 30, DailyBudgetMicrousd: usd / 2, MonthlyBudgetMicrousd: 12 * usd},
	Max:    {DailyJobStarts: 100, DailyBudgetMicrousd: usd, MonthlyBudgetMicrousd: 25 * usd},
	Master: {},
}

// LimitsFor returns the tier's limits. An unknown plan gets the strictest known tier
// rather than an empty (= unlimited) struct, so a corrupted row cannot buy free spend.
func LimitsFor(p Plan) Limits {
	found, ok := limits[p]
	if !ok {
		return limits[Free]
	}
	return found
}

// seoul is the product's home timezone, fixed at UTC+9.
//
// It is a FixedZone rather than a tzdata lookup because the runtime image is
// distroless/static and ships no zoneinfo — and because KST has had no DST since 1988,
// so a fixed offset and the IANA zone agree on every instant this product will ever see.
// The zone is a product constant, not env config: two deploys must never disagree about
// when a day ends.
var seoul = time.FixedZone("Asia/Seoul", 9*60*60)

// DayWindow returns the calendar day containing t: its start (inclusive) and the instant
// it resets (exclusive). Callers compare with `>= start && < end`.
func DayWindow(t time.Time) (start, end time.Time) {
	local := t.In(seoul)
	start = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, seoul)
	return start, start.AddDate(0, 0, 1)
}

// MonthWindow returns the calendar month containing t, on the same convention.
func MonthWindow(t time.Time) (start, end time.Time) {
	local := t.In(seoul)
	start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, seoul)
	return start, start.AddDate(0, 1, 0)
}

// ModelFloor pairs a model ref with the plan it requires. It is the shape a gate needs:
// the ref for the refusal's copy, the floor for the comparison.
type ModelFloor struct {
	Ref     string
	MinPlan Plan
}

// EnsureAllowed refuses every ref the acting plan may not run, naming all of them in one
// error so a bulk operation reports its whole problem rather than one item at a time.
// Required is the highest floor among the offenders — the single upgrade that clears the
// request.
func EnsureAllowed(acting Plan, floors []ModelFloor) error {
	var locked []string
	required := Free
	for _, floor := range floors {
		if acting.Allows(floor.MinPlan) {
			continue
		}
		locked = append(locked, floor.Ref)
		// An undeclared floor cannot be satisfied by any tier (the registry refuses to boot
		// with one), so it is reported as requiring the top of the ladder rather than as
		// the zero-value bottom.
		declared, known := rank[floor.MinPlan]
		if !known {
			required = Master
			continue
		}
		if required != Master && declared > rank[required] {
			required = floor.MinPlan
		}
	}
	if len(locked) == 0 {
		return nil
	}
	return &ModelLockedError{Models: locked, Required: required}
}
