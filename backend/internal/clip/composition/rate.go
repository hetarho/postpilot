package composition

import "math/bits"

// RateUnitPermille is 1x: a cut with no transform. The playback rate rides the
// language as integer permille so the edited output timeline is exact integer
// arithmetic everywhere, with no float rounding between preview and export.
const RateUnitPermille = 1000

// TransformedDurationMS is the ONE pre-transition output length of a source span
// played at a fixed rate (CDS-62), rounded to the nearest millisecond. Every
// caller — validation, correction, resolution, preview and rendering — computes
// a cut's length through it, so no two of them can disagree by a millisecond.
//
// The multiplication is checked rather than trusted: a span and a rate both
// arrive from stored or client data, and a silent overflow would turn a refused
// plan into a negative timeline.
func TransformedDurationMS(spanMS, ratePermille int) (int, bool) {
	if spanMS <= 0 || ratePermille <= 0 {
		return 0, false
	}
	scaled, ok := checkedMul(spanMS, RateUnitPermille)
	if !ok {
		return 0, false
	}
	// Nearest-millisecond rounding: half a rate-unit before the division.
	rounded, ok := checkedAdd(scaled, ratePermille/2)
	if !ok {
		return 0, false
	}
	out := rounded / ratePermille
	if out <= 0 {
		return 0, false
	}
	return out, true
}

func checkedMul(a, b int) (int, bool) {
	if a < 0 || b < 0 {
		return 0, false
	}
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	if hi != 0 || lo > uint64(maxInt) {
		return 0, false
	}
	return int(lo), true
}
func checkedAdd(a, b int) (int, bool) {
	if a < 0 || b < 0 || a > maxInt-b {
		return 0, false
	}
	return a + b, true
}

const maxInt = int(^uint(0) >> 1)
