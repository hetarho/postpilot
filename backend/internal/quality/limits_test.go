package quality

import (
	"testing"
	"time"
)

// The bands, minimums and windows are product settings rather than per-account options, and a
// changed band rewrites what every badge means, so a change here is a deliberate diff.
func TestBandsMinimumsAndWindowsArePinned(t *testing.T) {
	for name, pin := range map[string]struct{ got, want float64 }{
		"MeasureVersion":         {MeasureVersion, 1},
		"RunLength":              {RunLength, 8},
		"TitleWindow":            {TitleWindow, 100},
		"PostWindow":             {PostWindow, 20},
		"TitleSaturationBand":    {TitleSaturationBand, 0.30},
		"CrossPostPhrasesBand":   {CrossPostPhrasesBand, 0.10},
		"RepetitionShareBand":    {RepetitionShareBand, 0.08},
		"TitleRelevanceFloor":    {TitleRelevanceFloor, 0.50},
		"DistinctBlockTypesBand": {DistinctBlockTypesBand, 2},
	} {
		if pin.got != pin.want {
			t.Errorf("%s = %v, want %v", name, pin.got, pin.want)
		}
	}

	minimums := map[Metric]int{
		MetricTitleSaturation:  10,
		MetricCrossPostPhrases: 3,
		MetricInPostRepetition: 1,
		MetricComposition:      3,
	}
	for _, m := range Metrics() {
		if got := Minimum(m); got != minimums[m] {
			t.Errorf("Minimum(%s) = %d, want %d", m, got, minimums[m])
		}
		// Every minimum sits under both windows, so the published count after the TitleWindow cap
		// is exact whenever it is below one.
		if Minimum(m) > PostWindow || Minimum(m) > TitleWindow {
			t.Errorf("Minimum(%s) = %d exceeds a window", m, Minimum(m))
		}
	}
	if met(1<<30, "score") {
		t.Fatal("an unknown metric met a minimum")
	}
}

// The phrase batch's cadence and shape (QUAL-17, QUAL-38) are product settings too. A page is
// the search client's display size, so the two have to move together.
func TestPhraseBatchConstantsArePinned(t *testing.T) {
	for name, pin := range map[string]struct{ got, want time.Duration }{
		"PhraseRefreshInterval": {PhraseRefreshInterval, 24 * time.Hour},
		"PhraseRefreshCheck":    {PhraseRefreshCheck, 10 * time.Minute},
		"PhraseRetryDelay":      {PhraseRetryDelay, time.Hour},
	} {
		if pin.got != pin.want {
			t.Errorf("%s = %v, want %v", name, pin.got, pin.want)
		}
	}
	for name, pin := range map[string]struct{ got, want int }{
		"PhrasePageSize":  {PhrasePageSize, 100},
		"PhrasePages":     {PhrasePages, 3},
		"PhraseMinTokens": {PhraseMinTokens, 2},
		"PhraseMaxTokens": {PhraseMaxTokens, 5},
		"PhraseListMax":   {PhraseListMax, 50},
	} {
		if pin.got != pin.want {
			t.Errorf("%s = %d, want %d", name, pin.got, pin.want)
		}
	}
}
