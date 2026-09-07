package plan_test

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

func TestParseRefusesAnythingOffTheLadder(t *testing.T) {
	for _, value := range []string{"free", "basic", "pro", "max", "master", " max "} {
		if _, err := plan.Parse(value); err != nil {
			t.Errorf("Parse(%q) = %v, want a known rung", value, err)
		}
	}
	for _, value := range []string{"", "premium", "MAX", "owner"} {
		if got, err := plan.Parse(value); err == nil {
			t.Errorf("Parse(%q) = %q, want an error", value, got)
		}
	}
}

func TestMonthlyGrantsAreTheShippedLadder(t *testing.T) {
	for _, tc := range []struct {
		acting plan.Plan
		want   int
	}{
		{plan.Free, 50},
		{plan.Basic, 220},
		{plan.Pro, 575},
		{plan.Max, 1200},
		{plan.Master, 0},
	} {
		if got := plan.MonthlyCredits(tc.acting); got != tc.want {
			t.Errorf("MonthlyCredits(%s) = %d, want %d", tc.acting, got, tc.want)
		}
	}
}

// A paid rung must grant more than its price buys at the par purchase rate of one credit
// per US cent, or there is no reason to subscribe rather than top up.
func TestPaidRungsGrantABonusOverThePurchaseRate(t *testing.T) {
	for _, tc := range []struct {
		acting  plan.Plan
		bonusPc int
	}{
		{plan.Basic, 10},
		{plan.Pro, 15},
		{plan.Max, 20},
	} {
		var offer plan.Offer
		for _, candidate := range plan.Offers() {
			if candidate.Plan == tc.acting {
				offer = candidate
			}
		}
		if offer.PriceUSDCents == 0 {
			t.Fatalf("%s is not an offer with a price", tc.acting)
		}
		want := offer.PriceUSDCents + offer.PriceUSDCents*tc.bonusPc/100
		if offer.MonthlyCredits != want {
			t.Errorf("%s grants %d credits for %d cents, want %d (+%d%%)",
				tc.acting, offer.MonthlyCredits, offer.PriceUSDCents, want, tc.bonusPc)
		}
	}
}

// The reference case is what a comparison screen quotes, so it is pinned with its own
// arithmetic: three observe calls (10 photos in batches of 4) at 7 credits each plus one
// write call at 11. Each call is priced at the worst case the admission gate holds against.
func TestReferencePostCreditsPricesTheReferenceCase(t *testing.T) {
	const perObserve = 7 // 2 + ceil((9_000 + 5_120) * 3 / 10_000)
	const perWrite = 11  // 2 + ceil((9_000 + 20_480) * 3 / 10_000)
	if got, want := plan.ReferencePostCredits(), 3*perObserve+perWrite; got != want {
		t.Errorf("ReferencePostCredits() = %d, want %d", got, want)
	}
}

func TestEstimatedPostsFloorsTheGrantAgainstTheReferencePost(t *testing.T) {
	for _, tc := range []struct {
		acting plan.Plan
		want   int
	}{
		{plan.Free, 1},
		{plan.Basic, 6},
		{plan.Pro, 17},
		{plan.Max, 37},
	} {
		if got := plan.EstimatedPosts(tc.acting); got != tc.want {
			t.Errorf("EstimatedPosts(%s) = %d, want %d", tc.acting, got, tc.want)
		}
	}
}

// Exactly one rung is marked: two of them would make the mark meaningless, and none would
// leave the comparison screen with nothing to lead with.
func TestExactlyOneOfferIsRecommended(t *testing.T) {
	marked := make([]plan.Plan, 0, 1)
	for _, offer := range plan.Offers() {
		if offer.Recommended {
			marked = append(marked, offer.Plan)
		}
		if offer.EstimatedPosts != plan.EstimatedPosts(offer.Plan) {
			t.Errorf("%s offers %d posts, want %d",
				offer.Plan, offer.EstimatedPosts, plan.EstimatedPosts(offer.Plan))
		}
	}
	if len(marked) != 1 || marked[0] != plan.Pro {
		t.Errorf("recommended rungs = %v, want exactly [pro]", marked)
	}
	if plan.Recommended(plan.Master) {
		t.Error("master reported recommended, but it is not on offer")
	}
}

