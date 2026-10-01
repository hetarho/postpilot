package plan_test

import (
	"testing"

	"github.com/postpilot/backend/internal/plan"
)

func TestUpperMedianNeverUnderstatesAnEvenSample(t *testing.T) {
	for _, tc := range []struct {
		values []int
		want   int
	}{
		{nil, 0},
		{[]int{7}, 7},
		{[]int{9, 1, 5}, 5},
		{[]int{4, 1, 3, 2}, 3}, // (2+3)/2 = 2.5 rounds up
		{[]int{10, 10, 20, 20}, 15},
	} {
		if got := plan.UpperMedian(tc.values); got != tc.want {
			t.Errorf("UpperMedian(%v) = %d, want %d", tc.values, got, tc.want)
		}
	}
	values := []int{3, 1, 2}
	plan.UpperMedian(values)
	if values[0] != 3 || values[1] != 1 || values[2] != 2 {
		t.Fatalf("UpperMedian reordered its input: %v", values)
	}
}

func TestPostFigureSumIsAnEstimateWhenEitherPartIs(t *testing.T) {
	recent := plan.PostFigure{Credits: 12, Basis: plan.PostCreditsRecentUsage}
	estimate := plan.PostFigure{Credits: 30, Basis: plan.PostCreditsEstimate}
	if got := recent.Plus(recent); got != (plan.PostFigure{Credits: 24, Basis: plan.PostCreditsRecentUsage}) {
		t.Fatalf("recent+recent = %+v", got)
	}
	if got := recent.Plus(estimate); got != (plan.PostFigure{Credits: 42, Basis: plan.PostCreditsEstimate}) {
		t.Fatalf("recent+estimate = %+v", got)
	}
}

// The per-stage estimate shares the estimator cards' assumptions exactly: the observe share
// is photos × PerPhoto and the write share PerPostBase + chars × Per1000Chars, rounded up to
// whole credits at the end.
func TestStageEstimatesMatchTheEstimatorRates(t *testing.T) {
	rate := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29",
		ReferenceE4: 13_600_000, AppliedE4: 13_600_000}
	unit := func(int64, int64) (int64, bool) { return 1_000, true }
	rates, ok := plan.EstimatorRatesAt(unit, unit, rate)
	if !ok {
		t.Fatal("estimator rates")
	}
	observe, ok := plan.ObservePostCreditsAt(unit, rate, plan.EstimatorDefaultPhotos)
	if want := (plan.EstimatorDefaultPhotos*rates.PerPhoto + 999) / 1_000; !ok || observe != want {
		t.Fatalf("observe = %d ok=%v, want %d", observe, ok, want)
	}
	write, ok := plan.WritePostCreditsAt(unit, rate, plan.EstimatorDefaultChars)
	if want := (rates.PerPostBase + plan.EstimatorDefaultChars*rates.Per1000Chars/1_000 + 999) / 1_000; !ok || write != want {
		t.Fatalf("write = %d ok=%v, want %d", write, ok, want)
	}

	free := func(int64, int64) (int64, bool) { return 0, true }
	if got, ok := plan.WritePostCreditsAt(free, rate, plan.EstimatorDefaultChars); !ok || got != 0 {
		t.Fatalf("free write = %d ok=%v", got, ok)
	}
	unpriced := func(int64, int64) (int64, bool) { return 0, false }
	if _, ok := plan.ObservePostCreditsAt(unpriced, rate, 5); ok {
		t.Fatal("an unpriced model produced an observe estimate")
	}
	if _, ok := plan.WritePostCreditsAt(unit, plan.RateSnapshot{}, 1_000); ok {
		t.Fatal("an invalid rate produced a write estimate")
	}
}

// One assumed call is the estimator's own price for it, rounded up once (QUOTA-40, QUOTA-67).
func TestCallCreditsAtPricesOneCallAndRoundsUp(t *testing.T) {
	rate := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29",
		ReferenceE4: 13_600_000, AppliedE4: 13_600_000}
	var asked [2]int64
	pricer := func(in, out int64) (int64, bool) {
		asked = [2]int64{in, out}
		return 1_000, true
	}
	credits, ok := plan.CallCreditsAt(pricer, rate, 6_000, 3_000)
	if !ok || credits < 1 {
		t.Fatalf("credits = %d ok=%v", credits, ok)
	}
	// The edit allowance applies to both counts, as it does to every estimator call.
	if asked[0] <= 6_000 || asked[1] <= 3_000 {
		t.Fatalf("priced %v, want the allowance on both counts", asked)
	}
	free := func(int64, int64) (int64, bool) { return 0, true }
	if got, ok := plan.CallCreditsAt(free, rate, 6_000, 3_000); !ok || got != 0 {
		t.Fatalf("free call = %d ok=%v", got, ok)
	}
	unpriced := func(int64, int64) (int64, bool) { return 0, false }
	if _, ok := plan.CallCreditsAt(unpriced, rate, 6_000, 3_000); ok {
		t.Fatal("an unpriced model produced an estimate")
	}
	if _, ok := plan.CallCreditsAt(pricer, plan.RateSnapshot{}, 6_000, 3_000); ok {
		t.Fatal("an invalid rate produced an estimate")
	}
}
