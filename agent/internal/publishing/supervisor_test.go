package publishing

import (
	"context"
	"errors"
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
