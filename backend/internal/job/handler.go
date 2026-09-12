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
	admitter     Admitter
	pollInterval time.Duration
	wake         chan struct{}

	mu       sync.RWMutex
	handlers map[string]Handler
	terminal map[string]func(context.Context, Job, time.Time) error
	now      func() time.Time
	newID    func() string
}

func New(store Store, pollInterval time.Duration) *Queue {
	if pollInterval <= 0 {
		panic("job: poll interval must be positive")
	}
	return &Queue{
		store: store, pollInterval: pollInterval, wake: make(chan struct{}, 1),
		handlers: make(map[string]Handler), terminal: make(map[string]func(context.Context, Job, time.Time) error), now: time.Now, newID: newID,
	}
}

// OnTerminal releases resources owned by a job kind after its durable terminal write.
// Owners must make this idempotent and recover missed calls after interruption.
func (q *Queue) OnTerminal(kind string, fn func(context.Context, Job, time.Time) error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if fn == nil || q.terminal[kind] != nil {
		panic("job: invalid terminal observer")
	}
	q.terminal[kind] = fn
}
func (q *Queue) notifyTerminal(ctx context.Context, j Job, at time.Time) error {
	q.mu.RLock()
	fn := q.terminal[j.Kind]
	q.mu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx, j, at)
}

// Admit installs the plan gate. It is a setter rather than a New parameter because the
// gate is only meaningful for LLM-consuming kinds, and because the composition root
// builds the queue before the usage context it depends on. A queue with no admitter
// enqueues freely — which is what the queue's own tests want.
func (q *Queue) Admit(admitter Admitter) { q.admitter = admitter }

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
