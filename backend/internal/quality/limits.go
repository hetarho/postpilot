package quality

import (
	"math"
	"time"
)

// The product's own measurement settings (QUAL-6, QUAL-11). They are code constants rather than
// env: they are product-owned, not per-account options, and changing a band rewrites what every
// badge means.
const (
	// MeasureVersion is bumped whenever MeasureSelf would give a different answer for the same
	// input; every stored self-measurement of another version is recomputed. The digest table in
	// measure_digest_test.go fails when this is forgotten.
	MeasureVersion = 2
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

// The daily phrase batch (QUAL-17, QUAL-38). PhraseRefreshInterval is the product default the
// QUALITY_PHRASE_REFRESH_INTERVAL override replaces. PhraseRefreshCheck is how often the loop
// looks for a due field: separate from the interval on purpose, because ticking once per
// interval against a next refresh stamped a moment after the tick finds nothing due on the next
// tick, and would refresh every second interval.
const (
	PhraseRefreshInterval = 24 * time.Hour
	// PhraseRefreshMinInterval is the floor an override may not go below (ARCH-42). One pass is 9
	// fields × 3 pages = 27 search calls; at one an hour that is 648 a day, and a failing field
	// retries within the same hourly cadence, so the batch cannot exceed it — about 2.6% of the
	// 25,000/day quota of the Naver application every stack using the keys shares (staging, prod,
	// dev). A one-minute interval would be 38,880 a day, over the quota. It equals
	// PhraseRetryDelay, so a failing field never retries faster than the floor either.
	PhraseRefreshMinInterval = time.Hour
	PhraseRefreshCheck       = 10 * time.Minute
	PhraseRetryDelay         = time.Hour
	// PhrasePageSize is one search page, which must equal the search client's display size;
	// PhrasePages of them make QUAL-17's 300 results.
	PhrasePageSize  = 100
	PhrasePages     = 3
	PhraseMinTokens = 2
	PhraseMaxTokens = 5
	PhraseListMax   = 50
)
