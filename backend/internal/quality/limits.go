package quality

import "math"

// The product's own measurement settings (QUAL-6, QUAL-11). They are code constants rather than
// env: they are product-owned, not per-account options, and changing a band rewrites what every
// badge means.
const (
	// MeasureVersion is bumped whenever MeasureSelf would give a different answer for the same
	// input; every stored self-measurement of another version is recomputed.
	MeasureVersion = 1
	// RunLength is how many consecutive 어절 a shared stretch needs to count (QUAL-8).
	RunLength = 8
	// TitleWindow is how many recent published titles M1 reads (QUAL-7).
	TitleWindow = 100
	// PostWindow is how many recent published posts M2, M3 and M4 read (QUAL-8).
	PostWindow = 20

	// The bands, as QUAL-11 words them: M1 warns above 30%, M2 above a 10% median, M3 above an
	// 8% repetition share or below 50% title relevance, M4 at a median of 2 or fewer types.
	TitleSaturationBand    = 0.30
	CrossPostPhrasesBand   = 0.10
	RepetitionShareBand    = 0.08
	TitleRelevanceFloor    = 0.50
	DistinctBlockTypesBand = 2
)

// Minimum is how many published posts a metric needs before its account value means anything
// (QUAL-7 to QUAL-10). A metric this build does not know is never met.
func Minimum(m Metric) int {
	switch m {
	case MetricTitleSaturation:
		return 10
	case MetricCrossPostPhrases:
		return 3
	case MetricInPostRepetition:
		return 1
	case MetricComposition:
		return 3
	default:
		return math.MaxInt
	}
}
