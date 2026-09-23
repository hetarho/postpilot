package quality

import "testing"

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
