package plan_test

import (
	"math"
	"testing"

	"github.com/postpilot/backend/internal/plan"
)

func TestKrwChargeRoundsOnlyTheSummedJobOnce(t *testing.T) {
	applied, err := plan.AppliedRateE4(13_579_001)
	if err != nil || applied != 13_600_000 {
		t.Fatalf("applied=%d err=%v", applied, err)
	}
	rate := plan.RateSnapshot{Source: "eximbank", PublicationDate: "2026-09-29", ReferenceE4: 13_579_001, AppliedE4: applied}
	for _, tc := range []struct {
		cost int64
		want int
	}{{0, 0}, {1, 1}, {735, 1}, {736, 2}, {100_000, 136}} {
		got, err := plan.ChargeAt(tc.cost, rate)
		if err != nil || got != tc.want {
			t.Errorf("ChargeAt(%d)=%d err=%v, want %d", tc.cost, got, err, tc.want)
		}
	}
	if _, err := plan.ChargeAt(math.MaxInt64, rate); err == nil {
		t.Fatal("large charge should refuse instead of overflowing")
	}
}

func TestEstimatorUsesKrwMilliCreditsWithoutABaseCharge(t *testing.T) {
	rate := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29",
		ReferenceE4: 13_600_000, AppliedE4: 13_600_000}
	free := func(int64, int64) (int64, bool) { return 0, true }
	post, ok := plan.EstimatorRatesAt(free, free, rate)
	if !ok || post != (plan.Rates{}) {
		t.Fatalf("free post estimate=%+v ok=%v", post, ok)
	}
	clip, ok := plan.ClipEstimatorRatesAt(free, free, rate)
	if !ok || clip != (plan.ClipRates{}) {
		t.Fatalf("free clip estimate=%+v ok=%v", clip, ok)
	}
	unit := func(int64, int64) (int64, bool) { return 1_000, true }
	post, ok = plan.EstimatorRatesAt(unit, unit, rate)
	if !ok || post.PerPostBase != 1_360 || post.PerPhoto != 2_720 {
		t.Fatalf("paid KRW milli estimate=%+v ok=%v", post, ok)
	}
}
