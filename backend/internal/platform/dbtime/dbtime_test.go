package dbtime

import (
	"sort"
	"testing"
	"time"
)

func TestFormatIsFixedWidthUTC(t *testing.T) {
	zone := time.FixedZone("KST", 9*60*60)
	got := Format(time.Date(2026, 11, 16, 9, 0, 0, 0, zone))
	if want := "2026-11-16T00:00:00.000000000Z"; got != want {
		t.Fatalf("Format = %q, want %q", got, want)
	}
}

func TestFormatTextOrderEqualsTimeOrder(t *testing.T) {
	zone := time.FixedZone("KST", 9*60*60)
	ordered := []time.Time{
		time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 16, 0, 0, 0, 1, time.UTC),
		time.Date(2026, 11, 16, 0, 0, 0, 1000, time.UTC),
		// 09:00:00.5 KST is 00:00:00.5 UTC, later than the instants above.
		time.Date(2026, 11, 16, 9, 0, 0, 500_000_000, zone),
		time.Date(2026, 11, 16, 0, 0, 1, 0, time.UTC),
	}
	texts := make([]string, len(ordered))
	for i, v := range ordered {
		texts[i] = Format(v)
	}
	if !sort.StringsAreSorted(texts) {
		t.Fatalf("text order differs from time order: %v", texts)
	}
}

func TestParseRoundTripAndLegacyWidth(t *testing.T) {
	at := time.Date(2026, 11, 16, 1, 2, 3, 4, time.UTC)
	back, err := Parse(Format(at))
	if err != nil || !back.Equal(at) {
		t.Fatalf("round trip = %v, %v", back, err)
	}
	if _, err := Parse("2026-11-16T01:02:03Z"); err != nil {
		t.Fatalf("legacy width must still load: %v", err)
	}
	if _, err := Parse("nope"); err == nil {
		t.Fatal("want an error for garbage")
	}
}
