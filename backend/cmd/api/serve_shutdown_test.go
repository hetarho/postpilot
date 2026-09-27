package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

// GEN-33: graceful shutdown stops the in-process worker before HTTP shutdown. The serving
// context outlives the signal until every worker has returned, however long its last
// terminal write takes.
func TestShutdownStopsTheWorkersBeforeHTTP(t *testing.T) {
	signal, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	stopped := 0
	serving, stop := startWorkers(signal, 3, func(ctx context.Context) {
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond) // the handler's bounded terminal write
		mu.Lock()
		stopped++
		mu.Unlock()
	})
	defer stop()
	cancel()
	select {
	case <-serving.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("serving never ended after the workers stopped")
	}
	mu.Lock()
	defer mu.Unlock()
	if stopped != 3 {
		t.Fatalf("HTTP shutdown began with %d of 3 workers still running", 3-stopped)
	}
}

// A listen failure returns from serve, and its deferred stop ends the workers and then the
// serving context without any signal.
func TestStoppingTheWorkersEndsServing(t *testing.T) {
	serving, stop := startWorkers(context.Background(), 1, func(ctx context.Context) { <-ctx.Done() })
	stop()
	select {
	case <-serving.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("serving never ended after stop")
	}
}
