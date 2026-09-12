package clip

import (
	"math"
	"testing"
)

func TestQuoteCountCoversMinuteBoundariesAndGlobalLimit(t *testing.T) {
	cfg := MediaConfig{ChunkDurationMS: 60000, DurationToleranceMS: 1000, Sources: SourceConfig{MaxCount: 20, MaxDurationMS: 1800000}}
	for _, tc := range []struct {
		durations []int
		want      int
	}{
		{[]int{59000}, 1}, {[]int{60000}, 2}, {[]int{61000}, 2}, {[]int{1800000}, 30},
		{[]int{60000, 15000}, 3}, {[]int{90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000, 90000}, 40},
	} {
		b := SourceBatch{}
		for _, d := range tc.durations {
			b.Sources = append(b.Sources, SourceLease{SourceMetadata: SourceMetadata{DurationMS: d}})
		}
		got, err := ConservativeObservationCount(cfg, b)
		if err != nil || got != tc.want {
			t.Fatal(tc, got, err)
		}
	}
	for _, d := range []int{0, -1, 1800001, math.MaxInt} {
		if _, err := ConservativeObservationCount(cfg, SourceBatch{Sources: []SourceLease{{SourceMetadata: SourceMetadata{DurationMS: d}}}}); err == nil {
			t.Fatal(d)
		}
	}
	cfg.DurationToleranceMS = math.MaxInt
	if _, err := ConservativeObservationCount(cfg, SourceBatch{Sources: []SourceLease{{SourceMetadata: SourceMetadata{DurationMS: 1}}}}); err == nil {
		t.Fatal("overflow")
	}
}

func TestQuoteCountCanReachButNeverExceedsFortyNine(t *testing.T) {
	cfg := MediaConfig{ChunkDurationMS: 60000, DurationToleranceMS: 1000, Sources: SourceConfig{MaxCount: 20, MaxDurationMS: 1800000}}
	b := SourceBatch{Sources: make([]SourceLease, 20)}
	for i := range b.Sources {
		b.Sources[i].DurationMS = 1
	}
	b.Sources[19].DurationMS = 1800000 - 19
	if n, err := ConservativeObservationCount(cfg, b); err != nil || n != 49 {
		t.Fatal(n, err)
	}
}

func TestDisclosureVisibilityChangesQuoteBinding(t *testing.T) {
	p := Project{Disclosure: "sponsored"}
	shown := QuoteInputDigest(p, VideoTemplate{}, SourceBatch{}, GenerationPricing{})
	p.HideDisclosure = true
	if shown == QuoteInputDigest(p, VideoTemplate{}, SourceBatch{}, GenerationPricing{}) {
		t.Fatal("visibility did not invalidate quote")
	}
}
