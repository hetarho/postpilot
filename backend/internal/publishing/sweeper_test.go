package publishing

import (
	"testing"
	"time"
)

func TestSweeperRecoversLeasesMoreFrequentlyThanTheirTTL(t *testing.T) {
	sweeper := NewSweeper(nil, time.Hour, 45*time.Second)
	if sweeper.recoveryEvery != 22500*time.Millisecond {
		t.Fatalf("recovery interval = %s", sweeper.recoveryEvery)
	}
}

func TestSweeperKeepsTickerDurationPositive(t *testing.T) {
	sweeper := NewSweeper(nil, time.Hour, time.Nanosecond)
	if sweeper.recoveryEvery <= 0 {
		t.Fatalf("recovery interval = %s", sweeper.recoveryEvery)
	}
}
