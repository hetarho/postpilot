package usage

import (
	"math"
	"testing"
)

func TestCancelledClipChargeUsesTheUnusedReservationAndCannotOverflow(t *testing.T) {
	for _, tc := range []struct {
		reservation    int
		cost           int64
		confirmed, fee int
	}{
		{100, 60000, 20, 40}, {5, 0, 0, 3}, {5, 100, 3, 1},
		{0, 100, 0, 0}, {5, math.MaxInt64, 5, 0},
		{math.MaxInt, 0, 0, math.MaxInt/2 + 1},
	} {
		confirmed, fee := cancelledClipCharge(tc.cost, tc.reservation)
		if confirmed != tc.confirmed || fee != tc.fee || confirmed+fee > tc.reservation {
			t.Fatalf("reservation %d cost %d: confirmed=%d fee=%d", tc.reservation, tc.cost, confirmed, fee)
		}
	}
}
