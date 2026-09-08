package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The renewal catch-up belongs to the worker, not to composition: it charges cards and sends
// mail per due account, so running it before the listener came up let a backlog hold /health
// shut until the health-gated rollout rolled the release back (review/diff-260908 F5).
func TestBillingWorkerCatchesUpBeforeAnyTickAndSurvivesAFailedPass(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	passes := make(chan time.Time, 4)
	// The first pass fails. A worker that treats that as fatal would never tick again.
	failFirst := errors.New("provider unavailable")
	done := make(chan struct{})
	go func() {
		defer close(done)
		// An interval far shorter than production's, so the tick under test is the next
		// thing that happens rather than something the test waits ten minutes for.
		runBillingPasses(ctx, time.Millisecond, func(now time.Time) error {
			passes <- now
			err := failFirst
			failFirst = nil
			return err
		})
	}()

	// No tick has to elapse for the catch-up: the first pass is the worker's first action.
	select {
	case <-passes:
	case <-time.After(2 * time.Second):
		t.Fatal("no catch-up pass before the first tick")
	}
	// And the failure did not stop the ticker.
	select {
	case <-passes:
	case <-time.After(2 * time.Second):
		t.Fatal("no tick after a failing first pass")
	}

	// The worker stops with its context.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker outlived its context")
	}
}
