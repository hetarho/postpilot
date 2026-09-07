package auth

import (
	"testing"
	"time"
)

func TestThrottleBoundaryRetryAndIsolation(t *testing.T) {
	now := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
	for _, tc := range []struct {
		class  string
		max    int
		window time.Duration
	}{
		{ThrottleLogin, 10, 5 * time.Minute},
		{ThrottleSignup, 5, time.Hour},
		{ThrottleResend, 5, time.Hour},
		{ThrottleResetRequest, 5, time.Hour},
		{ThrottleReset, 10, time.Hour},
		{ThrottleGoogle, 10, 5 * time.Minute},
	} {
		t.Run(tc.class, func(t *testing.T) {
			throttle := NewThrottle()
			wantRetry := now.Add(tc.window)
			for attempt := 1; attempt <= tc.max; attempt++ {
				retryAt, ok := throttle.Allow(tc.class, "192.0.2.1", now)
				if !ok || !retryAt.Equal(wantRetry) {
					t.Fatalf("attempt %d = (%v, %v), want (%v, true)", attempt, retryAt, ok, wantRetry)
				}
			}
			if retryAt, ok := throttle.Allow(tc.class, "192.0.2.1", now); ok || !retryAt.Equal(wantRetry) {
				t.Fatalf("refusal = (%v, %v), want (%v, false)", retryAt, ok, wantRetry)
			}
		})
	}

	throttle := NewThrottle()
	for range 10 {
		throttle.Allow(ThrottleLogin, "192.0.2.1", now)
	}
	if _, ok := throttle.Allow(ThrottleLogin, "192.0.2.2", now); !ok {
		t.Fatal("a second IP shared the first IP's window")
	}
	if _, ok := throttle.Allow(ThrottleReset, "192.0.2.1", now); !ok {
		t.Fatal("a second class shared the login window")
	}

	wantRetry := now.Add(5 * time.Minute)
	if retryAt, ok := throttle.Allow(ThrottleLogin, "192.0.2.1", wantRetry); !ok || !retryAt.Equal(wantRetry.Add(5*time.Minute)) {
		t.Fatalf("boundary reset = (%v, %v)", retryAt, ok)
	}
}

func TestThrottleSweepDropsOnlyExpiredWindows(t *testing.T) {
	now := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC)
	throttle := NewThrottle()
	throttle.Allow(ThrottleLogin, "192.0.2.1", now)
	throttle.Allow(ThrottleSignup, "192.0.2.2", now)

	throttle.Sweep(now.Add(5 * time.Minute))
	if len(throttle.windows) != 1 {
		t.Fatalf("windows after sweep = %d, want only the one-hour window", len(throttle.windows))
	}
	if _, found := throttle.windows[throttleKey{class: ThrottleSignup, ip: "192.0.2.2"}]; !found {
		t.Fatal("sweep dropped an active signup window")
	}

	throttle.Sweep(now.Add(time.Hour))
	if len(throttle.windows) != 0 {
		t.Fatalf("windows after full expiry = %d, want 0", len(throttle.windows))
	}
}
