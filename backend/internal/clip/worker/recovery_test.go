package worker_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/media"
	"github.com/postpilot/backend/internal/clip/worker"
)

type stopReceipt struct {
	*control
	stopped      *atomic.Bool
	failure      clip.MediaFailure
	acknowledged bool
	err          error
}

func (c *stopReceipt) Fail(ctx context.Context, _ clip.MediaLeaseCredentials, f clip.MediaFailure) error {
	if !c.stopped.Load() {
		c.err = errors.New("acknowledged before executor stopped")
	}
	if ctx.Err() != nil {
		c.err = errors.New("stop receipt used cancelled execution context")
	}
	c.failure, c.acknowledged = f, true
	return nil
}
func TestCancellationAcknowledgementFollowsExecutorStop(t *testing.T) {
	var stopped atomic.Bool
	c := &stopReceipt{control: &control{renew: func(context.Context) (time.Time, error) { return time.Time{}, clip.ErrMediaCancelled }}, stopped: &stopped}
	loop := worker.Loop{Control: c, Executor: execute(func(ctx context.Context, _ clip.MediaWork) (string, error) {
		defer stopped.Store(true)
		<-ctx.Done()
		return "", ctx.Err()
	})}
	if err := loop.RunLease(t.Context(), work()); !errors.Is(err, clip.ErrMediaCancelled) {
		t.Fatal(err)
	}
	if c.err != nil || !c.acknowledged || c.failure != clip.MediaFailureCancelled {
		t.Fatal(c.failure, c.err)
	}
}
func TestRejectedOutputIsReportedWithoutAnotherEncode(t *testing.T) {
	var stopped atomic.Bool
	c := &stopReceipt{control: &control{completeError: clip.ErrInvalidMedia, renew: func(context.Context) (time.Time, error) { return time.Now().Add(time.Second), nil }}, stopped: &stopped}
	calls := 0
	loop := worker.Loop{Control: c, Executor: execute(func(context.Context, clip.MediaWork) (string, error) {
		calls++
		stopped.Store(true)
		return "result", nil
	})}
	if err := loop.RunLease(t.Context(), work()); !errors.Is(err, clip.ErrInvalidMedia) {
		t.Fatal(err)
	}
	if calls != 1 || c.err != nil || c.failure != clip.MediaFailureInvalidOutput {
		t.Fatal(calls, c.failure, c.err)
	}
}
func TestWorkerRestartCollectsOnlyItsAbandonedDirectory(t *testing.T) {
	base := t.TempDir()
	newAdapter := func(id string) *media.Adapter {
		t.Helper()
		cfg := clip.DefaultMediaConfig(clip.Environment{WorkRoot: worker.WorkRoot(base, id), FFmpegPath: "/bin/echo", FFprobePath: "/bin/echo", WorkStaleAge: time.Hour, MediaTimeout: time.Minute})
		a, err := media.New(cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	a, b := newAdapter("one"), newAdapter("two")
	root := worker.WorkRoot(base, "one")
	release, err := worker.LockWorkRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := worker.LockWorkRoot(root); err == nil {
		other()
		t.Fatal("two processes owned one worker root")
	}
	abandoned := filepath.Join(root, "postpilot-clip-00000000000000000000000000000000")
	if err = os.Mkdir(abandoned, 0700); err != nil {
		t.Fatal(err)
	}
	release()
	// A successor acquires the released kernel lock before boot collection.
	release, err = worker.LockWorkRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err = b.WithWorkspace(t.Context(), "another-worker", func(ws clip.MediaWorkspace) error {
		if err := a.CleanupAbandoned(t.Context()); err != nil {
			return err
		}
		if _, err := os.Stat(ws.Path); err != nil {
			return errors.New("another worker's live workspace disappeared")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(abandoned); !os.IsNotExist(err) {
		t.Fatal("fresh crash workspace was retained", err)
	}
	// A background pass through the API's root does not enter worker namespaces.
	if filepath.Dir(worker.WorkRoot(base, "../one")) != base || worker.WorkRoot(base, "one") == worker.WorkRoot(base, "two") {
		t.Fatal("unsafe worker namespace")
	}
}

func TestTransportAndStageDeadlinesHaveDifferentRecovery(t *testing.T) {
	for _, stageDeadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "transport", true: "stage"}[stageDeadline], func(t *testing.T) {
			var stopped atomic.Bool
			c := &stopReceipt{control: &control{renew: func(context.Context) (time.Time, error) { return time.Now().Add(time.Second), nil }}, stopped: &stopped}
			w := work()
			if stageDeadline {
				w.StageRemaining = 10 * time.Millisecond
			}
			loop := worker.Loop{Control: c, Executor: execute(func(ctx context.Context, _ clip.MediaWork) (string, error) {
				defer stopped.Store(true)
				if stageDeadline {
					<-ctx.Done()
				}
				return "", context.DeadlineExceeded
			})}
			if err := loop.RunLease(t.Context(), w); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			want := clip.MediaFailureWorkerLost
			if stageDeadline {
				want = clip.MediaFailureDeadlineExceeded
			}
			if c.err != nil || c.failure != want {
				t.Fatal(c.failure, want, c.err)
			}
		})
	}
}
