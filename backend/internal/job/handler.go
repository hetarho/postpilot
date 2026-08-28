package job

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Progress writes one durable snapshot. Handlers call it between provider calls, never
// from a transaction that spans one.
type Progress func(stage string, done, total int)

// Handler performs one registered job kind.
type Handler func(ctx context.Context, found Job, progress Progress) error

// Queue owns enqueue/query behavior, the handler registry, and the worker wake signal.
type Queue struct {
	store        Store
	pollInterval time.Duration
	wake         chan struct{}

	mu       sync.RWMutex
	handlers map[string]Handler
	now      func() time.Time
	newID    func() string
}

func New(store Store, pollInterval time.Duration) *Queue {
	if pollInterval <= 0 {
		panic("job: poll interval must be positive")
	}
	return &Queue{
		store: store, pollInterval: pollInterval, wake: make(chan struct{}, 1),
		handlers: make(map[string]Handler), now: time.Now, newID: newID,
	}
}

// Register binds a kind to its owning context at the composition root.
func (q *Queue) Register(kind string, handler Handler) {
	if kind == "" || handler == nil {
		panic("job: kind and handler are required")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.handlers[kind]; exists {
		panic(fmt.Sprintf("job: handler already registered for %q", kind))
	}
	q.handlers[kind] = handler
}

func (q *Queue) handler(kind string) Handler {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.handlers[kind]
}
