package publishing

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
)

func TestSharedPermitSerializesConnections(t *testing.T) {
	permit := make(chan struct{}, 1)
	permit <- struct{}{}
	first := Supervisor{Permit: permit}
	second := Supervisor{Permit: permit}
	if !first.acquire(context.Background()) {
		t.Fatal("first connection did not acquire execution permit")
	}
	acquired := make(chan struct{})
	go func() {
		if second.acquire(context.Background()) {
			close(acquired)
		}
	}()
	select {
	case <-acquired:
		t.Fatal("second connection acquired while the first publication was running")
	case <-time.After(20 * time.Millisecond):
	}
	first.release()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second connection did not acquire after the first released")
	}
	second.release()
}

type delayedClaimer struct {
	calls atomic.Int32
	claim *postpilotv1.ClaimPublishJobResponse
}

func (d *delayedClaimer) Claim(context.Context) (*postpilotv1.ClaimPublishJobResponse, error) {
	if d.calls.Add(1) == 1 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("Mac was offline; no job seen yet"))
	}
	return d.claim, nil
}

type executorFunc func(context.Context, *postpilotv1.ClaimPublishJobResponse) error

func (f executorFunc) Execute(ctx context.Context, claim *postpilotv1.ClaimPublishJobResponse) error {
	return f(ctx, claim)
}

type claimerFunc func(context.Context) (*postpilotv1.ClaimPublishJobResponse, error)

func (function claimerFunc) Claim(ctx context.Context) (*postpilotv1.ClaimPublishJobResponse, error) {
	return function(ctx)
}

func TestSupervisorClaimsAQueuedJobAfterAnOfflinePollWithoutRestart(t *testing.T) {
	claim := &postpilotv1.ClaimPublishJobResponse{Job: &postpilotv1.PublishJob{Id: "queued-while-offline"}}
	claimer := &delayedClaimer{claim: claim}
	ctx, cancel := context.WithCancel(context.Background())
	permit := make(chan struct{}, 1)
	permit <- struct{}{}
	supervisor := Supervisor{
		Client: claimer, PollInterval: time.Millisecond, Permit: permit,
		Executor: executorFunc(func(_ context.Context, got *postpilotv1.ClaimPublishJobResponse) error {
			if got.GetJob().GetId() != claim.GetJob().GetId() {
				t.Fatalf("claim = %+v", got)
			}
			cancel()
			return nil
		}),
	}
	done := make(chan error, 1)
	go func() { done <- supervisor.Run(ctx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("supervisor error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("queued job was not claimed after the Mac resumed polling; calls=%d", claimer.calls.Load())
	}
	if claimer.calls.Load() < 2 {
		t.Fatalf("claim calls = %d", claimer.calls.Load())
	}
}

func TestSupervisorStopsOnlyTheRevokedConnection(t *testing.T) {
	permit := make(chan struct{}, 1)
	permit <- struct{}{}
	executed := false
	supervisor := Supervisor{
		Client: claimerFunc(func(context.Context) (*postpilotv1.ClaimPublishJobResponse, error) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("revoked"))
		}),
		Executor: executorFunc(func(context.Context, *postpilotv1.ClaimPublishJobResponse) error {
			executed = true
			return nil
		}),
		Permit: permit, PollInterval: time.Millisecond,
	}
	err := supervisor.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "token was revoked") || executed {
		t.Fatalf("error=%v executed=%t", err, executed)
	}
	select {
	case <-permit:
		permit <- struct{}{}
	default:
		t.Fatal("revoked connection did not release the cross-account permit")
	}
}

// PUBLISH-29 pins the polling cadence itself, so the rule is tested as a decision rather
// than by waiting on a clock: an empty queue is the ordinary answer and must not be
// treated as a fault, which is also what lets a Mac that slept resume at full cadence.
func TestBackoffGrowsOnlyOnTransientFaultsAndAnEmptyQueueResumesFullCadence(t *testing.T) {
	interval := 5 * time.Second
	transient := connect.NewError(connect.CodeUnavailable, errors.New("VPS unreachable"))
	emptyQueue := connect.NewError(connect.CodeNotFound, errors.New("no queued job"))
	for name, testCase := range map[string]struct {
		current time.Duration
		err     error
		want    time.Duration
	}{
		"empty queue holds the base interval":     {interval, emptyQueue, interval},
		"empty queue after a long outage resets":  {80 * time.Second, emptyQueue, interval},
		"first transient fault doubles":           {interval, transient, 10 * time.Second},
		"repeated transient faults keep doubling": {20 * time.Second, transient, 40 * time.Second},
		"doubling stops once past a minute":       {80 * time.Second, transient, 80 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			if got := backoffAfter(testCase.current, interval, testCase.err); got != testCase.want {
				t.Fatalf("backoffAfter(%s, %s) = %s, want %s", testCase.current, interval, got, testCase.want)
			}
		})
	}
}

// The Mac sleeping through an outage must not need a restart: the supervisor keeps
// polling, executes nothing while the VPS is unreachable, and runs the recovered job
// exactly once when it answers.
func TestAnOutageExecutesNothingAndTheRecoveredJobRunsOnceWithoutARestart(t *testing.T) {
	var attempts, executions atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	permit := make(chan struct{}, 1)
	permit <- struct{}{}
	supervisor := Supervisor{
		PollInterval: time.Millisecond, Permit: permit,
		Client: claimerFunc(func(context.Context) (*postpilotv1.ClaimPublishJobResponse, error) {
			switch attempt := attempts.Add(1); {
			case attempt <= 3:
				return nil, connect.NewError(connect.CodeUnavailable, errors.New("VPS unreachable while the Mac slept"))
			case attempt == 4:
				return &postpilotv1.ClaimPublishJobResponse{Job: &postpilotv1.PublishJob{Id: "queued-during-the-outage"}}, nil
			default:
				return nil, connect.NewError(connect.CodeNotFound, errors.New("no queued job"))
			}
		}),
		Executor: executorFunc(func(_ context.Context, claim *postpilotv1.ClaimPublishJobResponse) error {
			if claim.GetJob().GetId() != "queued-during-the-outage" {
				t.Errorf("claim = %+v", claim)
			}
			executions.Add(1)
			cancel()
			return nil
		}),
	}
	if err := supervisor.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("supervisor error = %v", err)
	}
	if executions.Load() != 1 {
		t.Fatalf("executions = %d, want the recovered job run exactly once", executions.Load())
	}
	if attempts.Load() < 4 {
		t.Fatalf("claim attempts = %d, want polling to have continued through the outage", attempts.Load())
	}
}
