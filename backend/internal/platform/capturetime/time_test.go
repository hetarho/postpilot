package capturetime

import (
	"testing"
	"time"
)

func TestCaptureTimeNormalizationAndAbsentValues(t *testing.T) {
	for _, raw := range []*string{nil, new("")} {
		if got, err := Parse(raw); got != nil || err != nil {
			t.Fatal(got, err)
		}
	}
	raw := "2026-10-15T12:34:56+09:00"
	got, err := Parse(&raw)
	if err != nil || got == nil || *Format(got) != "2026-10-15T03:34:56Z" {
		t.Fatal(got, err)
	}
	for _, raw := range []string{"2026-02-30T12:00:00Z", "2026:10:15 12:00:00", "2026-10-15T12:00:00", "9999-12-31T23:00:00-23:00"} {
		if _, err := Parse(&raw); err == nil {
			t.Fatal("invalid instant accepted", raw)
		}
	}
	zero := time.Time{}
	if got, err := Normalize(&zero); got != nil || err != nil {
		t.Fatal(got, err)
	}
	old := time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC)
	current := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if Video(nil) != nil || Video(&old) != nil || Video(&current) == nil {
		t.Fatal("invalid movie epoch admitted")
	}
}
