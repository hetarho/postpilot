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

// The estimator's rates are what a comparison screen multiplies, so they are pinned with
// their own arithmetic. The two models are priced differently on purpose: a photo must be
// priced by the OBSERVE model and a character by the WRITE model, and one shared price pair
// would hide a crossed wire.
func TestEstimatorRatesPriceEachUnitWithTheModelThatServesIt(t *testing.T) {
	// $0.30 / $2.50 per million observing, $1.00 / $10.00 writing. The pricer is the exact
	// arithmetic the llm cost resolver performs: micro-USD = tokens x USD-per-million.
	observe := func(prompt, completion int64) (int64, bool) {
		return prompt*30/100 + completion*250/100, true
	}
	write := func(prompt, completion int64) (int64, bool) {
		return prompt + completion*10, true
	}

	rates, ok := plan.EstimatorRates(observe, write)
	if !ok {
		t.Fatal("EstimatorRates refused two priced models")
	}
	for _, tc := range []struct {
		name string
		got  int
		want int
	}{
		// 320 photo tokens + 200 observation tokens = 596 micro-USD -> 178 milli, plus one
		// batch share of the observe call: 500 milli of ChargeBase + 45 milli of prompt.
		{"per photo", rates.PerPhoto, 723},
		// 15 s x 300 tokens + the same observation entry and batch share.
		{"per video", rates.PerVideo, 1100},
		// 1 000 characters at 120 output tokens per 100, priced by the write model.
		{"per 1000 chars", rates.Per1000Chars, 3600},
		// The write call: its ChargeBase in full plus its prompt.
		{"per post base", rates.PerPostBase, 3800},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d milli-credits, want %d", tc.name, tc.got, tc.want)
		}
	}

	// The shape a client multiplies: one 1 000-character post with five photos.
	post := rates.PerPostBase + 5*rates.PerPhoto + rates.Per1000Chars
	if posts := plan.MonthlyCredits(plan.Basic) * 1000 / post; posts != 19 {
		t.Errorf("basic covers %d posts of that shape, want 19", posts)
	}
}

// A model with no published price cannot be quoted, and a combo that cannot be priced is
// not published at all.
func TestEstimatorRatesRefusesAnUnpricedModel(t *testing.T) {
	priced := func(prompt, completion int64) (int64, bool) { return prompt + completion, true }
	unpriced := func(int64, int64) (int64, bool) { return 0, false }

	if _, ok := plan.EstimatorRates(unpriced, priced); ok {
		t.Error("an unpriced observe model was accepted")
	}
	if _, ok := plan.EstimatorRates(priced, unpriced); ok {
		t.Error("an unpriced write model was accepted")
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

func TestAnchorWindowClampsShortMonthsAndReturnsToTheAnchorDay(t *testing.T) {
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)
	anchor := time.Date(2025, 1, 31, 12, 0, 0, 0, seoul)

	for _, tc := range []struct {
		name      string
		now       time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "ordinary February",
			now:       time.Date(2026, 2, 15, 3, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, 1, 31, 0, 0, 0, 0, seoul),
			wantEnd:   time.Date(2026, 2, 28, 0, 0, 0, 0, seoul),
		},
		{
			name:      "ordinary March returns to 31",
			now:       time.Date(2026, 3, 10, 3, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, 2, 28, 0, 0, 0, 0, seoul),
			wantEnd:   time.Date(2026, 3, 31, 0, 0, 0, 0, seoul),
		},
		{
			name:      "leap February",
			now:       time.Date(2028, 2, 15, 3, 0, 0, 0, time.UTC),
			wantStart: time.Date(2028, 1, 31, 0, 0, 0, 0, seoul),
			wantEnd:   time.Date(2028, 2, 29, 0, 0, 0, 0, seoul),
		},
		{
			name:      "leap March returns to 31",
			now:       time.Date(2028, 3, 10, 3, 0, 0, 0, time.UTC),
			wantStart: time.Date(2028, 2, 29, 0, 0, 0, 0, seoul),
			wantEnd:   time.Date(2028, 3, 31, 0, 0, 0, 0, seoul),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, end := plan.AnchorWindow(anchor, tc.now)
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) {
				t.Errorf("window = [%s, %s), want [%s, %s)", start, end, tc.wantStart, tc.wantEnd)
			}
			if got := plan.NextRenewal(anchor, tc.now); !got.Equal(tc.wantEnd) {
				t.Errorf("NextRenewal = %s, want %s", got, tc.wantEnd)
			}
		})
	}
}

func TestAnchorWindowBoundariesUseTheSeoulCalendar(t *testing.T) {
	seoul := time.FixedZone("Asia/Seoul", 9*60*60)
	cases := []struct {
		name      string
		anchor    time.Time
		now       time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name:      "day one is a calendar month",
			anchor:    time.Date(2025, 1, 1, 0, 0, 0, 0, seoul),
			now:       time.Date(2026, 3, 31, 15, 30, 0, 0, time.UTC),
			wantStart: time.Date(2026, 4, 1, 0, 0, 0, 0, seoul),
			wantEnd:   time.Date(2026, 5, 1, 0, 0, 0, 0, seoul),
		},
		{
			name:      "before this months anchor uses previous month",
			anchor:    time.Date(2025, 1, 20, 0, 0, 0, 0, seoul),
			now:       time.Date(2026, 9, 10, 12, 0, 0, 0, seoul),
			wantStart: time.Date(2026, 8, 20, 0, 0, 0, 0, seoul),
			wantEnd:   time.Date(2026, 9, 20, 0, 0, 0, 0, seoul),
		},
		{
			name:      "exact anchor boundary opens the new window",
			anchor:    time.Date(2025, 1, 20, 0, 0, 0, 0, seoul),
			now:       time.Date(2026, 9, 19, 15, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, 9, 20, 0, 0, 0, 0, seoul),
			wantEnd:   time.Date(2026, 10, 20, 0, 0, 0, 0, seoul),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := plan.AnchorWindow(tc.anchor, tc.now)
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) {
				t.Errorf("window = [%s, %s), want [%s, %s)", start, end, tc.wantStart, tc.wantEnd)
			}
			if start.Location().String() != "Asia/Seoul" || end.Location().String() != "Asia/Seoul" {
				t.Errorf("locations = %q/%q, want fixed Seoul", start.Location(), end.Location())
			}
		})
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

func TestPaymentMethodBonusCredits(t *testing.T) {
	if plan.PaymentMethodBonusCredits != 100 {
		t.Fatalf("PaymentMethodBonusCredits = %d, want 100", plan.PaymentMethodBonusCredits)
	}
}