// A corrupted row must not read as the operator account: zero means unlimited, so an
// unknown plan has to fall to the strictest grant rather than to the zero value.
func TestUnknownPlanGetsTheStrictestGrantNotUnlimited(t *testing.T) {
	if got := plan.MonthlyCredits(plan.Plan("premium")); got != plan.MonthlyCredits(plan.Free) {
		t.Errorf("MonthlyCredits(unknown) = %d, want the free grant %d", got, plan.MonthlyCredits(plan.Free))
	}
	if plan.Unlimited(plan.Plan("premium")) {
		t.Error("an unknown plan reported unlimited")
	}
	if !plan.Unlimited(plan.Master) {
		t.Error("master did not report unlimited")
	}
}

func TestChargeIsBasePlusRoundedUpMultiple(t *testing.T) {
	for _, tc := range []struct {
		name         string
		costMicrousd int64
		want         int
	}{
		{"a free call still costs the per-request base", 0, 2},
		// Anything that cost anything at all consumes a whole credit on top of the base:
		// rounding down would make a cheap model effectively unmetered.
		{"one micro-USD rounds up to a whole credit", 1, 3},
		{"just under one credit of cost", 3_333, 3},
		{"exactly one credit of cost at 3x", 3_334, 4},
		{"the free stage pair", 2_300, 3},
		{"the basic stage pair", 25_600, 10},
		{"a sonnet pair", 69_000, 23},
		{"opus on both stages", 255_500, 79},
		// A negative can only come from a corrupted row; it must not credit the account.
		{"a negative cost is floored, not refunded", -5_000, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := plan.Charge(tc.costMicrousd); got != tc.want {
				t.Errorf("Charge(%d) = %d, want %d", tc.costMicrousd, got, tc.want)
			}
		})
	}
}

func TestChargeNeverShrinksAsCostGrows(t *testing.T) {
	previous := plan.Charge(0)
	for cost := int64(0); cost <= 500_000; cost += 997 {
		got := plan.Charge(cost)
		if got < previous {
			t.Fatalf("Charge(%d) = %d, below the previous %d", cost, got, previous)
		}
		previous = got
	}
}

func TestMonthWindowIsTheSeoulCalendarMonth(t *testing.T) {
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)

	// 15:30 UTC on the 31st is already the 1st of the next month in Seoul, which is the
	// case a UTC calendar would put in the wrong window.
	at := time.Date(2026, 3, 31, 15, 30, 0, 0, time.UTC)
	start, end := plan.MonthWindow(at)

	wantStart := time.Date(2026, 4, 1, 0, 0, 0, 0, seoul)
	if !start.Equal(wantStart) {
		t.Errorf("month start = %s, want %s", start, wantStart)
	}
	wantEnd := time.Date(2026, 5, 1, 0, 0, 0, 0, seoul)
	if !end.Equal(wantEnd) {
		t.Errorf("month end = %s, want %s", end, wantEnd)
	}
	if !plan.NextRenewal(at).Equal(end) {
		t.Errorf("NextRenewal = %s, want the month end %s", plan.NextRenewal(at), end)
	}
}

func TestInsufficientCreditsCarriesItsWholeExplanation(t *testing.T) {
	renews := time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	err := &plan.InsufficientCreditsError{Required: 79, Balance: 12, RenewsAt: renews}

	if err.Reason() != plan.ReasonInsufficientCredits {
		t.Errorf("Reason() = %q, want %q", err.Reason(), plan.ReasonInsufficientCredits)
	}
	params := err.Params()
	for key, want := range map[string]string{
		"required":  "79",
		"balance":   "12",
		"renews_at": "2026-09-30T15:00:00Z",
	} {
		if params[key] != want {
			t.Errorf("Params()[%q] = %q, want %q", key, params[key], want)
		}
	}
}
