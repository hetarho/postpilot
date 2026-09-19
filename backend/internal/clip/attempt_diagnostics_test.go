package clip

import (
	"testing"
)

func TestObservationNumbersAreBoundedAndContentKeysAreExcluded(t *testing.T) {
	got := SafeAttemptValues(map[string]int{"focal_x_ppm": -100000, "raw_start_ms": -200, "raw_end_ms": 180000001, "after_ms": -1, "segment": 2, "private-canary": 42})
	if len(got) != 3 || got["focal_x_ppm"] != -100000 || got["raw_start_ms"] != -200 || got["segment"] != 2 {
		t.Fatal(got)
	}
}
