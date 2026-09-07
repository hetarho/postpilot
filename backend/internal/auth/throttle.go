package auth

import (
	"sync"
	"time"
)

const (
	ThrottleLogin        = "login"
	ThrottleSignup       = "signup"
	ThrottleResend       = "resend"
	ThrottleResetRequest = "reset_request"
	ThrottleReset        = "reset"
	ThrottleGoogle       = "google"
)

type throttleRule struct {
	max    int
	window time.Duration
}

var throttleRules = map[string]throttleRule{
	ThrottleLogin:        {max: 10, window: 5 * time.Minute},
	ThrottleSignup:       {max: 5, window: time.Hour},
	ThrottleResend:       {max: 5, window: time.Hour},
	ThrottleResetRequest: {max: 5, window: time.Hour},
	ThrottleReset:        {max: 10, window: time.Hour},
	ThrottleGoogle:       {max: 10, window: 5 * time.Minute},
}

type throttleKey struct {
	class string
	ip    string
}

type throttleWindow struct {
	count int
	end   time.Time
}

// Throttle applies the fixed per-IP windows for public authentication writes.
// It is process-local by design: account lock state is durable, attack traffic is not
// allowed to turn into a SQLite write before credential work even begins.
type Throttle struct {
	mu      sync.Mutex
	windows map[throttleKey]throttleWindow
}

func NewThrottle() *Throttle {
	return &Throttle{windows: make(map[throttleKey]throttleWindow)}
}

// Allow consumes one attempt in class for ip. The boundary request is allowed; the
// next one is refused until retryAt. An unknown class is refused so a misspelled
// interceptor entry cannot silently ship without protection.
func (t *Throttle) Allow(class, ip string, now time.Time) (retryAt time.Time, ok bool) {
	rule, known := throttleRules[class]
	if !known {
		return time.Time{}, false
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.windows == nil {
		t.windows = make(map[throttleKey]throttleWindow)
	}

	key := throttleKey{class: class, ip: ip}
	window, found := t.windows[key]
	if !found || !now.Before(window.end) {
		window = throttleWindow{end: now.Add(rule.window)}
	}
	if window.count >= rule.max {
		t.windows[key] = window
		return window.end, false
	}
	window.count++
	t.windows[key] = window
	return window.end, true
}

// Sweep drops windows whose retry instant has arrived. Active keys are retained.
func (t *Throttle) Sweep(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, window := range t.windows {
		if !now.Before(window.end) {
			delete(t.windows, key)
		}
	}
}
