package clip

import (
	"testing"
	"time"
)

func TestMediaStageLimits(t *testing.T) {
	defaults := DefaultMediaStageLimits(Environment{})
	if err := defaults.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, limits := range []MediaStageLimits{
		{0, time.Minute, time.Hour, 3},
		{time.Minute, time.Minute, time.Hour, 3},
		{time.Minute, 2 * time.Hour, time.Hour, 3},
		{time.Minute, time.Hour, 7 * time.Hour, 3},
		{time.Minute, time.Hour, 2 * time.Hour, 6},
		{time.Minute, time.Hour, 2 * time.Hour, 0},
	} {
		if limits.Validate() == nil {
			t.Fatalf("invalid limits: %+v", limits)
		}
	}
	override := DefaultMediaStageLimits(Environment{MediaLeaseTTL: 2 * time.Minute, MediaWaitTimeout: 3 * time.Minute, MediaStageTimeout: 4 * time.Minute, MediaMaxAttempts: 1})
	if override.LeaseTTL != 2*time.Minute || override.MaxAttempts != 1 || override.Validate() != nil {
		t.Fatalf("override: %+v", override)
	}
}
