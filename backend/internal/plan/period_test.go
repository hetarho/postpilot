package plan_test

import (
	"testing"
	"time"

	"github.com/postpilot/backend/internal/plan"
)

func TestOfferRules(t *testing.T) {
	want := []struct {
		plan                                   plan.Plan
		monthly, annual, daily, bonus, exports int
		grade                                  string
	}{
		{plan.Free, 0, 0, 0, 0, 0, "none"},
		{plan.Light, 1900, 19000, 15, 290, 2, "value"},
		{plan.Basic, 4900, 49000, 45, 510, 6, "balanced"},
		{plan.Pro, 9900, 99000, 85, 1070, 15, "premium"},
		{plan.Max, 29900, 299000, 235, 3170, 60, "top"},
	}
	offers := plan.Offers()
	if len(offers) != len(want) {
		t.Fatalf("offers = %d, want %d", len(offers), len(want))
	}
	for i, rule := range want {
		got := offers[i]
		if got.Plan != rule.plan || got.MonthlyKRW != rule.monthly || got.AnnualKRW != rule.annual || got.DailyCredits != rule.daily || got.MonthlyBonus != rule.bonus || got.ServerExports != rule.exports || got.ModelCeiling != rule.grade || got.Recommended != (rule.plan == plan.Pro) {
			t.Errorf("%s offer = %+v", rule.plan, got)
		}
	}
	if _, ok := plan.CommercialOffer(plan.Master); ok {
		t.Error("master must not be sold")
	}
	if got := plan.Packs(); len(got) != 3 || got[0].PriceKRW != 3000 || got[0].Credits != 1000 || got[1].PriceKRW != 9000 || got[1].Credits != 3000 || got[2].PriceKRW != 30000 || got[2].Credits != 10000 {
		t.Errorf("packs = %+v", got)
	}
}

func TestAnchoredPeriodsPreserveTimeAndOriginalDay(t *testing.T) {
	kst := time.FixedZone("Asia/Seoul", 9*3600)
	for _, year := range []int{2027, 2028} {
		anchor := time.Date(year, 1, 31, 11, 12, 13, 987654321, kst)
		febDay := 28
		if year == 2028 {
			febDay = 29
		}
		feb := time.Date(year, 2, febDay, 11, 12, 13, 987654321, kst)
		march := time.Date(year, 3, 31, 11, 12, 13, 987654321, kst)
		if got := plan.MonthBoundary(anchor, 1); !got.Equal(feb) {
			t.Errorf("February boundary = %s", got)
		}
		if got := plan.MonthBoundary(anchor, 2); !got.Equal(march) {
			t.Errorf("March boundary = %s", got)
		}
		start, end := plan.BenefitWindow(anchor, feb.Add(-time.Nanosecond))
		if !start.Equal(anchor) || !end.Equal(feb) {
			t.Errorf("before boundary = [%s,%s)", start, end)
		}
		start, end = plan.BenefitWindow(anchor, feb)
		if !start.Equal(feb) || !end.Equal(march) {
			t.Errorf("at boundary = [%s,%s)", start, end)
		}
		if got := plan.CoverageEnd(anchor, anchor, true); !got.Equal(time.Date(year+1, 1, 31, 11, 12, 13, 987654321, kst)) {
			t.Errorf("annual end = %s", got)
		}
		dailyStart, dailyEnd := plan.DailyWindow(anchor, anchor.Add(24*time.Hour-time.Nanosecond))
		if !dailyStart.Equal(anchor) || !dailyEnd.Equal(anchor.Add(24*time.Hour)) {
			t.Errorf("daily window = [%s,%s)", dailyStart, dailyEnd)
		}
	}
}

// BILL-4, QUOTA-37: a term ends one or twelve anchored months after the month it starts in,
// counted from the anchor, so a renewal from a clamped month returns to the original day.
func TestCoverageEndCountsFromTheMonthTheTermStarts(t *testing.T) {
	kst := time.FixedZone("Asia/Seoul", 9*3600)
	anchor := time.Date(2027, 1, 31, 10, 0, 0, 0, kst)
	feb := time.Date(2027, 2, 28, 10, 0, 0, 0, kst)
	for _, tc := range []struct {
		start  time.Time
		annual bool
		want   time.Time
	}{
		{anchor, false, feb},
		{anchor, true, time.Date(2028, 1, 31, 10, 0, 0, 0, kst)},
		{feb, false, time.Date(2027, 3, 31, 10, 0, 0, 0, kst)},
		{time.Date(2027, 3, 31, 10, 0, 0, 0, kst), false, time.Date(2027, 4, 30, 10, 0, 0, 0, kst)},
		{feb, true, time.Date(2028, 2, 29, 10, 0, 0, 0, kst)},
	} {
		if got := plan.CoverageEnd(anchor, tc.start, tc.annual); !got.Equal(tc.want) {
			t.Errorf("CoverageEnd(%s, annual=%v) = %s, want %s", tc.start, tc.annual, got, tc.want)
		}
	}
}

func TestProrationBoundariesAndRepeatedUpgrades(t *testing.T) {
	start := time.Date(2028, 2, 29, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Nanosecond)
	for _, tc := range []struct {
		at          time.Time
		floor, ceil int64
	}{
		{start, 3, 3}, {end.Add(-time.Nanosecond), 0, 1}, {end, 0, 0},
	} {
		floor, err := plan.ProrateFloor(3, start, end, tc.at)
		if err != nil || floor != tc.floor {
			t.Errorf("floor(%s) = %d, %v", tc.at, floor, err)
		}
		ceil, err := plan.ProrateCeil(3, start, end, tc.at)
		if err != nil || ceil != tc.ceil {
			t.Errorf("ceil(%s) = %d, %v", tc.at, ceil, err)
		}
	}
	if _, err := plan.ProrateCeil(3, start, end, start.Add(-time.Nanosecond)); err == nil {
		t.Error("time before term accepted")
	}
	if _, err := plan.ProrateFloor(-1, start, end, start); err == nil {
		t.Error("negative amount accepted")
	}
	if _, err := plan.ProrateCeil(1<<62, start, end, start.Add(time.Nanosecond)); err != nil {
		t.Errorf("checked multiplication rejected valid quotient: %v", err)
	}

	kst := time.FixedZone("Asia/Seoul", 9*3600)
	anchor := time.Date(2028, 1, 31, 12, 0, 0, 0, kst)
	paidEnd := plan.CoverageEnd(anchor, anchor, true)
	benefitEnd := plan.MonthBoundary(anchor, 1)
	first, err := plan.QuoteUpgrade(plan.Light, plan.Basic, true, anchor, paidEnd, anchor, benefitEnd, anchor)
	if err != nil || first.ChargeKRW != 30000 || first.BonusCredits != 220 || first.ServerExports != 4 {
		t.Errorf("light→basic = %+v, %v", first, err)
	}
	second, err := plan.QuoteUpgrade(plan.Basic, plan.Pro, true, anchor, paidEnd, anchor, benefitEnd, anchor)
	if err != nil || second.ChargeKRW != 50000 || second.BonusCredits != 560 || second.ServerExports != 9 {
		t.Errorf("basic→pro = %+v, %v", second, err)
	}
}
