package worker_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/media"
	"github.com/postpilot/backend/internal/clip/worker"
)

type control struct {
	renew              func(context.Context) (time.Time, error)
	completed, claimed atomic.Int32
	completeError      error
	work               *clip.MediaWork
}

func (c *control) Claim(context.Context, clip.MediaWorkerProfile) (*clip.MediaWork, error) {
	if c.claimed.Add(1) == 1 {
		return c.work, nil
	}
	return nil, nil
}
func (c *control) Renew(ctx context.Context, _ clip.MediaLeaseCredentials, _ int) (time.Time, error) {
	return c.renew(ctx)
}
func (c *control) Complete(context.Context, clip.MediaLeaseCredentials, string) error {
	c.completed.Add(1)
	return c.completeError
}
func (c *control) Fail(context.Context, clip.MediaLeaseCredentials, clip.MediaFailure) error {
	return nil
}

type execute func(context.Context, clip.MediaWork) (string, error)

func (f execute) Execute(ctx context.Context, w clip.MediaWork) (string, error) { return f(ctx, w) }
func work() clip.MediaWork {
	return clip.MediaWork{LeaseRemaining: 400 * time.Millisecond, LeaseExpiresAt: time.Now().Add(400 * time.Millisecond), HeartbeatAfter: 40 * time.Millisecond, StageRemaining: 5 * time.Second}
}
func TestMissedRenewalKillsChildProcessBeforeExpiry(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "pid")
	// The group contains both a shell and a child; neither may outlive its lease.
	c := &control{renew: func(ctx context.Context) (time.Time, error) { <-ctx.Done(); return time.Time{}, ctx.Err() }}
	loop := worker.Loop{Control: c, Executor: execute(func(ctx context.Context, _ clip.MediaWork) (string, error) {
		r := media.ExecRunner{StdoutLimit: 1024, StderrLimit: 1024, WaitDelay: time.Second}
		_, err := r.Run(ctx, media.Command{Binary: "/bin/sh", Args: []string{"-c", `sleep 60 & echo $! > "$1"; wait`, "worker-test", pidfile}, Dir: dir})
		return "", err
	})}
	w := work()
	start := time.Now()
	err := loop.RunLease(t.Context(), w)
	if !errors.Is(err, clip.ErrMediaLeaseLost) || c.completed.Load() != 0 || time.Since(start) > time.Second {
		t.Fatalf("work survived expiry: %v", err)
	}
	raw, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("descendant survived cancellation", pid)
}
func TestRenewalContinuesAndStaleCompletionIsNotSuccess(t *testing.T) {
	var renewed atomic.Int32
	c := &control{renew: func(context.Context) (time.Time, error) {
		renewed.Add(1)
		return time.Now().Add(400 * time.Millisecond), nil
	}, completeError: clip.ErrMediaLeaseLost}
	l := worker.Loop{Control: c, Executor: execute(func(ctx context.Context, _ clip.MediaWork) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(650 * time.Millisecond):
			return "result", nil
		}
	})}
	if err := l.RunLease(t.Context(), work()); !errors.Is(err, clip.ErrMediaLeaseLost) {
		t.Fatal(err)
	}
	if renewed.Load() < 5 || c.completed.Load() != 1 {
		t.Fatal(renewed.Load(), c.completed.Load())
	}
}
func TestShutdownStopsClaimsAndBoundsCurrentDrain(t *testing.T) {
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	started := make(chan struct{})
	w := work()
	c := &control{work: &w, renew: func(context.Context) (time.Time, error) { return time.Now().Add(time.Second), nil }}
	l := worker.Loop{Control: c, Executor: execute(func(ctx context.Context, _ clip.MediaWork) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}), Profile: clip.MediaWorkerProfile{ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profile: clip.MediaCPUProfile}, DrainTimeout: 80 * time.Millisecond, PollDelay: func() time.Duration { return time.Millisecond }}
	done := make(chan error, 1)
	go func() { done <- l.Run(ctx) }()
	<-started
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("drain unbounded")
	}
	if c.claimed.Load() != 1 || c.completed.Load() != 0 {
		t.Fatal("claimed or completed during cancelled drain")
	}
}

func TestShutdownLetsCurrentAttemptFinishWithinDrain(t *testing.T) {
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	w := work()
	c := &control{work: &w, renew: func(context.Context) (time.Time, error) { return time.Now().Add(time.Second), nil }}
	l := worker.Loop{Control: c, Executor: execute(func(ctx context.Context, _ clip.MediaWork) (string, error) {
		stop()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return "receipt", nil
		}
	}), Profile: clip.MediaWorkerProfile{ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Profile: clip.MediaCPUProfile}, DrainTimeout: 200 * time.Millisecond, PollDelay: func() time.Duration { return time.Millisecond }}
	if err := l.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if c.claimed.Load() != 1 || c.completed.Load() != 1 {
		t.Fatal("drain dropped completed work", c.claimed.Load(), c.completed.Load())
	}
}
