package experiment

import (
	"context"
	"sync"
	"testing"
	"time"
)

type countingRetention struct {
	mu    sync.Mutex
	calls []time.Time
	swept chan struct{}
}

func (r *countingRetention) PurgeExpired(_ context.Context, before time.Time) (int64, error) {
	r.mu.Lock()
	r.calls = append(r.calls, before)
	r.mu.Unlock()
	r.swept <- struct{}{}
	return 0, nil
}
func (r *countingRetention) PurgePost(context.Context, string, string) error { return nil }

// MODEL-42: the retention sweep runs once at boot and then on every interval, so a deploy
// cadence faster than the interval never postpones the purge.
func TestSweeperRunsAtBootAndThenOnTheInterval(t *testing.T) {
	run := func(interval time.Duration) (*countingRetention, context.CancelFunc, chan struct{}) {
		retention := &countingRetention{swept: make(chan struct{}, 8)}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			NewSweeper(retention).Run(ctx, interval)
			close(done)
		}()
		return retention, cancel, done
	}
	wait := func(retention *countingRetention, what string) {
		t.Helper()
		select {
		case <-retention.swept:
		case <-time.After(5 * time.Second):
			t.Fatal("no sweep " + what)
		}
	}
	// An hour-long interval cannot tick inside the wait, so the sweep seen is the boot one.
	boot, cancel, done := run(time.Hour)
	wait(boot, "at boot")
	cancel()
	<-done
	ticking, cancel, done := run(10 * time.Millisecond)
	wait(ticking, "at boot")
	wait(ticking, "on the interval")
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the sweeper did not stop with its context")
	}
}
